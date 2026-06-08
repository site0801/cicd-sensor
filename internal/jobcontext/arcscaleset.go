package jobcontext

import "context"

// ARCScaleSet identifies one Actions Runner Controller `gha-runner-scale-set`
// release, used as the per-scale-set key when the Agent fetches host scope
// configuration from cicd-sensor-manager. The two fields mirror the labels
// the ARC controller writes onto every runner pod:
//
//	actions.github.com/scale-set-namespace
//	actions.github.com/scale-set-name
//
// The zero value represents single-scale-set mode: every job on the Agent
// shares one host scope configuration. Non-ARC runner environments always
// carry the zero value.
//
// Per-scale-set isolation is what cicd-sensor exposes today; the broader
// concept is multi-tenancy, but the wire and agent surface intentionally
// stay ARC-shaped until a second runner environment needs the same
// primitive.
type ARCScaleSet struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// IsZero reports whether the value carries no scale-set information.
func (s ARCScaleSet) IsZero() bool {
	return s.Namespace == "" && s.Name == ""
}

// arcScaleSetContextKey carries the resolved scale-set from the listener
// down to the manager-config fetch path. We use context propagation instead
// of threading the value through every JobRegistry call so the existing
// JobRegistry signatures stay stable.
type arcScaleSetContextKey struct{}

// WithARCScaleSet returns ctx augmented with the given scale-set identity.
// The value is later read by the manager-config fetch path to scope the
// per-scale-set FetchConfig request. The zero ARCScaleSet is the
// single-scale-set fallback.
func WithARCScaleSet(ctx context.Context, s ARCScaleSet) context.Context {
	return context.WithValue(ctx, arcScaleSetContextKey{}, s)
}

// ARCScaleSetFromContext returns the scale-set identity carried by ctx, or
// the zero ARCScaleSet if none has been attached. The zero return value is
// the signal to use single-scale-set host scope config.
func ARCScaleSetFromContext(ctx context.Context) ARCScaleSet {
	if ctx == nil {
		return ARCScaleSet{}
	}
	s, _ := ctx.Value(arcScaleSetContextKey{}).(ARCScaleSet)
	return s
}
