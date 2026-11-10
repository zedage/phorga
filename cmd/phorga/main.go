package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/dedup"
	"github.com/zedage/phorga/internal/notify"
	"github.com/zedage/phorga/internal/organizer"
	"github.com/zedage/phorga/internal/scheduler"
	"github.com/zedage/phorga/internal/state"
	"github.com/zedage/phorga/internal/webui"
)

// Exit codes returned by run.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const uiShutdownTimeout = 5 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

// run starts Phorga and blocks until ctx is cancelled (normally by SIGINT or
// SIGTERM) or the web UI fails. It returns the process exit code. Logs are
// written as JSON to logOut. The configuration is loaded once at startup and
// never reloaded (CFG-005).
func run(ctx context.Context, args []string, logOut io.Writer) int {
	flags := flag.NewFlagSet("phorga", flag.ContinueOnError)
	flags.SetOutput(logOut)
	configPath := flags.String("config", "", "Path to config file (YAML or JSON)")
	webUIFlag := flags.Bool("web-ui", false, "Enable the built-in configuration web UI (overrides config)")
	webUIAddr := flags.String("web-ui-addr", "", "Listen address for the web UI (overrides config, e.g. 127.0.0.1:8080)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("phorga starting")

	cfgPath, cfg, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("startup failed", "error", err)
		return exitError
	}
	slog.Info("config loaded", "path", cfgPath, "stateDir", cfg.StateDir)

	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		slog.Error("failed to create state directory", "path", cfg.StateDir, "error", err)
		return exitError
	}

	sched := scheduler.New()
	if err := registerJobs(sched, cfg); err != nil {
		slog.Error("failed to register jobs", "error", err)
		return exitError
	}
	sched.Start()
	defer sched.Stop()
	slog.Info("scheduler started, waiting for jobs...")

	var ui *webui.Server
	uiErrCh := make(chan error, 1)
	if webUIEnabled(*webUIFlag, cfg) {
		ui, err = webui.New(webui.Options{Address: *webUIAddr, ConfigPath: cfgPath, Config: cfg})
		if err != nil {
			slog.Error("web UI setup failed", "error", err)
			return exitError
		}
		go func() { uiErrCh <- ui.Start() }()
	}

	exitCode := exitOK
	select {
	case <-ctx.Done():
		slog.Info("shutdown requested, stopping gracefully...")
	case uiErr := <-uiErrCh:
		if uiErr != nil {
			slog.Error("web UI stopped unexpectedly", "error", uiErr)
			exitCode = exitError
		}
	}

	if ui != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), uiShutdownTimeout)
		if err := ui.Shutdown(shutdownCtx); err != nil {
			slog.Error("web UI shutdown failed", "error", err)
		}
		cancel()
	}
	slog.Info("phorga stopped")
	return exitCode
}

// loadConfig resolves, loads and validates the configuration (CFG-001..003).
func loadConfig(cliPath string) (string, *config.Config, error) {
	path, err := config.ResolveConfigPath(cliPath)
	if err != nil {
		return "", nil, fmt.Errorf("config resolution failed: %w", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", nil, fmt.Errorf("config loading failed: %w", err)
	}
	if err := config.Validate(cfg); err != nil {
		return "", nil, err
	}
	return path, cfg, nil
}

// webUIEnabled reports whether the web UI should start: via --web-ui or
// webUI.enabled in the config (WEB-001).
func webUIEnabled(flagSet bool, cfg *config.Config) bool {
	return flagSet || (cfg.WebUI != nil && cfg.WebUI.Enabled)
}

// registerJobs registers one job per organizer source and one per enabled
// duplicate detection job (ORG-010, DUP-008).
func registerJobs(sched *scheduler.Scheduler, cfg *config.Config) error {
	for i, profile := range cfg.Organizer {
		for j, source := range profile.Sources {
			p, s := profile, source
			name := fmt.Sprintf("organizer-%d-source-%d", i, j)
			statePath := organizer.StatePath(cfg.StateDir, p.Target, s.Path)

			if _, err := sched.Register(name, s.Schedule, func() {
				st := state.Load(statePath)
				result, duration := organizer.RunWithTime(p, s, st, statePath)
				if p.Ntfy != nil {
					title, msg := notify.FormatOrganizerSummary(s.Path, result.Processed, result.Skipped, result.Errors, duration)
					notify.New(p.Ntfy.URL).SendSummary(title, msg)
				}
			}); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}

	for i, dd := range cfg.DuplicateDetection {
		job := dd
		if !job.Enabled {
			continue
		}
		name := fmt.Sprintf("dedup-%d", i)

		if _, err := sched.Register(name, job.Schedule, func() {
			start := time.Now()
			result := dedup.Run(job)
			if job.Ntfy != nil {
				title, msg := notify.FormatDedupSummary(result.TotalFiles, result.DuplicatesPair, result.ActionsApplied, result.Errors, time.Since(start))
				notify.New(job.Ntfy.URL).SendSummary(title, msg)
			}
		}); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}
