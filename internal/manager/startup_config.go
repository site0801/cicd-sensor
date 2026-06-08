package manager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/cicd-sensor/cicd-sensor/internal/logtype"
	"github.com/cicd-sensor/cicd-sensor/internal/rule"
	"go.yaml.in/yaml/v4"
)

type startupBindConfig struct {
	Address string `yaml:"address"`
	Port    *int   `yaml:"port"`
}

const (
	defaultBindAddress = "0.0.0.0"
	defaultBindPort    = 8080
)

// StartupConfig is the manager startup configuration loaded from manager.yaml.
type StartupConfig struct {
	Revision                string            `yaml:"-"`
	Bind                    startupBindConfig `yaml:"bind"`
	DefaultMaxAlertsPerRule int               `yaml:"default_max_alerts_per_rule,omitempty"`
	DisableBaselineRules    bool              `yaml:"disable_baseline_rules,omitempty"`
	MonitorMode             bool              `yaml:"monitor_mode,omitempty"`
	Sinks                   SinksConfig       `yaml:"sinks,omitempty"`
	Logs                    LogsConfig        `yaml:"logs,omitempty"`
	// ARCScaleSets is an optional list of per-scale-set overrides. When a
	// FetchConfig request carries an arc_scale_set that matches one of
	// these entries by (namespace, name), the manager serves the entry's
	// resolved configuration; otherwise it falls back to the global
	// defaults above. Entries with no overrides are equivalent to the
	// global defaults and do not need to be listed.
	ARCScaleSets []ARCScaleSetConfig `yaml:"arc_scale_sets,omitempty"`
}

