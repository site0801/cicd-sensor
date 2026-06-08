package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cicd-sensor/cicd-sensor/internal/agent"
	"github.com/cicd-sensor/cicd-sensor/internal/agent/arcscaleset"
	"github.com/cicd-sensor/cicd-sensor/internal/agent/k8sclient"
	"github.com/cicd-sensor/cicd-sensor/internal/agent/listener"
	"github.com/cicd-sensor/cicd-sensor/internal/agent/managerclient"
	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
	"github.com/cicd-sensor/cicd-sensor/internal/slogid"
	"github.com/cicd-sensor/cicd-sensor/internal/version"
)

const (
	agentUsage      = "usage: cicd-sensor agent start [flags]"
	agentStartUsage = "usage: cicd-sensor agent start [flags]"
)

type agentStartOptions struct {
	Provider      string
	Runner        string
	ManagerURL    string
	ManagerToken  string
	SocketPath    string
	ShutdownGrace time.Duration

	// ARC carries options consumed only by Provider == "github-arc". The
	// zero value is correct for every other provider.
	ARC arcStartOptions
}

// arcStartOptions groups the github-arc-specific configuration. It is
// nested under agentStartOptions so the provider-specific surface stays
// out of the universal options namespace; future runner environments add
// their own grouped struct rather than flat fields here.
type arcStartOptions struct {
	// Namespaces is the list of Kubernetes namespaces hosting
	// AutoscalingRunnerSet resources whose runner pods this Agent should
	// classify by scale-set identity.
	Namespaces []string
}

func runAgentSubcommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, agentUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "start":
		runAgentStart(args)
	default:
		fmt.Fprintln(os.Stderr, agentUsage)
		os.Exit(2)
	}
}

func runAgentStart(args []string) {
	fs := flag.NewFlagSet("agent start", flag.ExitOnError)
	var socketPath string
	var provider string
	var runner string
	var managerURL string
	var managerTokenFilePath string
	var shutdownGrace time.Duration
	socketPath = defaultSocketPath
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), agentStartUsage)
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Required:")
		fmt.Fprintln(fs.Output(), "  --provider github|gitlab")
		fmt.Fprintln(fs.Output(), "        CI provider this host runs.")
		fmt.Fprintln(fs.Output(), "  --runner machine|kubernetes")
		fmt.Fprintln(fs.Output(), "        Runner type.")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Optional:")
		fmt.Fprintf(fs.Output(), "  --socket PATH\n        Agent control socket path. (default %q)\n", defaultSocketPath)
		fmt.Fprintln(fs.Output(), "  --manager-url URL")
		fmt.Fprintln(fs.Output(), "        Host scope manager URL. Required for host/start.")
		fmt.Fprintln(fs.Output(), "  CICD_SENSOR_MANAGER_TOKEN or --manager-token-file PATH")
		fmt.Fprintln(fs.Output(), "        Host scope manager bearer token. Required only when --manager-url is set.")
		fmt.Fprintln(fs.Output(), "  --shutdown-grace DURATION")
		fmt.Fprintln(fs.Output(), "        Best-effort drain window used after SIGTERM. (default 8s)")
	}
	fs.StringVar(&socketPath, "socket", socketPath, "Agent control socket path.")
	fs.StringVar(&provider, "provider", "", "CI provider this host runs (github or gitlab).")
	fs.StringVar(&runner, "runner", "", "Runner type (machine or kubernetes).")
	fs.StringVar(&managerURL, "manager-url", "", "Host scope manager URL.")
	fs.StringVar(&managerTokenFilePath, "manager-token-file", "", "Path to a file containing the host scope manager bearer token. Overrides CICD_SENSOR_MANAGER_TOKEN.")
	fs.DurationVar(&shutdownGrace, "shutdown-grace", 8*time.Second, "Best-effort drain window used after SIGTERM.")
	var arcNamespacesRaw string
	fs.StringVar(&arcNamespacesRaw, "arc-namespaces", "", "Comma-separated Kubernetes namespaces hosting AutoscalingRunnerSet resources. Required when --provider=github-arc.")
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, agentStartUsage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := newCLIJSONLogger()
	slog.SetDefault(logger)

	opts := agentStartOptions{
		Provider:      provider,
		Runner:        runner,
		ManagerURL:    managerURL,
		SocketPath:    socketPath,
		ShutdownGrace: shutdownGrace,
		ARC: arcStartOptions{
			Namespaces: parseARCNamespaces(arcNamespacesRaw),
		},
	}
	if err := validateAgentStartRequiredOptions(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "agent_started",
		"version", version.Current,
		"socket", opts.SocketPath,
		"provider", opts.Provider,
		"runner", opts.Runner,
	)

	var hostManager managerclient.Connection
	var hostManagerClient *managerclient.ConfigClient
	if opts.ManagerURL != "" {
		managerToken, err := resolveManagerTokenSecret(managerTokenFilePath, logger)
		if err != nil {
			slog.ErrorContext(ctx, "agent_failed", "error", err)
			os.Exit(1)
		}
		opts.ManagerToken = managerToken
		if err := validateAgentStartOptions(opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		hostManager = managerclient.Connection{BaseURL: opts.ManagerURL, Token: opts.ManagerToken}
		hostManagerClient, err = managerclient.NewConfigClient(logger, hostManager)
		if err != nil {
			slog.ErrorContext(ctx, "agent_failed", "error", err)
			os.Exit(1)
		}
		slog.InfoContext(ctx, "host_manager_client_enabled", "manager_url", hostManager.BaseURL)
	} else if managerTokenFilePath != "" {
		fmt.Fprintln(os.Stderr, "--manager-token-file requires --manager-url")
		os.Exit(1)
	}

	a := agent.NewAgent(logger, opts.SocketPath, jobcontext.Provider(opts.Provider), opts.Runner, hostManager, hostManagerClient)
	a.SetShutdownGrace(opts.ShutdownGrace)

	if err := configureProvider(ctx, a, opts, logger); err != nil {
		slog.ErrorContext(ctx, "agent_failed", "error", err)
		os.Exit(1)
	}

	if err := a.Run(ctx); err != nil {
		if errors.Is(err, listener.ErrAlreadyRunning) {
			slog.InfoContext(ctx, "agent_already_running", "socket", opts.SocketPath)
			return
		}
		slog.ErrorContext(ctx, "agent_failed", "error", err)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "agent_stopped")
}

