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
	"strings"
	"syscall"
	"time"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/dedup"
	"github.com/zedage/phorga/internal/notify"
	"github.com/zedage/phorga/internal/organizer"
	"github.com/zedage/phorga/internal/runstatus"
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
	// After the first signal, restore the default signal handling, so a
	// second Ctrl+C ends the process at once instead of being swallowed
	// while running jobs stop (RUN-001).
	go func() {
		<-ctx.Done()
		stop()
	}()
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
	recorder := runstatus.Load(runstatus.Path(cfg.StateDir))
	jobs, err := registerJobs(sched, cfg, recorder)
	if err != nil {
		slog.Error("failed to register jobs", "error", err)
		return exitError
	}
	sched.Start()
	// Stop cancels running jobs and waits for them (RUN-001). It is called
	// explicitly below; the defer covers early returns. Stop is idempotent.
	defer sched.Stop()
	slog.Info("scheduler started, waiting for jobs...")

	var ui *webui.Server
	uiErrCh := make(chan error, 1)
	if webUIEnabled(*webUIFlag, cfg) {
		ui, err = webui.New(webui.Options{
			Address:    *webUIAddr,
			ConfigPath: cfgPath,
			Config:     cfg,
			Status:     &statusBoard{sched: sched, recorder: recorder, jobs: jobs},
		})
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
	sched.Stop()
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

// jobInfo describes a registered job for the status page (WEB-011).
type jobInfo struct {
	name        string
	kind        string // "organizer" or "dedup"
	label       string
	schedule    string
	counterKeys []string
}

var (
	organizerCounterKeys = []string{"processed", "skipped", "errors"}
	dedupCounterKeys     = []string{"files", "duplicates", "actions", "errors"}
)

// registerJobs registers one job per organizer source and one per enabled
// duplicate detection job (ORG-010, DUP-008). Each run's result is recorded
// for the status page (WEB-011).
func registerJobs(sched *scheduler.Scheduler, cfg *config.Config, recorder *runstatus.Recorder) ([]jobInfo, error) {
	var jobs []jobInfo
	for i, profile := range cfg.Organizer {
		for j, source := range profile.Sources {
			name := fmt.Sprintf("organizer-%d-source-%d", i, j)
			statePath := organizer.StatePath(cfg.StateDir, profile.Target, source.Path)
			if _, err := sched.Register(name, source.Schedule, organizerJob(name, profile, source, statePath, recorder)); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			jobs = append(jobs, jobInfo{
				name: name, kind: "organizer", schedule: source.Schedule,
				label:       fmt.Sprintf("%s → %s", source.Path, profile.Target),
				counterKeys: organizerCounterKeys,
			})
		}
	}

	for i, dd := range cfg.DuplicateDetection {
		if !dd.Enabled {
			continue
		}
		name := fmt.Sprintf("dedup-%d", i)
		if _, err := sched.Register(name, dd.Schedule, dedupJob(name, dd, recorder)); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		jobs = append(jobs, jobInfo{
			name: name, kind: "dedup", schedule: dd.Schedule,
			label:       strings.Join(dd.Sources, ", "),
			counterKeys: dedupCounterKeys,
		})
	}
	return jobs, nil
}

// organizerJob runs one organizer source, notifies, and records the result.
// A run stopped by shutdown is neither recorded nor notified (RUN-001).
func organizerJob(name string, p config.OrganizerProfile, s config.OrganizerSource, statePath string, recorder *runstatus.Recorder) scheduler.Job {
	return func(ctx context.Context) {
		start := time.Now()
		ctx = withProgress(ctx, recorder, name, start)
		defer recorder.ClearProgress(name)
		st := state.Load(statePath)
		result, duration := organizer.RunWithTime(ctx, p, s, st, statePath)
		if result.Canceled {
			return
		}
		recorder.Finish(name, runstatus.Record{
			Start: start, Duration: duration, DryRun: result.DryRun,
			Counters: map[string]int{"processed": result.Processed, "skipped": result.Skipped, "errors": result.Errors},
		})
		if p.Ntfy != nil {
			title, msg := notify.FormatOrganizerSummary(s.Path, result.Processed, result.Skipped, result.Errors, duration)
			notify.New(p.Ntfy.URL).SendSummary(title, msg)
		}
	}
}

// dedupJob runs one duplicate detection job, notifies, and records the result.
// A run stopped by shutdown is neither recorded nor notified (RUN-001).
func dedupJob(name string, job config.DuplicateDetection, recorder *runstatus.Recorder) scheduler.Job {
	return func(ctx context.Context) {
		start := time.Now()
		ctx = withProgress(ctx, recorder, name, start)
		defer recorder.ClearProgress(name)
		result := dedup.Run(ctx, job)
		duration := time.Since(start)
		if result.Canceled {
			return
		}
		recorder.Finish(name, runstatus.Record{
			Start: start, Duration: duration, DryRun: result.DryRun,
			Counters: map[string]int{"files": result.TotalFiles, "duplicates": result.DuplicatesPair, "actions": result.ActionsApplied, "errors": result.Errors},
		})
		if job.Ntfy != nil {
			title, msg := notify.FormatDedupSummary(result.TotalFiles, result.DuplicatesPair, result.ActionsApplied, result.Errors, duration)
			notify.New(job.Ntfy.URL).SendSummary(title, msg)
		}
	}
}

// withProgress makes a job's progress visible on the status page (WEB-014).
func withProgress(ctx context.Context, recorder *runstatus.Recorder, name string, start time.Time) context.Context {
	return runstatus.WithReporter(ctx, func(phase string, done, total int) {
		recorder.SetProgress(name, runstatus.Progress{Phase: phase, Done: done, Total: total, Start: start})
	})
}

// statusBoard combines scheduler state and recorded results for the web UI
// status page (WEB-011). It implements webui.StatusSource.
type statusBoard struct {
	sched    *scheduler.Scheduler
	recorder *runstatus.Recorder
	jobs     []jobInfo
}

func (b *statusBoard) Jobs() []webui.JobStatus {
	out := make([]webui.JobStatus, 0, len(b.jobs))
	for _, j := range b.jobs {
		js := webui.JobStatus{
			Name: j.name, Kind: j.kind, Label: j.label, Schedule: j.schedule,
			Next: b.sched.Next(j.name), Running: b.sched.Running(j.name),
		}
		if p, ok := b.recorder.Progress(j.name); ok && js.Running {
			js.Phase, js.Done, js.Total, js.Since = p.Phase, p.Done, p.Total, p.Start
		}
		if rec, ok := b.recorder.Last(j.name); ok {
			js.HasLast, js.LastStart, js.LastDuration, js.DryRun = true, rec.Start, rec.Duration, rec.DryRun
			for _, key := range j.counterKeys {
				js.Counters = append(js.Counters, webui.Counter{Key: key, Value: rec.Counters[key]})
			}
		}
		out = append(out, js)
	}
	return out
}
