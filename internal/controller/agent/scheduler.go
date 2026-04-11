package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// Scheduler polls the DB every minute and fires runs for schedule-triggered agents.
type Scheduler struct {
	store   *db.Store
	harness AgentHarness
	runsDir string
}

// NewScheduler creates a new Scheduler backed by store.
// harness is the AgentHarness to use for executing agents; if nil, StubHarness is used.
// runsDir is the base directory for agent run work directories.
func NewScheduler(store *db.Store, harness AgentHarness, runsDir string) *Scheduler {
	if harness == nil {
		harness = StubHarness{}
	}
	return &Scheduler{store: store, harness: harness, runsDir: runsDir}
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
		prev := sched.Next(now.Add(-2 * time.Minute))
		if prev.After(now) {
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
			if !lastRun.StartedAt.Before(prev) {
				continue
			}
		}

		// Fire the agent in a goroutine.
		runID := uuid.New().String()

		// Create work directory before the DB record.
		workDir := filepath.Join(s.runsDir, runID)
		if mkErr := os.MkdirAll(workDir, 0o750); mkErr != nil {
			slog.Error("agent scheduler: failed to create work dir", "runId", runID, "err", mkErr)
			continue
		}

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
		harness := s.harness
		store := s.store
		slog.Info("agent run triggered", "agent", agent.Slug, "runId", runID, "trigger", "schedule")
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, status := ExecuteRun(ctx, harness, agent, workDir)
			finishedAt := time.Now().UTC()
			LogRunCompletion(agent.Slug, runID, workDir, status)
			if updateErr := store.UpdateAgentRun(ctx, runID, status, output, finishedAt); updateErr != nil {
				slog.Error("agent scheduler: failed to update run", "runId", runID, "err", updateErr)
			}
		}()
	}
}

// LogRunCompletion logs the final status and work directory contents for a completed run.
func LogRunCompletion(agentSlug, runID, workDir, status string) {
	entries, err := os.ReadDir(workDir)
	files := make([]string, 0, len(entries))
	if err == nil {
		for _, e := range entries {
			files = append(files, e.Name())
		}
	}
	slog.Info("agent run completed", "agent", agentSlug, "runId", runID, "status", status, "workDirFiles", files)
}

// ExecuteRun calls harness.Execute and reads the output file or agentoutput.txt.
// Returns (output, status).
func ExecuteRun(ctx context.Context, harness AgentHarness, a db.Agent, workDir string) (string, string) {
	err := harness.Execute(ctx, a, workDir)
	if err != nil {
		// Detect context cancellation (run timeout).
		if ctx.Err() != nil {
			return "agent run timed out", "error"
		}
		// Try to read agentoutput.txt written by ClaudeHarness.
		agentOut := filepath.Join(workDir, "agentoutput.txt")
		if data, readErr := os.ReadFile(agentOut); readErr == nil && len(data) > 0 {
			return string(data), "error"
		}
		return fmt.Sprintf("agent execution failed: %v", err), "error"
	}

	// Read the output file.
	outFile := filepath.Join(workDir, OutputFilename(a.OutputFmt))
	data, readErr := os.ReadFile(outFile)
	if readErr != nil || len(data) == 0 {
		return "agent produced no output file", "error"
	}
	return string(data), "done"
}
