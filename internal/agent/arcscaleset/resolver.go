package arcscaleset

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

// CacheLookup is the subset of *Cache the resolver consumes. Tests
// substitute a fake.
type CacheLookup interface {
	Lookup(podUID string) (jobcontext.ARCScaleSet, bool)
}

// Resolver implements listener.ARCScaleSetResolver. It reads the peer's
// /proc/<pid>/cgroup line to find the kubelet pod-cgroup segment, extracts
// the pod UID, and looks it up in the Cache.
type Resolver struct {
	cache  CacheLookup
	logger *slog.Logger
}

// NewResolver constructs a Resolver backed by the given Cache.
func NewResolver(logger *slog.Logger, cache CacheLookup) (*Resolver, error) {
	if cache == nil {
		return nil, errors.New("cache is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Resolver{
		cache:  cache,
		logger: logger.With("component", "arc_scale_set_resolver"),
	}, nil
}

// Resolve implements listener.ARCScaleSetResolver.
//
// The error contract is intentionally narrow:
//
//   - A peer outside a Kubernetes pod returns the zero ARCScaleSet and nil
//     error, signalling the single-scale-set fallback. This is the right
//     behaviour for hooks that legitimately bypass ARC.
//   - A cache miss for a Kubernetes pod returns the zero ARCScaleSet and
//     nil error too, with a log line at info. The cache may simply not have
//     refreshed since the pod was scheduled; treating this as a hard error
//     would fail jobs during normal cluster churn.
//   - Filesystem failures reading /proc are returned as errors so the
//     listener can surface them as 5xx.
func (r *Resolver) Resolve(ctx context.Context, peerPID int32) (jobcontext.ARCScaleSet, error) {
	cgroupFile, err := podCgroupReader(peerPID)
	if err != nil {
		return jobcontext.ARCScaleSet{}, err
	}
	uid, err := extractPodUIDFromCgroup(cgroupFile)
	if err != nil {
		if errors.Is(err, ErrNotInKubernetesPod) {
			r.logger.InfoContext(ctx, "arc_scale_set_peer_outside_pod",
				"peer_pid", peerPID,
			)
			return jobcontext.ARCScaleSet{}, nil
		}
		return jobcontext.ARCScaleSet{}, err
	}
	s, ok := r.cache.Lookup(uid)
	if !ok {
		r.logger.InfoContext(ctx, "arc_scale_set_cache_miss",
			"peer_pid", peerPID,
			"pod_uid", uid,
		)
		return jobcontext.ARCScaleSet{}, nil
	}
	r.logger.InfoContext(ctx, "arc_scale_set_resolved",
		"peer_pid", peerPID,
		"pod_uid", uid,
		"scale_set_namespace", s.Namespace,
		"scale_set_name", s.Name,
	)
	return s, nil
}
