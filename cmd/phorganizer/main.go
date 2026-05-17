package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zedage/phorganizer/internal/config"
	"github.com/zedage/phorganizer/internal/dedup"
	"github.com/zedage/phorganizer/internal/notify"
	"github.com/zedage/phorganizer/internal/organizer"
	"github.com/zedage/phorganizer/internal/scheduler"
	"github.com/zedage/phorganizer/internal/state"
)


func main() {
	configPath := flag.String("config", "", "Path to config file (YAML or JSON)")
	flag.Parse()

	// Setup structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("phorganizer starting")

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
	os.MkdirAll(cfg.StateDir, 0755)

	// Register organizer jobs
	for i, profile := range cfg.Organizer {
		for j, source := range profile.Sources {
			p := profile // capture
			s := source  // capture

			name := fmt.Sprintf("organizer-%d-source-%d", i, j)
			statePath := organizer.StatePath(cfg.StateDir, p.Target, s.Path)

			_, regErr := sched.Register(name, s.Schedule, func() {
				st := state.Load(statePath)
				result, duration := organizer.RunWithTime(p, s, st, statePath)

				// Send notification
				var ntfyClient *notify.Client
				if p.Ntfy != nil {
					ntfyClient = notify.New(p.Ntfy.URL)
				}
				title, msg := notify.FormatOrganizerSummary(s.Path, result.Processed, result.Skipped, result.Errors, duration)
				if ntfyClient != nil {
					ntfyClient.SendSummary(title, msg)
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
		job := dd // capture
		if !job.Enabled {
			continue
		}

		name := fmt.Sprintf("dedup-%d", i)

		_, regErr := sched.Register(name, job.Schedule, func() {
			start := time.Now()
			result := dedup.Run(job)
			duration := time.Since(start)

			// Send notification
			var ntfyClient *notify.Client
			if job.Ntfy != nil {
				ntfyClient = notify.New(job.Ntfy.URL)
			}
			title, msg := notify.FormatDedupSummary(result.TotalFiles, result.DuplicatesPair, result.ActionsApplied, result.Errors, duration)
			if ntfyClient != nil {
				ntfyClient.SendSummary(title, msg)
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

	// Block on signal for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh

	slog.Info("received signal, shutting down gracefully...", "signal", sig)
	sched.Stop()
	slog.Info("phorganizer stopped")
}
