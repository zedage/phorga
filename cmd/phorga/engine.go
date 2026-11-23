package main

import (
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/dedup"
	"github.com/zedage/phorga/internal/runstatus"
	"github.com/zedage/phorga/internal/scheduler"
)

// engine owns the configuration in effect and the jobs scheduled from it.
// The web UI hands it a new configuration, which it saves to the config
// file and puts into effect without a restart (CFG-008). A job that is
// running at that moment finishes with the settings it started with.
type engine struct {
	mu       sync.Mutex
	path     string
	cfg      *config.Config
	sched    *scheduler.Scheduler
	recorder *runstatus.Recorder
	cache    *dedup.Cache
	jobs     []jobInfo

	// Fixed at startup; changing it needs a restart.
	stateDir string
}

// newEngine schedules the jobs of cfg. stateDir, recorder and cache belong
// to the state directory loaded at startup.
func newEngine(path string, cfg *config.Config, sched *scheduler.Scheduler, recorder *runstatus.Recorder) (*engine, error) {
	e := &engine{
		path:     path,
		cfg:      cfg,
		sched:    sched,
		recorder: recorder,
		// One fingerprint cache for all duplicate detection jobs (DUP-012).
		cache:    dedup.LoadCache(dedup.CachePath(cfg.StateDir)),
		stateDir: cfg.StateDir,
	}
	jobs, err := registerJobs(sched, e.jobConfig(cfg), recorder, e.cache)
	e.jobs = jobs
	return e, err
}

// jobConfig returns cfg with the state directory loaded at startup, so the
// jobs keep using it until a restart.
func (e *engine) jobConfig(cfg *config.Config) *config.Config {
	c := *cfg
	c.StateDir = e.stateDir
	return &c
}

// Jobs returns the jobs scheduled right now.
func (e *engine) Jobs() []jobInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.jobs)
}

// Apply validates cfg, saves it to the config file and schedules its jobs
// in place of the current ones (CFG-008). Nothing changes when validation
// or saving fails. It returns the configuration as read back from the file
// (with defaults) and the fields whose change needs a restart.
func (e *engine) Apply(cfg *config.Config) (*config.Config, []string, error) {
	if err := config.Validate(cfg); err != nil {
		return nil, nil, err
	}
	data, err := config.Marshal(e.path, cfg)
	if err != nil {
		return nil, nil, err
	}
	return e.ApplyRaw(data)
}

// ApplyRaw is Apply for the text of a config file, as edited in the web UI
// (WEB-018). The text is written as it is, so its comments stay.
func (e *engine) ApplyRaw(data []byte) (*config.Config, []string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	parsed, err := config.Parse(e.path, data)
	if err != nil {
		return nil, nil, err
	}
	if err := config.Validate(parsed); err != nil {
		return nil, nil, err
	}
	if err := config.WriteFile(e.path, data); err != nil {
		return nil, nil, err
	}
	applied, err := config.Load(e.path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading back the saved config: %w", err)
	}

	restart := e.restartNeeded(applied)
	old := make(map[string]jobInfo, len(e.jobs))
	for _, j := range e.jobs {
		old[j.name] = j
		e.sched.Unregister(j.name)
	}
	jobs, err := registerJobs(e.sched, e.jobConfig(applied), e.recorder, e.cache)
	kept := map[string]bool{}
	for _, j := range jobs {
		// Job names are positions ("organizer-0-source-1"). When a name now
		// stands for another folder, its last result belongs to the old one.
		if o, ok := old[j.name]; ok && o.kind == j.kind && o.label == j.label {
			kept[j.name] = true
		}
	}
	for name := range old {
		if !kept[name] {
			e.recorder.Forget(name)
		}
	}
	e.cfg, e.jobs = applied, jobs
	slog.Info("configuration applied", "path", e.path, "jobs", len(jobs), "restartNeeded", restart)
	if err != nil {
		return applied, restart, fmt.Errorf("scheduling jobs: %w", err)
	}
	return applied, restart, nil
}

// restartNeeded lists the settings of cfg that differ from the ones in
// effect and only take effect after a restart.
func (e *engine) restartNeeded(cfg *config.Config) []string {
	var fields []string
	if cfg.StateDir != e.stateDir {
		fields = append(fields, "stateDir")
	}
	oldUI, newUI := webUIOf(e.cfg), webUIOf(cfg)
	// The UI that serves this request keeps running; turning it off takes
	// a restart. Turning it on needs nothing, it is already running.
	if oldUI.Enabled && !newUI.Enabled {
		fields = append(fields, "webUI.enabled")
	}
	if newUI.Address != oldUI.Address {
		fields = append(fields, "webUI.address")
	}
	return fields
}

func webUIOf(cfg *config.Config) config.WebUIConfig {
	if cfg.WebUI == nil {
		return config.WebUIConfig{Address: config.DefaultWebUIAddress}
	}
	return *cfg.WebUI
}
