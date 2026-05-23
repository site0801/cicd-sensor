package joblogs

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/cicd-sensor/cicd-sensor/internal/logkind"
	logv1 "github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/log/v1"
	"github.com/cicd-sensor/cicd-sensor/internal/version"
)

func TestMarshalRuntimeTelemetryLogEntrySanitizesEventProcess(t *testing.T) {
	t.Parallel()

	payload, err := MarshalRuntimeTelemetryLogEntry(RuntimeTelemetryLogInput{
		ScopeLogContext: testScopeLogContext(),
		Event:           eventWithSecretArgv(),
	})
	if err != nil {
		t.Fatalf("marshal runtime telemetry log: %v", err)
	}

	var got logv1.JobRuntimeTelemetryLogEntry
	if err := protojson.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal runtime telemetry log: %v", err)
	}
	assertProtoEventProcessSanitized(t, got.GetEvent())
}

func TestMarshalRuntimeTelemetryLogEntryStampsLogTypeAndVersions(t *testing.T) {
	t.Parallel()

	payload, err := MarshalRuntimeTelemetryLogEntry(RuntimeTelemetryLogInput{
		ScopeLogContext: testScopeLogContext(),
		Event:           eventWithSecretArgv(),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got logv1.JobRuntimeTelemetryLogEntry
	if err := protojson.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GetLogType() != string(logkind.JobRuntimeTelemetry) {
		t.Errorf("log_type: got %q, want %q", got.GetLogType(), string(logkind.JobRuntimeTelemetry))
	}
	if got.GetSchemaVersion() != "v1" {
		t.Errorf("schema_version: got %q, want %q", got.GetSchemaVersion(), "v1")
	}
	if got.GetAgentVersion() != version.Current {
		t.Errorf("agent_version: got %q, want %q", got.GetAgentVersion(), version.Current)
	}
}
