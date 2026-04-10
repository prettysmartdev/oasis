package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestAgent(slug string) Agent {
	now := time.Now().UTC().Truncate(time.Second)
	return Agent{
		ID:          "agent-id-" + slug,
		Name:        "Test Agent " + slug,
		Slug:        slug,
		Description: "A test agent",
		Icon:        "🤖",
		Prompt:      "Test prompt for " + slug,
		Trigger:     "tap",
		Schedule:    "",
		OutputFmt:   "markdown",
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestAgentCreateGetRoundTrip verifies that CreateAgent followed by GetAgent preserves all fields.
func TestAgentCreateGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := newTestAgent("roundtrip")
	agent.Description = "A detailed description"
	agent.Icon = "🚀"
	agent.Prompt = "Summarise the news today."
	agent.Trigger = "schedule"
	agent.Schedule = "0 8 * * *"
	agent.OutputFmt = "html"
	agent.Enabled = false

	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	got, err := s.GetAgent(ctx, agent.Slug)
	if err != nil {
		t.Fatalf("GetAgent error: %v", err)
	}

	if got.ID != agent.ID {
		t.Errorf("ID: got %q, want %q", got.ID, agent.ID)
	}
	if got.Name != agent.Name {
		t.Errorf("Name: got %q, want %q", got.Name, agent.Name)
	}
	if got.Slug != agent.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, agent.Slug)
	}
	if got.Description != agent.Description {
		t.Errorf("Description: got %q, want %q", got.Description, agent.Description)
	}
	if got.Icon != agent.Icon {
		t.Errorf("Icon: got %q, want %q", got.Icon, agent.Icon)
	}
	if got.Prompt != agent.Prompt {
		t.Errorf("Prompt: got %q, want %q", got.Prompt, agent.Prompt)
	}
	if got.Trigger != agent.Trigger {
		t.Errorf("Trigger: got %q, want %q", got.Trigger, agent.Trigger)
	}
	if got.Schedule != agent.Schedule {
		t.Errorf("Schedule: got %q, want %q", got.Schedule, agent.Schedule)
	}
	if got.OutputFmt != agent.OutputFmt {
		t.Errorf("OutputFmt: got %q, want %q", got.OutputFmt, agent.OutputFmt)
	}
	if got.Enabled != agent.Enabled {
		t.Errorf("Enabled: got %v, want %v", got.Enabled, agent.Enabled)
	}
}

// TestListAgentsOrder verifies that ListAgents returns agents in creation order (oldest first).
func TestListAgentsOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	slugs := []string{"agent-alpha", "agent-beta", "agent-gamma"}
	for i, slug := range slugs {
		a := newTestAgent(slug)
		// Stagger creation times so ordering is deterministic.
		a.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Second).Truncate(time.Second)
		a.UpdatedAt = a.CreatedAt
		if err := s.CreateAgent(ctx, a); err != nil {
			t.Fatalf("CreateAgent %q: %v", slug, err)
		}
	}

	agents, err := s.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents error: %v", err)
	}
	if len(agents) != len(slugs) {
		t.Fatalf("expected %d agents, got %d", len(slugs), len(agents))
	}
	for i, want := range slugs {
		if agents[i].Slug != want {
			t.Errorf("agents[%d].Slug = %q, want %q", i, agents[i].Slug, want)
		}
	}
}

// TestAgentDeleteCascadesToRuns verifies that deleting an agent also removes its runs.
func TestAgentDeleteCascadesToRuns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := newTestAgent("cascade-agent")
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	run := AgentRun{
		ID:         "run-cascade-1",
		AgentID:    agent.ID,
		TriggerSrc: "tap",
		Status:     "done",
		Output:     "output text",
		StartedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := s.CreateAgentRun(ctx, run); err != nil {
		t.Fatalf("CreateAgentRun error: %v", err)
	}

	// Verify the run exists before deletion.
	if _, err := s.GetAgentRun(ctx, run.ID); err != nil {
		t.Fatalf("GetAgentRun before delete: %v", err)
	}

	// Delete the agent.
	if err := s.DeleteAgent(ctx, agent.Slug); err != nil {
		t.Fatalf("DeleteAgent error: %v", err)
	}

	// The run should now be gone (cascade).
	_, err := s.GetAgentRun(ctx, run.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for cascaded run, got: %v", err)
	}
}

