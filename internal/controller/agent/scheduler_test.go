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

func newTestScheduler(t *testing.T, store *db.Store) *Scheduler {
	t.Helper()
	return NewScheduler(store, StubHarness{}, t.TempDir())
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

	a := createScheduleAgent(t, store, "disabled-agent", "* * * * *", false)

	sched := newTestScheduler(t, store)
	var wg sync.WaitGroup
	sched.tick(ctx, time.Now(), &wg)
	wg.Wait()

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

	a := createScheduleAgent(t, store, "no-refire-agent", "* * * * *", true)

	now := time.Now().UTC()
	windowStart := now.Truncate(time.Minute)

	existingRun := db.AgentRun{
		ID:         "existing-run",
		AgentID:    a.ID,
		TriggerSrc: "schedule",
		Status:     "done",
		Output:     "previous output",
		StartedAt:  windowStart.Add(10 * time.Second),
	}
	if err := store.CreateAgentRun(ctx, existingRun); err != nil {
		t.Fatalf("CreateAgentRun: %v", err)
	}

	sched := newTestScheduler(t, store)
	var wg sync.WaitGroup
	sched.tick(ctx, now, &wg)
	wg.Wait()

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

	a := createScheduleAgent(t, store, "fire-agent", "* * * * *", true)

	sched := newTestScheduler(t, store)
	var wg sync.WaitGroup

	now := time.Now().UTC().Truncate(time.Minute).Add(30 * time.Second)
	sched.tick(ctx, now, &wg)
	wg.Wait()

	run, err := store.GetLatestAgentRun(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetLatestAgentRun after tick: %v", err)
	}
	if run.TriggerSrc != "schedule" {
		t.Errorf("run.TriggerSrc = %q, want %q", run.TriggerSrc, "schedule")
	}
	// Status should be "done" since StubHarness completes synchronously via wg.Wait().
	if run.Status != "done" {
		t.Errorf("run.Status = %q, want %q", run.Status, "done")
	}
}

// TestSchedulerSkipsInvalidCron verifies no panic and no run when cron expression is invalid.
func TestSchedulerSkipsInvalidCron(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

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

	sched := newTestScheduler(t, store)
	var wg sync.WaitGroup
	sched.tick(ctx, time.Now(), &wg)
	wg.Wait()

	_, err := store.GetLatestAgentRun(ctx, a.ID)
	if err == nil {
		t.Error("expected no run for bad cron agent, but one was created")
	}
}