// ARCScaleSetConfig is a per-scale-set override of the manager's served
// configuration. Unset pointer fields inherit from the global StartupConfig
// values; this lets an entry declare "same as global except for these
// specific fields" without restating the entire configuration.
type ARCScaleSetConfig struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`

	// DefaultMaxAlertsPerRule overrides the global default when non-nil.
	DefaultMaxAlertsPerRule *int `yaml:"default_max_alerts_per_rule,omitempty"`
	// DisableBaselineRules overrides the global value when non-nil.
	DisableBaselineRules *bool `yaml:"disable_baseline_rules,omitempty"`
	// MonitorMode overrides the global value when non-nil.
	MonitorMode *bool `yaml:"monitor_mode,omitempty"`
	// RulesFile overrides the global --rules-file path for jobs in this
	// scale set. Empty means inherit the global path.
	RulesFile string `yaml:"rules_file,omitempty"`
}

// SinksConfig maps an operator-defined sink name to its physical destination.
type SinksConfig map[string]SinkConfig

// SinkConfig describes one physical manager output destination.
type SinkConfig struct {
	Type      string `yaml:"type"`
	URI       string `yaml:"uri,omitempty"`
	Region    string `yaml:"region,omitempty"`
	ProjectID string `yaml:"project_id,omitempty"`
	Topic     string `yaml:"topic,omitempty"`
}

// LogsConfig maps a log type to one sink name.
type LogsConfig map[string]LogOutput

// LogOutput selects the named sink for one manager-ingested log.
type LogOutput struct {
	Sink string `yaml:"sink"`
}

// LoadStartupConfig reads and validates the manager startup config file.
func LoadStartupConfig(path string) (StartupConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StartupConfig{}, fmt.Errorf("read startup config %s: %w", path, err)
	}

	var cfg StartupConfig
	if err := yaml.Load(data, &cfg, yaml.WithKnownFields()); err != nil {
		return StartupConfig{}, fmt.Errorf("parse startup config %s: %w", path, err)
	}
	if cfg.Bind.Address == "" {
		cfg.Bind.Address = defaultBindAddress
	}
	if cfg.Bind.Port == nil {
		port := defaultBindPort
		cfg.Bind.Port = &port
	}
	if *cfg.Bind.Port < 0 || *cfg.Bind.Port > 65535 {
		return StartupConfig{}, fmt.Errorf("bind.port must be between 0 and 65535")
	}
	if err := rule.ValidateMaxAlertsBound(
		cfg.DefaultMaxAlertsPerRule,
		"default_max_alerts_per_rule",
	); err != nil {
		return StartupConfig{}, err
	}
	if err := validateSinks(cfg.Sinks); err != nil {
		return StartupConfig{}, err
	}
	if err := validateLogs(cfg.Logs, cfg.Sinks); err != nil {
		return StartupConfig{}, err
	}
	if err := validateARCScaleSets(cfg.ARCScaleSets); err != nil {
		return StartupConfig{}, err
	}
	sum := sha256.Sum256(data)
	cfg.Revision = "sha256:" + hex.EncodeToString(sum[:])
	return cfg, nil
}

func validateARCScaleSets(entries []ARCScaleSetConfig) error {
	seen := make(map[ARCScaleSetKey]struct{}, len(entries))
	for i, entry := range entries {
		if strings.TrimSpace(entry.Namespace) == "" {
			return fmt.Errorf("arc_scale_sets[%d].namespace must not be empty", i)
		}
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("arc_scale_sets[%d].name must not be empty", i)
		}
		key := ARCScaleSetKey{Namespace: entry.Namespace, Name: entry.Name}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("arc_scale_sets[%d]: duplicate selector (namespace=%q, name=%q)", i, entry.Namespace, entry.Name)
		}
		seen[key] = struct{}{}
		if entry.DefaultMaxAlertsPerRule != nil {
			if err := rule.ValidateMaxAlertsBound(
				*entry.DefaultMaxAlertsPerRule,
				fmt.Sprintf("arc_scale_sets[%d].default_max_alerts_per_rule", i),
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// ARCScaleSetKey identifies one ARC scale-set entry inside StartupConfig.
// It mirrors the proto ARCScaleSet on the wire and is the map key for
// per-scale-set entries inside the Server.
type ARCScaleSetKey struct {
	Namespace string
	Name      string
}

// BindAddress returns the net/http listen address represented by bind config.
func (cfg StartupConfig) BindAddress() string {
	return net.JoinHostPort(cfg.Bind.Address, strconv.Itoa(*cfg.Bind.Port))
}

func validateSinks(sinks SinksConfig) error {
	for name, sc := range sinks {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("sinks: name must not be empty")
		}
		switch sc.Type {
		case "aws_s3":
			if err := validateS3Sink(name, sc); err != nil {
				return err
			}
		case "google_storage":
			if err := validateGCSSink(name, sc); err != nil {
				return err
			}
		case "google_pubsub":
			if err := validatePubSubSink(name, sc); err != nil {
				return err
			}
		default:
			return fmt.Errorf("sinks.%s.type %q is not one of aws_s3/google_storage/google_pubsub", name, sc.Type)
		}
	}
	return nil
}

func validateS3Sink(name string, sc SinkConfig) error {
	if sc.URI == "" {
		return fmt.Errorf("sinks.%s.uri is required", name)
	}
	if !strings.HasPrefix(sc.URI, "s3://") {
		return fmt.Errorf("sinks.%s.uri must start with s3://", name)
	}
	if sc.Region == "" {
		return fmt.Errorf("sinks.%s.region is required for aws_s3", name)
	}
	if sc.ProjectID != "" || sc.Topic != "" {
		return fmt.Errorf("sinks.%s: project_id and topic are only valid for google_pubsub", name)
	}
	return nil
}

func validateGCSSink(name string, sc SinkConfig) error {
	if sc.URI == "" {
		return fmt.Errorf("sinks.%s.uri is required", name)
	}
	if !strings.HasPrefix(sc.URI, "gs://") {
		return fmt.Errorf("sinks.%s.uri must start with gs://", name)
	}
	if sc.Region != "" || sc.ProjectID != "" || sc.Topic != "" {
		return fmt.Errorf("sinks.%s: region, project_id, and topic are not valid for google_storage", name)
	}
	return nil
}

func validatePubSubSink(name string, sc SinkConfig) error {
	if sc.ProjectID == "" {
		return fmt.Errorf("sinks.%s.project_id is required for google_pubsub", name)
	}
	if sc.Topic == "" {
		return fmt.Errorf("sinks.%s.topic is required for google_pubsub", name)
	}
	if sc.Region != "" || sc.URI != "" {
		return fmt.Errorf("sinks.%s: region and uri are not valid for google_pubsub", name)
	}
	return nil
}

func validateLogs(logs LogsConfig, sinks SinksConfig) error {
	for logName, logOutput := range logs {
		if !knownOutputKind(logName) {
			return fmt.Errorf("logs.%s: unknown log key", logName)
		}
		if strings.TrimSpace(logOutput.Sink) == "" {
			return fmt.Errorf("logs.%s.sink: sink name is required", logName)
		}
		if _, ok := sinks[logOutput.Sink]; !ok {
			return fmt.Errorf("logs.%s.sink %q is not a defined sink name", logName, logOutput.Sink)
		}
	}
	return nil
}

func knownOutputKind(logName string) bool {
	_, ok := logtype.Parse(logName)
	return ok
}
