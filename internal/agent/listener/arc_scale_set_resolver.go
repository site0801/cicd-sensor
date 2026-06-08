package listener

import (
	"context"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

// ARCScaleSetResolver maps a runner-pod peer PID to the ARC scale-set that
// owns the pod.
//
// The interface is defined in the listener package so the ARC routes can
// take it as a dependency without dragging Kubernetes client types into the
// listener wire. The real implementation lives in
// internal/agent/arcscaleset.
type ARCScaleSetResolver interface {
	// Resolve returns the scale-set identity for the runner pod containing
	// peerPID. A zero ARCScaleSet with err == nil means single-scale-set
	// fallback should be used (either the peer is not in a Kubernetes pod,
	// or no per-scale-set configuration has been delivered for this scale
	// set).
	Resolve(ctx context.Context, peerPID int32) (jobcontext.ARCScaleSet, error)
}

// nilARCScaleSetResolver is the single-scale-set fallback used when no ARC
// resolver is configured.
type nilARCScaleSetResolver struct{}

// NewNilARCScaleSetResolver returns a resolver that always reports the zero
// ARCScaleSet, collapsing every job onto a single host scope. Use it as the
// default for non-ARC providers and for ARC deployments that have not yet
// configured per-scale-set isolation.
func NewNilARCScaleSetResolver() ARCScaleSetResolver { return nilARCScaleSetResolver{} }

// Resolve always returns the zero ARCScaleSet.
func (nilARCScaleSetResolver) Resolve(context.Context, int32) (jobcontext.ARCScaleSet, error) {
	return jobcontext.ARCScaleSet{}, nil
}
