package sink

import (
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/logkind"
)

func TestPubSubFlushPolicy(t *testing.T) {
	tests := []struct {
		name    string
		logKind logkind.LogKind
		want    FlushPolicy
	}{
		{
			name:    "detection is immediate",
			logKind: logkind.JobDetection,
			want:    FlushPolicy{FlushThresholdBytes: 1, FlushIntervalSeconds: 1},
		},
		{
			name:    "telemetry batches briefly",
			logKind: logkind.JobRuntimeTelemetry,
			want:    FlushPolicy{FlushThresholdBytes: 256 * 1024, FlushIntervalSeconds: 5},
		},
		{
			name:    "result is immediate",
			logKind: logkind.JobResult,
			want:    FlushPolicy{FlushThresholdBytes: 1, FlushIntervalSeconds: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (&pubsubSink{}).FlushPolicy(tt.logKind); got != tt.want {
				t.Fatalf("FlushPolicy(%q): got %+v, want %+v", tt.logKind, got, tt.want)
			}
		})
	}
}