// TestGetLatestAgentRunOrdering verifies that GetLatestAgentRun returns the most recent run.
func TestGetLatestAgentRunOrdering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := newTestAgent("latest-run-agent")
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	base := time.Now().UTC().Truncate(time.Second)

	// Insert an older run first.
	older := AgentRun{
		ID:         "run-older",
		AgentID:    agent.ID,
		TriggerSrc: "tap",
		Status:     "done",
		Output:     "older output",
		StartedAt:  base,
	}
	if err := s.CreateAgentRun(ctx, older); err != nil {
		t.Fatalf("CreateAgentRun older: %v", err)
	}

	// Insert a newer run.
	newer := AgentRun{
		ID:         "run-newer",
		AgentID:    agent.ID,
		TriggerSrc: "tap",
		Status:     "done",
		Output:     "newer output",
		StartedAt:  base.Add(time.Minute),
	}
	if err := s.CreateAgentRun(ctx, newer); err != nil {
		t.Fatalf("CreateAgentRun newer: %v", err)
	}

	got, err := s.GetLatestAgentRun(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetLatestAgentRun error: %v", err)
	}
	if got.ID != newer.ID {
		t.Errorf("GetLatestAgentRun: got run %q, want %q", got.ID, newer.ID)
	}
}

// TestAgentSlugUniqueness verifies that inserting two agents with the same slug returns an error.
func TestAgentSlugUniqueness(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first := newTestAgent("dup-agent")
	if err := s.CreateAgent(ctx, first); err != nil {
		t.Fatalf("CreateAgent first: %v", err)
	}

	second := newTestAgent("dup-agent")
	second.ID = "different-agent-id"
	err := s.CreateAgent(ctx, second)
	if err == nil {
		t.Fatal("expected error for duplicate agent slug, got nil")
	}
	if !strings.Contains(err.Error(), "UNIQUE") {
		t.Errorf("expected UNIQUE constraint error, got: %v", err)
	}
}

// TestGetLatestAgentRunNotFound verifies ErrNotFound when an agent has no runs.
func TestGetLatestAgentRunNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := newTestAgent("no-runs-agent")
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	_, err := s.GetLatestAgentRun(ctx, agent.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for agent with no runs, got: %v", err)
	}
}

// TestCreateAgentWithModelPersists verifies that the Model field is stored and retrieved correctly.
func TestCreateAgentWithModelPersists(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAgent("model-agent")
	a.Model = "claude-opus-4-6"
	if err := s.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	got, err := s.GetAgent(ctx, a.Slug)
	if err != nil {
		t.Fatalf("GetAgent error: %v", err)
	}
	if got.Model != "claude-opus-4-6" {
		t.Errorf("Model: got %q, want %q", got.Model, "claude-opus-4-6")
	}
}

// TestCreateAgentWithEmptyModelStoresEmpty verifies that an empty Model field is stored
// and retrieved as an empty string.
func TestCreateAgentWithEmptyModelStoresEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAgent("empty-model-agent")
	a.Model = ""
	if err := s.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	got, err := s.GetAgent(ctx, a.Slug)
	if err != nil {
		t.Fatalf("GetAgent error: %v", err)
	}
	if got.Model != "" {
		t.Errorf("Model: got %q, want empty string", got.Model)
	}
}

// TestUpdateAgentModelField verifies that the model field can be updated via UpdateAgent.
func TestUpdateAgentModelField(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := newTestAgent("update-model-agent")
	a.Model = ""
	if err := s.CreateAgent(ctx, a); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	if err := s.UpdateAgent(ctx, a.Slug, map[string]any{"model": "claude-haiku-4-5-20251001"}); err != nil {
		t.Fatalf("UpdateAgent error: %v", err)
	}

	got, err := s.GetAgent(ctx, a.Slug)
	if err != nil {
		t.Fatalf("GetAgent error: %v", err)
	}
	if got.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model after update: got %q, want %q", got.Model, "claude-haiku-4-5-20251001")
	}
}

// TestGetRunningAgentRun verifies GetRunningAgentRun returns a run with status="running".
func TestGetRunningAgentRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	agent := newTestAgent("running-agent")
	if err := s.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent error: %v", err)
	}

	run := AgentRun{
		ID:         "run-running",
		AgentID:    agent.ID,
		TriggerSrc: "tap",
		Status:     "running",
		StartedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := s.CreateAgentRun(ctx, run); err != nil {
		t.Fatalf("CreateAgentRun error: %v", err)
	}

	got, err := s.GetRunningAgentRun(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetRunningAgentRun error: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("got run %q, want %q", got.ID, run.ID)
	}
	if got.Status != "running" {
		t.Errorf("got status %q, want %q", got.Status, "running")
	}

	// No running run → ErrNotFound.
	_, err = s.GetRunningAgentRun(ctx, "no-such-agent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
