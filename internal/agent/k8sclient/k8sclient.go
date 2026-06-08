// Package k8sclient is a minimal in-cluster Kubernetes API client. It is
// scoped to the read paths the cicd-sensor Agent needs for ARC scale-set
// resolution and intentionally does not depend on k8s.io/client-go.
//
// The client reads its credentials from the standard service account mount
// (/var/run/secrets/kubernetes.io/serviceaccount/), authenticates to the
// kube-apiserver Service exposed at KUBERNETES_SERVICE_HOST /
// KUBERNETES_SERVICE_PORT, and performs JSON GETs against the core API.
package k8sclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// defaultServiceAccountMount is where the projected service account
	// volume is exposed inside every pod the kubelet schedules.
	defaultServiceAccountMount = "/var/run/secrets/kubernetes.io/serviceaccount"

	tokenFilename  = "token"
	caCertFilename = "ca.crt"

	defaultRequestTimeout = 10 * time.Second
)

// Config describes how to reach the kube-apiserver. Use InClusterConfig to
// populate it from the standard service account mount.
type Config struct {
	// APIServer is the base URL of the kube-apiserver, e.g.
	// https://kubernetes.default.svc.
	APIServer string
	// Token is the bearer token forwarded on every request.
	Token string
	// CACertPEM is the PEM-encoded CA bundle the client uses to verify the
	// kube-apiserver. Empty means use the system roots, which is rarely
	// correct in-cluster.
	CACertPEM []byte
	// RequestTimeout bounds the per-request roundtrip. Defaults to 10s.
	RequestTimeout time.Duration
}

// InClusterConfig loads a Config from the standard projected service account
// volume and the KUBERNETES_SERVICE_HOST / KUBERNETES_SERVICE_PORT env vars
// that the kubelet injects into every pod.
func InClusterConfig() (Config, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return Config{}, errors.New("in-cluster config requires KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT")
	}
	token, err := os.ReadFile(filepath.Join(defaultServiceAccountMount, tokenFilename))
	if err != nil {
		return Config{}, fmt.Errorf("read service account token: %w", err)
	}
	caPEM, err := os.ReadFile(filepath.Join(defaultServiceAccountMount, caCertFilename))
	if err != nil {
		return Config{}, fmt.Errorf("read service account CA bundle: %w", err)
	}
	return Config{
		APIServer: "https://" + net.JoinHostPort(host, port),
		Token:     strings.TrimSpace(string(token)),
		CACertPEM: caPEM,
	}, nil
}

// Client is a minimal kube-apiserver client.
type Client struct {
	apiServer  *url.URL
	token      string
	httpClient *http.Client
}

// New constructs a Client from cfg.
func New(cfg Config) (*Client, error) {
	if cfg.APIServer == "" {
		return nil, errors.New("APIServer is required")
	}
	if cfg.Token == "" {
		return nil, errors.New("Token is required")
	}
	apiServer, err := url.Parse(cfg.APIServer)
	if err != nil {
		return nil, fmt.Errorf("parse APIServer: %w", err)
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(cfg.CACertPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cfg.CACertPEM) {
			return nil, errors.New("CACertPEM does not contain any PEM-encoded certificates")
		}
		tlsConfig.RootCAs = pool
	}

	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
	return &Client{
		apiServer:  apiServer,
		token:      cfg.Token,
		httpClient: httpClient,
	}, nil
}

// Pod is the subset of the v1.Pod object the Agent reads. Only the metadata
// fields the ARC scale-set resolver needs are decoded.
type Pod struct {
	Namespace string
	Name      string
	UID       string
	Labels    map[string]string
}

// ListPodsInNamespace returns the pods in the given namespace. The Agent
// keeps this list cached and refreshes it periodically; on-demand lookups
// are not made from the request path.
func (c *Client) ListPodsInNamespace(ctx context.Context, namespace string) ([]Pod, error) {
	if c == nil {
		return nil, errors.New("nil client")
	}
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}

	endpoint := *c.apiServer
	endpoint.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods", url.PathEscape(namespace))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list pods in %s: %w", namespace, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("list pods in %s: status %d: %s", namespace, resp.StatusCode, string(body))
	}

	var wire struct {
		Items []struct {
			Metadata struct {
				Namespace string            `json:"namespace"`
				Name      string            `json:"name"`
				UID       string            `json:"uid"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("decode pods response: %w", err)
	}

	pods := make([]Pod, 0, len(wire.Items))
	for _, item := range wire.Items {
		pods = append(pods, Pod{
			Namespace: item.Metadata.Namespace,
			Name:      item.Metadata.Name,
			UID:       item.Metadata.UID,
			Labels:    item.Metadata.Labels,
		})
	}
	return pods, nil
}
