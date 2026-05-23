// Package logkind names the three top-level job-log kinds shared by the
// agent (emitter) and the manager (router). Centralised so the agent's
// `log_type` JSON field and the manager's routing keys cannot drift.
package logkind

// LogKind identifies a top-level job log stream.
type LogKind string

const (
	JobDetection        LogKind = "job_detection_log"
	JobRuntimeTelemetry LogKind = "job_runtime_telemetry_log"
	JobResult           LogKind = "job_result_log"
)

// Schema versions per log kind. Bump on breaking changes (rename / retype /
// remove a field, or change a field's semantics). Do NOT bump for additive
// changes like adding a new column.
const (
	JobDetectionSchemaVersion        = "v1"
	JobRuntimeTelemetrySchemaVersion = "v1"
	JobResultSchemaVersion           = "v1"
)

// Parse returns the matching LogKind for value, or (zero, false) if value is
// not a known kind.
func Parse(value string) (LogKind, bool) {
	switch LogKind(value) {
	case JobDetection, JobRuntimeTelemetry, JobResult:
		return LogKind(value), true
	default:
		return "", false
	}
}
