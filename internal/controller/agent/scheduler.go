package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// Scheduler polls the DB every minute and fires runs for schedule-triggered agents.
type Scheduler struct {
	store *db.Store
}

// NewScheduler creates a new Scheduler backed by store.
func NewScheduler(store *db.Store) *Scheduler {
	return &Scheduler{store: store}
}

// Start runs the scheduler loop until ctx is cancelled.
// It drains any in-flight runs gracefully, waiting up to 10 seconds.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			// Wait up to 10s for in-flight runs to complete.
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				slog.Warn("agent scheduler: timed out waiting for in-flight runs")
			}
			return

		case now := <-ticker.C:
			s.tick(ctx, now, &wg)
		}
	}
}

// tick fires scheduled agents whose cron window has elapsed since their last run.
func (s *Scheduler) tick(ctx context.Context, now time.Time, wg *sync.WaitGroup) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		slog.Error("agent scheduler: failed to list agents", "err", err)
		return
	}

	for _, a := range agents {
		if !a.Enabled || a.Trigger != "schedule" || a.Schedule == "" {
			continue
		}

		sched, err := parser.Parse(a.Schedule)
		if err != nil {
			slog.Warn("agent scheduler: invalid cron expression", "agent", a.Slug, "schedule", a.Schedule, "err", err)
			continue
		}

		// Find the most recent scheduled time at or before now.
		// sched.Next returns the first time strictly after the argument, so we
		// start from (now - 2 min) and advance until the next candidate would
		// overshoot now.  This correctly handles sub-hourly schedules like
		// "* * * * *" where a naïve single Next call returns the previous
		// window instead of the current one.
		prev := sched.Next(now.Add(-2 * time.Minute))
		if prev.After(now) {
			// No scheduled time in the past window.
			continue
		}
		for {
			candidate := sched.Next(prev)
			if candidate.After(now) {
				break
			}
			prev = candidate
		}

		// Check if we already have a run that started within this window.
		lastRun, err := s.store.GetLatestAgentRun(ctx, a.ID)
		if err == nil && lastRun != nil {
			// If the last run started at or after the most recent scheduled time, skip.
			if !lastRun.StartedAt.Before(prev) {
				continue
			}
		}

		// Fire the agent in a goroutine.
		runID := uuid.New().String()
		run := db.AgentRun{
			ID:         runID,
			AgentID:    a.ID,
			TriggerSrc: "schedule",
			Status:     "running",
			StartedAt:  time.Now().UTC(),
		}
		if err := s.store.CreateAgentRun(ctx, run); err != nil {
			slog.Error("agent scheduler: failed to create run", "agent", a.Slug, "err", err)
			continue
		}

		agent := a // capture loop var
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, runErr := Run(ctx, agent)
			status := "done"
			if runErr != nil {
				status = "error"
				output = runErr.Error()
				slog.Error("agent scheduler: run failed", "agent", agent.Slug, "runId", runID, "err", runErr)
			}
			finishedAt := time.Now().UTC()
			if updateErr := s.store.UpdateAgentRun(ctx, runID, status, output, finishedAt); updateErr != nil {
				slog.Error("agent scheduler: failed to update run", "runId", runID, "err", updateErr)
			}
		}()
	}
}
