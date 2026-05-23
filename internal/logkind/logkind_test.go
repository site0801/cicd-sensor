package logkind

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  LogKind
		ok    bool
	}{
		{name: "job detection", input: "job_detection_log", want: JobDetection, ok: true},
		{name: "runtime telemetry", input: "job_runtime_telemetry_log", want: JobRuntimeTelemetry, ok: true},
		{name: "job result", input: "job_result_log", want: JobResult, ok: true},
		{name: "unknown", input: "detection", ok: false},
		{name: "empty", input: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok: got %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("kind: got %q, want %q", got, tt.want)
			}
		})
	}
}