func validateAgentStartOptions(opts agentStartOptions) error {
	if err := validateAgentStartRequiredOptions(opts); err != nil {
		return err
	}
	if opts.ManagerURL == "" {
		return nil
	}
	if opts.ManagerToken == "" {
		return fmt.Errorf("manager token is required: set CICD_SENSOR_MANAGER_TOKEN or --manager-token-file")
	}
	return nil
}

func validateAgentStartRequiredOptions(opts agentStartOptions) error {
	if opts.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	switch opts.Provider {
	case "github", "github-arc", "gitlab":
	default:
		return fmt.Errorf("provider must be github, github-arc, or gitlab")
	}
	if opts.Runner == "" {
		return fmt.Errorf("runner is required")
	}
	switch opts.Runner {
	case "machine", "kubernetes":
	default:
		return fmt.Errorf("runner must be machine or kubernetes")
	}
	if opts.Provider == "github-arc" && opts.Runner != "kubernetes" {
		return fmt.Errorf("provider github-arc requires --runner=kubernetes")
	}
	if opts.Provider == "github-arc" && len(opts.ARC.Namespaces) == 0 {
		return fmt.Errorf("provider github-arc requires --arc-namespaces with at least one namespace")
	}
	if opts.ShutdownGrace <= 0 {
		return fmt.Errorf("shutdown-grace must be positive")
	}
	return nil
}

// parseARCNamespaces splits a comma-separated value and drops empty entries.
func parseARCNamespaces(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// configureProvider applies provider-specific Agent configuration. It
// dispatches on the provider name and delegates to a provider-named
// helper; non-provider-specific setup (shutdown grace, manager client)
// stays in runAgentStart so the dispatch surface is small and adding a
// future provider variant follows the same shape.
func configureProvider(ctx context.Context, a *agent.Agent, opts agentStartOptions, logger *slog.Logger) error {
	switch opts.Provider {
	case "github-arc":
		return configureARC(ctx, a, opts.ARC, logger)
	}
	return nil
}

// configureARC wires the in-cluster Kubernetes client, the per-pod label
// cache, and the scale-set resolver. The cache's background refresh
// goroutine is started here so its lifetime is tied to the agent run's
// context; it returns when ctx is canceled.
func configureARC(ctx context.Context, a *agent.Agent, opts arcStartOptions, logger *slog.Logger) error {
	k8sCfg, err := k8sclient.InClusterConfig()
	if err != nil {
		return fmt.Errorf("load in-cluster config: %w", err)
	}
	client, err := k8sclient.New(k8sCfg)
	if err != nil {
		return fmt.Errorf("build k8s client: %w", err)
	}
	cache, err := arcscaleset.NewCache(logger, client, opts.Namespaces)
	if err != nil {
		return fmt.Errorf("build scale-set cache: %w", err)
	}
	resolver, err := arcscaleset.NewResolver(logger, cache)
	if err != nil {
		return fmt.Errorf("build scale-set resolver: %w", err)
	}
	a.SetARCScaleSetResolver(resolver)
	go cache.Run(ctx)
	logger.InfoContext(ctx, "arc_scale_set_resolver_enabled",
		"namespaces", opts.Namespaces,
	)
	return nil
}

func newCLIJSONLogger() *slog.Logger {
	return slog.New(slogid.Wrap(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano))
			}
			return a
		},
	}))).With("component", "cicd-sensor-agent")
}
