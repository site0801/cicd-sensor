package k8sclient

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListPodsInNamespace_DecodesLabels(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/namespaces/arc-prod/pods"; got != want {
			t.Errorf("path: got %q, want %q", got, want)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header: got %q, want %q", got, "Bearer test-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "items": [
		    {"metadata":{"namespace":"arc-prod","name":"runner-1","uid":"uid-1","labels":{"actions.github.com/scale-set-name":"prod"}}},
		    {"metadata":{"namespace":"arc-prod","name":"runner-2","uid":"uid-2","labels":{}}}
		  ]
		}`))
	}))
	defer server.Close()

	caPEM := certPEMFromTestServer(t, server)
	cfg := Config{
		APIServer: server.URL,
		Token:     "test-token",
		CACertPEM: caPEM,
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pods, err := client.ListPodsInNamespace(context.Background(), "arc-prod")
	if err != nil {
		t.Fatalf("ListPodsInNamespace: %v", err)
	}
	if got, want := len(pods), 2; got != want {
		t.Fatalf("pods: got %d, want %d", got, want)
	}
	if pods[0].UID != "uid-1" || pods[0].Labels["actions.github.com/scale-set-name"] != "prod" {
		t.Fatalf("pod 0 mismatch: %+v", pods[0])
	}
}

func TestListPodsInNamespace_HTTPErrorSurfaced(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	caPEM := certPEMFromTestServer(t, server)
	client, err := New(Config{APIServer: server.URL, Token: "t", CACertPEM: caPEM})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := client.ListPodsInNamespace(context.Background(), "arc-prod"); err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestNewRejectsMissingAPIServer(t *testing.T) {
	if _, err := New(Config{Token: "t"}); err == nil {
		t.Fatal("expected error when APIServer is empty")
	}
}

func TestNewRejectsBadCAPEM(t *testing.T) {
	if _, err := New(Config{APIServer: "https://example", Token: "t", CACertPEM: []byte("not pem")}); err == nil {
		t.Fatal("expected error for invalid CA PEM")
	}
}

// certPEMFromTestServer extracts the test server's self-signed cert as PEM
// so the production code path that parses CACertPEM is exercised.
func certPEMFromTestServer(t *testing.T, server *httptest.Server) []byte {
	t.Helper()
	cert := server.Certificate()
	if cert == nil || len(cert.Raw) == 0 {
		t.Fatal("test server has no certificate")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}
