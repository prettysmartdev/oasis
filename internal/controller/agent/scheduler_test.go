package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("db.New error: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	})
	return s
}

func createScheduleAgent(t *testing.T, s *db.Store, slug, schedule string, enabled bool) db.Agent {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	a := db.Agent{
		ID:        "agent-" + slug,
		Name:      "Agent " + slug,
		Slug:      slug,
		Prompt:    "test prompt",
		Trigger:   "schedule",
		Schedule:  schedule,
		OutputFmt: "markdown",
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateAgent(context.Background(), a); err != nil {
		t.Fatalf("CreateAgent %q: %v", slug, err)
	}
	return a
}

// TestSchedulerSkipsDisabledAgents verifies that tick does not fire runs for disabled agents.
func TestSchedulerSkipsDisabledAgents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a disabled agent whose schedule would otherwise match.
	a := createScheduleAgent(t, store, "disabled-agent", "* * * * *", false)

	sched := NewScheduler(store)
	var wg sync.WaitGroup
	// Tick with a time that satisfies the "* * * * *" schedule.
	sched.tick(ctx, time.Now(), &wg)
	wg.Wait()

	// No run should have been created for the disabled agent.
	_, err := store.GetLatestAgentRun(ctx, a.ID)
	if err == nil {
		t.Error("expected no run for disabled agent, but a run was created")
	}
}

// TestSchedulerDoesNotRefireInSameWindow verifies that if a run already started within
// the current cron window, the scheduler does not fire again.
func TestSchedulerDoesNotRefireInSameWindow(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Use a schedule that fires every minute: "* * * * *".
	a := createScheduleAgent(t, store, "no-refire-agent", "* * * * *", true)

	// Simulate a run that started within the current cron window.
	// The current window for "* * * * *" starts at the beginning of the current minute.
	now := time.Now().UTC()
	windowStart := now.Truncate(time.Minute)

	existingRun := db.AgentRun{
		ID:         "existing-run",
		AgentID:    a.ID,
		TriggerSrc: "schedule",
		Status:     "done",
		Output:     "previous output",
		// started_at is within the current window (after the last tick).
		StartedAt: windowStart.Add(10 * time.Second),
	}
	if err := store.CreateAgentRun(ctx, existingRun); err != nil {
		t.Fatalf("CreateAgentRun: %v", err)
	}

	sched := NewScheduler(store)
	var wg sync.WaitGroup
	sched.tick(ctx, now, &wg)
	wg.Wait()

	// Only the original run should exist; no new run should have been created.
	latest, err := store.GetLatestAgentRun(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetLatestAgentRun: %v", err)
	}
	if latest.ID != existingRun.ID {
		t.Errorf("scheduler re-fired in same window: got run %q, want %q", latest.ID, existingRun.ID)
	}
}

// TestSchedulerFiresScheduledAgent verifies that a schedule agent with no prior run
// is executed by tick.
func TestSchedulerFiresScheduledAgent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Use "* * * * *" (every minute) so the schedule always matches.
	a := createScheduleAgent(t, store, "fire-agent", "* * * * *", true)

	sched := NewScheduler(store)
	var wg sync.WaitGroup

	// Tick at a time that is 30s into a minute window.
	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	sched.tick(ctx, now, &wg)
	wg.Wait()

	// A run should have been created.
	run, err := store.GetLatestAgentRun(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetLatestAgentRun after tick: %v", err)
	}
	if run.TriggerSrc != "schedule" {
		t.Errorf("run.TriggerSrc = %q, want %q", run.TriggerSrc, "schedule")
	}
	// Status should be "done" since Run() completes synchronously via wg.Wait().
	if run.Status != "done" {
		t.Errorf("run.Status = %q, want %q", run.Status, "done")
	}
}

// TestSchedulerSkipsInvalidCron verifies no panic and no run when cron expression is invalid.
func TestSchedulerSkipsInvalidCron(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create an agent with an invalid cron expression.
	now := time.Now().UTC().Truncate(time.Second)
	a := db.Agent{
		ID:        "agent-bad-cron",
		Name:      "Bad Cron Agent",
		Slug:      "bad-cron",
		Prompt:    "test",
		Trigger:   "schedule",
		Schedule:  "not-a-valid-cron",
		OutputFmt: "markdown",
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	sched := NewScheduler(store)
	var wg sync.WaitGroup
	// Should not panic even with an invalid cron expression.
	sched.tick(ctx, time.Now(), &wg)
	wg.Wait()

	// No run should be created.
	_, err := store.GetLatestAgentRun(ctx, a.ID)
	if err == nil {
		t.Error("expected no run for bad cron agent, but one was created")
	}
}
