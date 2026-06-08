package arcscaleset

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/cicd-sensor/cicd-sensor/internal/agent/k8sclient"
	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

const (
	// scaleSetNamespaceLabel is the label the ARC controller writes onto
	// every runner pod identifying the scale-set namespace. cicd-sensor
	// reads this label as the first half of the scale-set identity.
	scaleSetNamespaceLabel = "actions.github.com/scale-set-namespace"
	// scaleSetNameLabel is the second half of the scale-set identity.
	scaleSetNameLabel = "actions.github.com/scale-set-name"

	// defaultRefreshInterval is the polling cadence used when the caller
	// does not override it. The cache uses GET-and-replace; the load on
	// the kube-apiserver is one request per watched namespace per tick.
	defaultRefreshInterval = 30 * time.Second
)

// PodLister is the subset of k8sclient.Client the cache consumes. Tests
// substitute a fake.
type PodLister interface {
	ListPodsInNamespace(ctx context.Context, namespace string) ([]k8sclient.Pod, error)
}

// Cache holds the pod-UID → ARCScaleSet mapping that backs the resolver.
// It is refreshed in the background on a fixed cadence.
type Cache struct {
	lister          PodLister
	namespaces      []string
	logger          *slog.Logger
	refreshInterval time.Duration

	mu       sync.RWMutex
	byUID    map[string]jobcontext.ARCScaleSet
	revision uint64
}

// NewCache constructs a Cache that watches the given ARC namespaces. The
// returned cache is empty until Refresh or Run populates it.
func NewCache(logger *slog.Logger, lister PodLister, namespaces []string) (*Cache, error) {
	if lister == nil {
		return nil, errors.New("PodLister is required")
	}
	if len(namespaces) == 0 {
		return nil, errors.New("at least one ARC namespace is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{
		lister:          lister,
		namespaces:      append([]string(nil), namespaces...),
		logger:          logger.With("component", "arc_scale_set_cache"),
		refreshInterval: defaultRefreshInterval,
		byUID:           make(map[string]jobcontext.ARCScaleSet),
	}, nil
}

// SetRefreshInterval overrides the polling cadence used by Run. Must be
// called before Run.
func (c *Cache) SetRefreshInterval(d time.Duration) {
	if d > 0 {
		c.refreshInterval = d
	}
}

// Lookup returns the scale-set identity for a pod UID and reports whether
// the cache has that pod. A miss usually means the cache has not yet
// refreshed since the pod was scheduled; callers should treat a miss as
// "use single-scale-set fallback" rather than as an error.
func (c *Cache) Lookup(podUID string) (jobcontext.ARCScaleSet, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.byUID[podUID]
	return s, ok
}

// Refresh re-reads every watched namespace and atomically swaps the cache.
// It is safe to call concurrently with Lookup.
func (c *Cache) Refresh(ctx context.Context) error {
	next := make(map[string]jobcontext.ARCScaleSet)
	for _, namespace := range c.namespaces {
		pods, err := c.lister.ListPodsInNamespace(ctx, namespace)
		if err != nil {
			// A single-namespace failure leaves prior entries in
			// place rather than dropping the whole scale-set map.
			c.logger.WarnContext(ctx, "arc_scale_set_namespace_list_failed",
				"namespace", namespace,
				"error", err,
			)
			continue
		}
		for _, pod := range pods {
			s := scaleSetFromPodLabels(pod.Labels)
			if s.IsZero() || pod.UID == "" {
				continue
			}
			next[pod.UID] = s
		}
	}
	c.mu.Lock()
	c.byUID = next
	c.revision++
	revision := c.revision
	c.mu.Unlock()
	c.logger.InfoContext(ctx, "arc_scale_set_cache_refreshed",
		"pod_count", len(next),
		"revision", revision,
	)
	return nil
}

// Run drives Refresh on the configured cadence until ctx is canceled. It
// performs one initial refresh synchronously so callers can rely on Lookup
// having content before Run returns its first tick.
func (c *Cache) Run(ctx context.Context) {
	// Best-effort initial fill; failures are logged inside Refresh.
	_ = c.Refresh(ctx)
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.Refresh(ctx)
		}
	}
}

// Revision returns a monotonic counter incremented on every successful
// Refresh. Useful for tests and for log correlation.
func (c *Cache) Revision() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revision
}

func scaleSetFromPodLabels(labels map[string]string) jobcontext.ARCScaleSet {
	if labels == nil {
		return jobcontext.ARCScaleSet{}
	}
	return jobcontext.ARCScaleSet{
		Namespace: labels[scaleSetNamespaceLabel],
		Name:      labels[scaleSetNameLabel],
	}
}
