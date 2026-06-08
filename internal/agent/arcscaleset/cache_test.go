package arcscaleset

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/agent/k8sclient"
	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

type fakeLister struct {
	pods map[string][]k8sclient.Pod
	// failures lets a single namespace fail in a specific call so the test
	// can confirm Refresh keeps the rest of the cache intact.
	failures map[string]error
	calls    atomic.Int64
}

func (f *fakeLister) ListPodsInNamespace(_ context.Context, namespace string) ([]k8sclient.Pod, error) {
	f.calls.Add(1)
	if err, ok := f.failures[namespace]; ok {
		return nil, err
	}
	return f.pods[namespace], nil
}

func TestCacheRefresh_IndexesPodUIDToScaleSet(t *testing.T) {
	lister := &fakeLister{
		pods: map[string][]k8sclient.Pod{
			"arc-prod": {
				{
					Namespace: "arc-prod",
					Name:      "prod-runner-abc",
					UID:       "uid-prod-1",
					Labels: map[string]string{
						scaleSetNamespaceLabel: "arc-prod",
						scaleSetNameLabel:      "prod-deploy",
						"other":                "label",
					},
				},
			},
			"arc-ci": {
				{
					Namespace: "arc-ci",
					Name:      "ci-runner-xyz",
					UID:       "uid-ci-1",
					Labels: map[string]string{
						scaleSetNamespaceLabel: "arc-ci",
						scaleSetNameLabel:      "ci-tests",
					},
				},
				{
					Namespace: "arc-ci",
					Name:      "unlabeled-pod",
					UID:       "uid-ci-2",
					Labels:    map[string]string{"unrelated": "true"},
				},
			},
		},
	}
	cache, err := NewCache(slog.Default(), lister, []string{"arc-prod", "arc-ci"})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got, want := cache.Revision(), uint64(1); got != want {
		t.Fatalf("Revision: got %d, want %d", got, want)
	}

	if s, ok := cache.Lookup("uid-prod-1"); !ok || s != (jobcontext.ARCScaleSet{Namespace: "arc-prod", Name: "prod-deploy"}) {
		t.Fatalf("prod lookup: ok=%v scale-set=%+v", ok, s)
	}
	if s, ok := cache.Lookup("uid-ci-1"); !ok || s != (jobcontext.ARCScaleSet{Namespace: "arc-ci", Name: "ci-tests"}) {
		t.Fatalf("ci lookup: ok=%v scale-set=%+v", ok, s)
	}
	if _, ok := cache.Lookup("uid-ci-2"); ok {
		t.Fatal("unlabeled pod should not be indexed")
	}
}

func TestCacheRefresh_TolerantToNamespaceFailure(t *testing.T) {
	lister := &fakeLister{
		pods: map[string][]k8sclient.Pod{
			"ok": {{UID: "uid-1", Labels: map[string]string{scaleSetNamespaceLabel: "ok", scaleSetNameLabel: "a"}}},
		},
		failures: map[string]error{
			"broken": errors.New("apiserver flake"),
		},
	}
	cache, err := NewCache(slog.Default(), lister, []string{"ok", "broken"})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, ok := cache.Lookup("uid-1"); !ok {
		t.Fatal("healthy namespace should still index its pods")
	}
}

func TestNewCacheRejectsEmptyNamespaces(t *testing.T) {
	if _, err := NewCache(slog.Default(), &fakeLister{}, nil); err == nil {
		t.Fatal("NewCache with no namespaces should return an error")
	}
}
