package arcscaleset

import (
	"context"
	"log/slog"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

type fakeCache struct {
	entries map[string]jobcontext.ARCScaleSet
}

func (f *fakeCache) Lookup(uid string) (jobcontext.ARCScaleSet, bool) {
	s, ok := f.entries[uid]
	return s, ok
}

func withCgroupReader(t *testing.T, reader func(int32) (string, error)) {
	t.Helper()
	prev := podCgroupReader
	podCgroupReader = reader
	t.Cleanup(func() { podCgroupReader = prev })
}

func TestResolverResolvesPodInCache(t *testing.T) {
	withCgroupReader(t, func(pid int32) (string, error) {
		return "0::/kubepods.slice/kubepods-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/cri-containerd-abc.scope\n", nil
	})
	cache := &fakeCache{
		entries: map[string]jobcontext.ARCScaleSet{
			"6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3": {Namespace: "arc-prod", Name: "deploy"},
		},
	}
	r, err := NewResolver(slog.Default(), cache)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	s, err := r.Resolve(context.Background(), 42)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if s != (jobcontext.ARCScaleSet{Namespace: "arc-prod", Name: "deploy"}) {
		t.Fatalf("scale-set: got %+v", s)
	}
}

func TestResolverReturnsZeroForPeerOutsidePod(t *testing.T) {
	withCgroupReader(t, func(pid int32) (string, error) {
		return "0::/system.slice/sshd.service\n", nil
	})
	r, _ := NewResolver(slog.Default(), &fakeCache{})
	s, err := r.Resolve(context.Background(), 1)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !s.IsZero() {
		t.Fatalf("expected zero ARCScaleSet, got %+v", s)
	}
}

func TestResolverReturnsZeroOnCacheMiss(t *testing.T) {
	withCgroupReader(t, func(pid int32) (string, error) {
		return "0::/kubepods.slice/kubepods-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/cri.scope\n", nil
	})
	r, _ := NewResolver(slog.Default(), &fakeCache{entries: map[string]jobcontext.ARCScaleSet{}})
	s, err := r.Resolve(context.Background(), 99)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !s.IsZero() {
		t.Fatalf("expected zero ARCScaleSet on cache miss, got %+v", s)
	}
}
