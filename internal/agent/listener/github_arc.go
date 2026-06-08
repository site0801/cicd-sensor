package listener

import (
	"net/http"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

// handleGitHubARCHostStart is the ARC variant of handleGitHubHostStart. It
// resolves the runner pod's scale-set identity from the peer's pod cgroup
// and attaches it to the request context, then delegates to the generic
// GitHub host-start handler. With the scale-set in context, the
// manager-config fetch inside JobRegistry can scope its FetchConfig
// request to the matching per-scale-set host scope configuration.
//
// This is the only GitHub route that needs ARC-specific pre-processing.
// host/end, job/health, project/start, and project/result all act on
// already-tracked Jobs whose scale-set was attached at host/start time.
func (l *Listener) handleGitHubARCHostStart(w http.ResponseWriter, r *http.Request) {
	peerPID, err := requestPeerPID(r.Context())
	if err != nil {
		l.logger.WarnContext(r.Context(), "peer_pid_unavailable", "error", err)
		l.writeError(w, r, http.StatusBadRequest, "peer pid unavailable")
		return
	}
	scaleSet, err := l.arcScaleSetResolver.Resolve(r.Context(), peerPID)
	if err != nil {
		l.logger.WarnContext(r.Context(), "arc_scale_set_resolve_failed",
			"error", err,
			"peer_pid", peerPID,
		)
		l.writeError(w, r, http.StatusInternalServerError, "arc scale-set resolve failed")
		return
	}
	ctx := r.Context()
	if !scaleSet.IsZero() {
		ctx = jobcontext.WithARCScaleSet(ctx, scaleSet)
	}
	l.handleGitHubHostStart(w, r.WithContext(ctx))
}
