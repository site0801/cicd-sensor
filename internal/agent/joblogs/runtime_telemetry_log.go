package joblogs

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cicd-sensor/cicd-sensor/internal/jobevent"
	"github.com/cicd-sensor/cicd-sensor/internal/logkind"
	logv1 "github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/log/v1"
	"github.com/cicd-sensor/cicd-sensor/internal/protoconv"
	"github.com/cicd-sensor/cicd-sensor/internal/version"
)

type RuntimeTelemetryLogInput struct {
	ScopeLogContext
	Event jobevent.EventRecord
}

func MarshalRuntimeTelemetryLogEntry(in RuntimeTelemetryLogInput) ([]byte, error) {
	message := &logv1.JobRuntimeTelemetryLogEntry{
		Timestamp:     timestamppb.New(in.Event.Timestamp.UTC()),
		LogType:       proto.String(string(logkind.JobRuntimeTelemetry)),
		SchemaVersion: proto.String(logkind.JobRuntimeTelemetrySchemaVersion),
		AgentVersion:  proto.String(version.Current),
		LogId:         proto.String(newLogID()),
		Job:           protoconv.ToJobLogContext(in.Identity, in.Metadata),
		Scope:         proto.String(string(in.Scope)),
		RunnerKind:    proto.String(in.RunnerKind),
		Event:         sanitizedLogEventRecord(in.Event),
	}
	return logJSONMarshal.Marshal(message)
}
