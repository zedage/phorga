package main

import (
	"context"
	"flag"
	"fmt"
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

func main() {
	configPath := flag.String("config", "", "Path to config file (YAML or JSON)")
	webUIFlag := flag.Bool("web-ui", false, "Enable the built-in configuration web UI (overrides config)")
	webUIAddr := flag.String("web-ui-addr", "", "Listen address for the web UI (overrides config, e.g. 127.0.0.1:8080)")
	flag.Parse()

	// Setup structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("phorga starting")

	// Resolve config path
	cfgPath, err := config.ResolveConfigPath(*configPath)
	if err != nil {
		slog.Error("config resolution failed", "error", err)
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("config loading failed", "error", err)
		os.Exit(1)
	}

	// Validate config
	if err := config.Validate(cfg); err != nil {
		slog.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded", "path", cfgPath, "stateDir", cfg.StateDir)

	// Create scheduler
	sched := scheduler.New()

	// Ensure state directory exists
	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		slog.Error("failed to create state directory", "path", cfg.StateDir, "error", err)
		os.Exit(1)
	}

	// Register organizer jobs
	for i, profile := range cfg.Organizer {
		for j, source := range profile.Sources {
			p := profile
			s := source

			name := fmt.Sprintf("organizer-%d-source-%d", i, j)
			statePath := organizer.StatePath(cfg.StateDir, p.Target, s.Path)

			_, regErr := sched.Register(name, s.Schedule, func() {
				st := state.Load(statePath)
				result, duration := organizer.RunWithTime(p, s, st, statePath)

				if p.Ntfy != nil {
					title, msg := notify.FormatOrganizerSummary(s.Path, result.Processed, result.Skipped, result.Errors, duration)
					notify.New(p.Ntfy.URL).SendSummary(title, msg)
				}
			})
			if regErr != nil {
				slog.Error("failed to register organizer job", "name", name, "error", regErr)
				os.Exit(1)
			}
		}
	}

	// Register duplicate detection jobs
	for i, dd := range cfg.DuplicateDetection {
		job := dd
		if !job.Enabled {
			continue
		}

		name := fmt.Sprintf("dedup-%d", i)

		_, regErr := sched.Register(name, job.Schedule, func() {
			start := time.Now()
			result := dedup.Run(job)
			duration := time.Since(start)

			if job.Ntfy != nil {
				title, msg := notify.FormatDedupSummary(result.TotalFiles, result.DuplicatesPair, result.ActionsApplied, result.Errors, duration)
				notify.New(job.Ntfy.URL).SendSummary(title, msg)
			}
		})
		if regErr != nil {
			slog.Error("failed to register dedup job", "name", name, "error", regErr)
			os.Exit(1)
		}
	}

	// Start scheduler
	sched.Start()
	slog.Info("scheduler started, waiting for jobs...")

	// Optional web UI: enabled via config or --web-ui flag
	var ui *webui.Server
	uiEnabled := *webUIFlag || (cfg.WebUI != nil && cfg.WebUI.Enabled)
	uiErrCh := make(chan error, 1)
	if uiEnabled {
		ui, err = webui.New(webui.Options{
			Address:    *webUIAddr,
			ConfigPath: cfgPath,
			Config:     cfg,
		})
		if err != nil {
			slog.Error("web UI setup failed", "error", err)
			sched.Stop()
			os.Exit(1)
		}
		go func() {
			uiErrCh <- ui.Start()
		}()
	}

	// Block on signal (or web UI failure) for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	exitCode := 0
	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down gracefully...", "signal", sig)
	case uiErr := <-uiErrCh:
		if uiErr != nil {
			slog.Error("web UI stopped unexpectedly", "error", uiErr)
			exitCode = 1
		}
	}

	if ui != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := ui.Shutdown(ctx); err != nil {
			slog.Error("web UI shutdown failed", "error", err)
		}
		cancel()
	}
	sched.Stop()
	slog.Info("phorga stopped")
	os.Exit(exitCode)
}
