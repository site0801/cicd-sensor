package manager

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	managerv1beta1 "github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/manager/v1beta1"
	"github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/manager/v1beta1/managerv1beta1connect"
	"github.com/cicd-sensor/cicd-sensor/internal/rule/baseline"
)

// Server is the manager server that exposes Connect RPCs.
type Server struct {
	logger        *slog.Logger
	tokens        *TokenStore
	config        *ServedConfig
	baselineRules BaselineRuleSource
	localRules    *RuleSourceCache
	startup       *StartupConfig
	httpServer    *http.Server

	// arcScaleSetEntries holds resolved per-scale-set overrides, keyed by
	// (namespace, name). FetchConfig consults this map first and falls
	// back to config / localRules when no entry matches.
	arcScaleSetEntries map[ARCScaleSetKey]*arcScaleSetEntry

	outputRouter *OutputRouter
	now          func() time.Time
}

// arcScaleSetEntry is one per-scale-set override resolved against the
// global ServedConfig at startup. localRules is nil when the entry does
// not declare its own rules_file; in that case the server's default
// localRules cache is reused.
type arcScaleSetEntry struct {
	config     *ServedConfig
	localRules *RuleSourceCache
}

// NewServer creates a manager server mounted on addr. Baseline and local rule
// caches are owned here so callers only pass startup intent, not cache objects.
func NewServer(logger *slog.Logger, addr string, tokens []string, config *ServedConfig, rulesPath string, startup *StartupConfig, router *OutputRouter) *Server {
	return newServer(logger, addr, tokens, config, baseline.NewCache(), NewRuleSourceCache(rulesPath), startup, router)
}

func newServer(logger *slog.Logger, addr string, tokens []string, config *ServedConfig, baselineRules BaselineRuleSource, localRules *RuleSourceCache, startup *StartupConfig, router *OutputRouter) *Server {
	s := &Server{
		logger:             logger.With("component", "manager"),
		tokens:             NewTokenStore(tokens),
		config:             config,
		baselineRules:      baselineRules,
		localRules:         localRules,
		startup:            startup,
		arcScaleSetEntries: buildARCScaleSetEntries(config, startup),
		outputRouter:       router,
		now:                time.Now,
	}

	mux := http.NewServeMux()
	configPath, configHandler := managerv1beta1connect.NewConfigServiceHandler(
		newConfigServiceHandler(s),
		connect.WithReadMaxBytes(managerMaxRequestBytes),
		connect.WithInterceptors(unaryOnlyInterceptor{}),
	)
	mux.Handle(configPath, configHandler)
	collectorPath, collectorHandler := managerv1beta1connect.NewCollectorServiceHandler(
		newCollectorServiceHandler(s),
		connect.WithReadMaxBytes(managerMaxRequestBytes),
		connect.WithInterceptors(unaryOnlyInterceptor{}),
	)
	mux.Handle(collectorPath, collectorHandler)

	// Auth is enforced at the HTTP layer via connectrpc/authn middleware so
	// unauthenticated requests are rejected before the Connect framework
	// decompresses or unmarshals the body. unaryOnlyInterceptor is a separate
	// defense-in-depth guard on the Connect handlers.
	authMiddleware := newAuthMiddleware(s.logger, s.tokens)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: authMiddleware.Wrap(mux),
		// ReadHeaderTimeout bounds slowloris-style header stalls; the full
		// ReadTimeout then bounds the body read.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return s
}

// managerMaxRequestBytes caps the Connect request body for unary RPCs. 16 MiB
// comfortably covers FetchConfig payloads (typed rule sources) while
// keeping DoS surface bounded.
const managerMaxRequestBytes = 16 * 1024 * 1024

// Handler exposes the composed http.Handler that carries the Connect
// service mounts and interceptors. Intended for integration tests that need
// to bypass the listener (e.g. httptest.NewServer).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Run starts the HTTP server and blocks until ctx is canceled.
func (s *Server) Run(ctx context.Context) error {
	defer func() {
		if err := s.Close(); err != nil && s.logger != nil {
			s.logger.ErrorContext(ctx, "manager_output_router_close_failed", "error", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		s.logger.InfoContext(ctx, "manager_server_started", "addr", s.httpServer.Addr)
		err := s.httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.ErrorContext(ctx, "manager_shutdown_request_abandoned", "error", err)
			return fmt.Errorf("server shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) Close() error {
	if s == nil || s.outputRouter == nil {
		return nil
	}
	return s.outputRouter.Close()
}

// buildARCScaleSetEntries materializes the per-scale-set overrides
// declared in startup.ARCScaleSets against the global ServedConfig. The
// returned map is keyed by (namespace, name) for O(1) lookup from
// FetchConfig. nil startup or empty overrides return nil so the lookup
// site can detect "no per-scale-set entries configured" cheaply.
func buildARCScaleSetEntries(global *ServedConfig, startup *StartupConfig) map[ARCScaleSetKey]*arcScaleSetEntry {
	if startup == nil || len(startup.ARCScaleSets) == 0 {
		return nil
	}
	out := make(map[ARCScaleSetKey]*arcScaleSetEntry, len(startup.ARCScaleSets))
	for _, override := range startup.ARCScaleSets {
		key := ARCScaleSetKey{Namespace: override.Namespace, Name: override.Name}
		entry := &arcScaleSetEntry{
			config: applyARCScaleSetOverride(global, override),
		}
		if override.RulesFile != "" {
			entry.localRules = NewRuleSourceCache(override.RulesFile)
		}
		out[key] = entry
	}
	return out
}

// resolveARCScaleSetEntry returns the ServedConfig and RuleSourceCache
// to serve for a FetchConfig request. When scaleSet matches a configured
// per-scale-set override, the override's resolved config and (if
// declared) rules cache are returned. Otherwise the server's global
// defaults are returned. The third return value is true when a match
// was made; callers use it for diagnostic logging.
func (s *Server) resolveARCScaleSetEntry(scaleSet *managerv1beta1.ARCScaleSet) (*ServedConfig, *RuleSourceCache, bool) {
	if scaleSet == nil || s.arcScaleSetEntries == nil {
		return s.config, s.localRules, false
	}
	key := ARCScaleSetKey{Namespace: scaleSet.Namespace, Name: scaleSet.Name}
	entry, ok := s.arcScaleSetEntries[key]
	if !ok {
		return s.config, s.localRules, false
	}
	rules := entry.localRules
	if rules == nil {
		rules = s.localRules
	}
	return entry.config, rules, true
}

// applyARCScaleSetOverride returns a new ServedConfig with the override's
// non-nil fields applied on top of the global config. The returned
// pointer is independent of the global so concurrent FetchConfig calls
// for different scale sets cannot observe a partially-mutated value.
func applyARCScaleSetOverride(global *ServedConfig, override ARCScaleSetConfig) *ServedConfig {
	var out ServedConfig
	if global != nil {
		out = *global
	}
	if override.DefaultMaxAlertsPerRule != nil {
		out.DefaultMaxAlertsPerRule = *override.DefaultMaxAlertsPerRule
	}
	if override.DisableBaselineRules != nil {
		out.DisableBaselineRules = *override.DisableBaselineRules
	}
	if override.MonitorMode != nil {
		out.MonitorMode = *override.MonitorMode
	}
	return &out
}
