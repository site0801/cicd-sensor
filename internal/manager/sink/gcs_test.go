package sink

import (
	"errors"
	"testing"

	"google.golang.org/api/googleapi"

	"github.com/cicd-sensor/cicd-sensor/internal/logkind"
)

func TestGCSFlushPolicy(t *testing.T) {
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
			name:    "telemetry batches for object storage",
			logKind: logkind.JobRuntimeTelemetry,
			want:    FlushPolicy{FlushThresholdBytes: 128 * 1024 * 1024, FlushIntervalSeconds: 60},
		},
		{
			name:    "result is immediate",
			logKind: logkind.JobResult,
			want:    FlushPolicy{FlushThresholdBytes: 1, FlushIntervalSeconds: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (&gcsSink{}).FlushPolicy(tt.logKind); got != tt.want {
				t.Fatalf("FlushPolicy(%q): got %+v, want %+v", tt.logKind, got, tt.want)
			}
		})
	}
}

func TestIsGCSThrottle(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "429", err: &googleapi.Error{Code: 429}, want: true},
		{name: "500", err: &googleapi.Error{Code: 500}},
		{name: "plain error", err: errors.New("throttled")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGCSThrottle(tt.err); got != tt.want {
				t.Fatalf("isGCSThrottle: got %v, want %v", got, tt.want)
			}
		})
	}
}
