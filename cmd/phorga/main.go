package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
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
// written as JSON to logOut. The configuration is loaded at startup; changes
// saved in the web UI are applied while it runs (CFG-008).
func run(ctx context.Context, args []string, logOut io.Writer) int {
	flags := flag.NewFlagSet("phorga", flag.ContinueOnError)
	flags.SetOutput(logOut)
	configPath := flags.String("config", "", "Path to config file (YAML or JSON)")
	webUIFlag := flags.Bool("web-ui", false, "Enable the built-in configuration web UI (overrides config)")
	webUIAddr := flags.String("web-ui-addr", "", "Listen address for the web UI (overrides config, e.g. 127.0.0.1:8080)")
	showVersion := flags.Bool("version", false, "Print the version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if *showVersion {
		fmt.Fprintln(logOut, "phorga", currentVersion())
		return exitOK
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("phorga starting", "version", currentVersion())

	cfgPath, cfg, exists, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("startup failed", "error", err)
		return exitError
	}
	if exists {
		slog.Info("config loaded", "path", cfgPath, "stateDir", cfg.StateDir)
	} else {
		// Setup mode (CFG-009): start without jobs; the web UI creates the
		// file on the first save.
		addSetupBrowseRoots(cfg)
		slog.Warn("no config file yet, starting without jobs", "path", cfgPath)
	}

	if !cfg.HasJobs() && cfg.WebUI == nil {
		// Set up in the browser: keep the UI on after the first save, or it
		// would be gone at the next restart (CFG-009).
		cfg.WebUI = &config.WebUIConfig{Enabled: true, Address: config.DefaultWebUIAddress}
	}

	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		slog.Error("failed to create state directory", "path", cfg.StateDir, "error", err)
		return exitError
	}
	// Without access to its state, a copy job would copy every file again
	// on every run (RUN-003).
	if err := checkStateAccess(cfg.StateDir); err != nil {
		slog.Error("state directory not accessible; give the user Phorga runs as read and write access, e.g. with chown (README, Docker, Upgrading)",
			append([]any{"error", err}, processUser()...)...)
		return exitError
	}

	sched := scheduler.New()
	recorder := runstatus.Load(runstatus.Path(cfg.StateDir))
	eng, err := newEngine(cfgPath, cfg, sched, recorder)
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
	// Without jobs there is nothing to do but set Phorga up, so the web UI
	// always starts then (CFG-009, WEB-001).
	if webUIEnabled(*webUIFlag, cfg) || !cfg.HasJobs() {
		ui, err = webui.New(webui.Options{
			Address:    webUIAddress(*webUIAddr),
			ConfigPath: cfgPath,
			Config:     cfg,
			Status:     &statusBoard{sched: sched, recorder: recorder, jobs: eng.Jobs},
			Apply:      eng.Apply,
			ApplyRaw:   eng.ApplyRaw,
			Password:   webUIPassword(),
			Version:    currentVersion(),
		})
		if err != nil {
			slog.Error("web UI setup failed", "error", err)
			return exitError
		}
		if err := checkConfigWritable(cfgPath); err != nil {
			slog.Warn("saving in the web UI will fail: the config file or its folder is not writable; mount the folder writable, not the file alone (RUN-003)",
				append([]any{"path", cfgPath, "error", err}, processUser()...)...)
		}
		if openWithoutPassword(ui.Address(), webUIPassword()) {
			slog.Warn("the web UI has no password and listens on more than loopback; anyone who can reach its port can change the configuration. Set "+envWebUIPassword+", or publish the port on 127.0.0.1 only (WEB-020)", "address", ui.Address())
		}
		if !cfg.HasJobs() {
			slog.Info("no jobs configured; set Phorga up in the web UI", "url", "http://"+ui.Address()+"/ui")
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
// A missing file gives the setup configuration and exists=false (CFG-009).
func loadConfig(cliPath string) (string, *config.Config, bool, error) {
	path := config.ResolveConfigPath(cliPath)
	cfg, exists, err := config.LoadOrDefault(path)
	if err != nil {
		return "", nil, false, fmt.Errorf("config loading failed: %w", err)
	}
	if err := config.Validate(cfg); err != nil {
		return "", nil, false, err
	}
	return path, cfg, exists, nil
}

// webUIEnabled reports whether the web UI should start: via --web-ui or
// webUI.enabled in the config (WEB-001).
func webUIEnabled(flagSet bool, cfg *config.Config) bool {
	return flagSet || (cfg.WebUI != nil && cfg.WebUI.Enabled)
}

// envWebUIAddr sets the web UI address like --web-ui-addr. The Docker image
// sets it to 0.0.0.0:8080 (WEB-001).
const envWebUIAddr = "PHORGA_WEB_UI_ADDR"

// webUIAddress returns the listen address that overrides webUI.address:
// the --web-ui-addr flag, else PHORGA_WEB_UI_ADDR, else "" (WEB-001).
func webUIAddress(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envWebUIAddr)
}

// envWebUIPassword protects the web UI with a password (WEB-020). It is
// only read from the environment, so the config file and the UI never
// contain it.
const envWebUIPassword = "PHORGA_WEB_UI_PASSWORD"

func webUIPassword() string {
	return os.Getenv(envWebUIPassword)
}

// openWithoutPassword reports whether the UI listens on more than loopback
// without a password, which start-up warns about (WEB-020).
func openWithoutPassword(address, password string) bool {
	if password != "" {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// setupBrowseRoots are folders the Docker setup mounts by convention. In
// setup mode, the ones that exist become folder picker roots, so the picker
// works before anything is configured (CFG-009).
var setupBrowseRoots = []string{config.DefaultSynologyHomes, "/photos"}

func addSetupBrowseRoots(cfg *config.Config) {
	for _, root := range setupBrowseRoots {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			cfg.WebUI.BrowseRoots = append(cfg.WebUI.BrowseRoots, root)
		}
	}
}

// jobInfo describes a registered job for the status page (WEB-011).
type jobInfo struct {
	name        string
	kind        string // "organizer" or "dedup"
	label       string
	title       string // the profile's or job's name, may be empty (CFG-011)
	field       string // form field of the schedule, for the status page link (WEB-019)
	schedule    string
	counterKeys []string
}

var (
	organizerCounterKeys = []string{"processed", "skipped", "errors"}
	dedupCounterKeys     = []string{"files", "duplicates", "actions", "errors"}
)

// registerJobs registers one job per organizer source and one per enabled
// duplicate detection job (ORG-010, DUP-008). Each run's result is recorded
// for the status page (WEB-011). All duplicate detection jobs share cache
// (DUP-012).
func registerJobs(sched *scheduler.Scheduler, cfg *config.Config, recorder *runstatus.Recorder, cache *dedup.Cache) ([]jobInfo, error) {
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
				title:       profile.Name,
				field:       fmt.Sprintf("organizer.%d.sources.%d.schedule", i, j),
				counterKeys: organizerCounterKeys,
			})
		}
	}

	for i, dd := range cfg.DuplicateDetection {
		if !dd.Enabled {
			continue
		}
		name := fmt.Sprintf("dedup-%d", i)
		if _, err := sched.Register(name, dd.Schedule, dedupJob(name, dd, cache, recorder)); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		jobs = append(jobs, jobInfo{
			name: name, kind: "dedup", schedule: dd.Schedule,
			label:       strings.Join(dd.Sources, ", "),
			title:       dd.Name,
			field:       fmt.Sprintf("duplicateDetection.%d.schedule", i),
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
		st, err := state.Load(statePath)
		if err != nil {
			// Without its state the run would treat every file as new
			// (ORG-015), so it does not run at all.
			slog.Error("organizer run skipped: its state file cannot be read", "job", name, "path", statePath, "error", err)
			recorder.Finish(name, runstatus.Record{
				Start: start, Duration: time.Since(start), DryRun: p.DryRun,
				Counters: map[string]int{"processed": 0, "skipped": 0, "errors": 1},
			})
			if p.Ntfy != nil {
				title, msg := notify.FormatOrganizerSummary(s.Path, 0, 0, 1, time.Since(start))
				notify.New(p.Ntfy.URL).SendSummary(title, msg)
			}
			return
		}
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
func dedupJob(name string, job config.DuplicateDetection, cache *dedup.Cache, recorder *runstatus.Recorder) scheduler.Job {
	return func(ctx context.Context) {
		start := time.Now()
		ctx = withProgress(ctx, recorder, name, start)
		defer recorder.ClearProgress(name)
		result := dedup.RunWithCache(ctx, job, cache)
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
	jobs     func() []jobInfo // the jobs scheduled right now
}

func (b *statusBoard) Jobs() []webui.JobStatus {
	jobs := b.jobs()
	out := make([]webui.JobStatus, 0, len(jobs))
	for _, j := range jobs {
		js := webui.JobStatus{
			Name: j.name, Kind: j.kind, Label: j.label, Schedule: j.schedule,
			Title: j.title, Field: j.field,
			Next: b.sched.Next(j.name), Running: b.sched.Running(j.name),
			Waiting: b.sched.Waiting(j.name),
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
