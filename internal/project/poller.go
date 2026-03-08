package project

import (
	"context"
	"log/slog"
	"time"
)

// Poller manages background polling goroutines for projects that have poll_interval configured.
type Poller struct {
	Manager *Manager
	cancel  context.CancelFunc
}

// NewPoller creates a new Poller backed by the given Manager.
func NewPoller(manager *Manager) *Poller {
	return &Poller{Manager: manager}
}

// Start launches a background goroutine for each project that has poll_interval set.
// Call Stop() to shut down all polling goroutines.
func (p *Poller) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	for _, projectCfg := range p.Manager.Config.Projects {
		if projectCfg.PollInterval == "" {
			continue
		}

		interval, err := time.ParseDuration(projectCfg.PollInterval)
		if err != nil {
			slog.Error("invalid poll_interval for project, skipping",
				"project", projectCfg.Name,
				"poll_interval", projectCfg.PollInterval,
				"error", err,
			)
			continue
		}

		slog.Info("starting poller for project", "project", projectCfg.Name, "interval", interval)
		go p.pollProject(ctx, projectCfg.Name, interval)
	}
}

// Stop cancels all polling goroutines. It is safe to call multiple times.
func (p *Poller) Stop() {
	if p.cancel != nil {
		slog.Info("stopping all project pollers")
		p.cancel()
	}
}

// pollProject is the goroutine body for a single project poller.
func (p *Poller) pollProject(ctx context.Context, name string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("poller started", "project", name, "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopped", "project", name)
			return
		case <-ticker.C:
			slog.Info("polling project", "project", name)
			result := p.Manager.SyncProject(name)
			if result.Error != nil {
				slog.Error("poll sync failed",
					"project", name,
					"error", result.Error,
				)
			} else {
				slog.Info("poll sync succeeded",
					"project", name,
					"commit", result.Commit,
				)
			}
		}
	}
}
