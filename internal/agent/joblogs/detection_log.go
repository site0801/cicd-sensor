package joblogs

import (
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cicd-sensor/cicd-sensor/internal/agent/observations"
	"github.com/cicd-sensor/cicd-sensor/internal/jobevent"
	"github.com/cicd-sensor/cicd-sensor/internal/logkind"
	logv1 "github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/log/v1"
	"github.com/cicd-sensor/cicd-sensor/internal/protoconv"
	"github.com/cicd-sensor/cicd-sensor/internal/version"
)

type DetectionLogInput struct {
	ScopeLogContext
	Hit                 *observations.HitEntry
	Event               jobevent.EventRecord
	RuleName            string
	RuleDescription     string
	RulesetRevision     string
	RuleAlertTruncation string
}

func MarshalDetectionLogEntry(in DetectionLogInput) ([]byte, error) {
	if in.Hit == nil {
		return nil, nil
	}
	rulesetRevision := in.Hit.RulesetRevision
	if rulesetRevision == "" {
		rulesetRevision = in.RulesetRevision
	}
	timestamp := in.Event.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	message := &logv1.JobDetectionLogEntry{
		Timestamp:           timestamppb.New(timestamp.UTC()),
		LogType:             proto.String(string(logkind.JobDetection)),
		SchemaVersion:       proto.String(logkind.JobDetectionSchemaVersion),
		AgentVersion:        proto.String(version.Current),
		LogId:               proto.String(newLogID()),
		Job:                 protoconv.ToJobLogContext(in.Identity, in.Metadata),
		Scope:               proto.String(string(in.Scope)),
		RunnerKind:          proto.String(in.RunnerKind),
		RulesetId:           proto.String(in.Hit.Identity.RulesetID),
		RuleId:              proto.String(in.Hit.Identity.RuleID),
		RulesetRevision:     proto.String(rulesetRevision),
		RuleName:            proto.String(in.RuleName),
		RuleDescription:     proto.String(in.RuleDescription),
		Action:              proto.String(in.Hit.Action),
		RuleAlertTruncation: proto.String(in.RuleAlertTruncation),
		Event:               sanitizedLogEventRecord(in.Event),
	}
	return logJSONMarshal.Marshal(message)
}
