package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Agent represents a registered AI agent in the oasis registry.
type Agent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Prompt      string    `json:"prompt"`
	Trigger     string    `json:"trigger"`   // "tap" | "schedule" | "webhook"
	Schedule    string    `json:"schedule"`  // cron expression; only when trigger="schedule"
	OutputFmt   string    `json:"outputFmt"` // "markdown" | "html" | "plaintext"
	Model       string    `json:"model"`     // optional claude model override (e.g. "claude-opus-4-5")
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AgentRun represents a single execution run of an agent.
type AgentRun struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agentId"`
	TriggerSrc string     `json:"triggerSrc"`
	Status     string     `json:"status"`
	Output     string     `json:"output"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// CreateAgent inserts a new agent record.
func (s *Store) CreateAgent(ctx context.Context, agent Agent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO agents (id, name, slug, description, icon, prompt, trigger, schedule, output_fmt, model, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.Name, agent.Slug, agent.Description, agent.Icon,
		agent.Prompt, agent.Trigger, agent.Schedule, agent.OutputFmt, agent.Model,
		boolToInt(agent.Enabled),
		agent.CreatedAt.UTC().Format(time.RFC3339),
		agent.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetAgent retrieves an agent by slug. Returns ErrNotFound if it does not exist.
func (s *Store) GetAgent(ctx context.Context, slug string) (*Agent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, description, icon, prompt, trigger, schedule, output_fmt, model, enabled, created_at, updated_at
		 FROM agents WHERE slug = ?`, slug)
	a, err := scanAgent(row)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListAgents returns all agents ordered by creation time (oldest first).
func (s *Store) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, description, icon, prompt, trigger, schedule, output_fmt, model, enabled, created_at, updated_at
		 FROM agents ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// UpdateAgent applies a partial update to the agent identified by slug.
// Supported fields: name, description, icon, prompt, trigger, schedule, outputFmt, model, enabled.
// Returns ErrNotFound if the slug does not exist.
func (s *Store) UpdateAgent(ctx context.Context, slug string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}

	// Map of allowed JSON keys to their SQL column names.
	allowed := map[string]string{
		"name":        "name",
		"description": "description",
		"icon":        "icon",
		"prompt":      "prompt",
		"trigger":     "trigger",
		"schedule":    "schedule",
		"outputFmt":   "output_fmt",
		"model":       "model",
		"enabled":     "enabled",
	}

	setClauses := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)

	for k, v := range fields {
		col, ok := allowed[k]
		if !ok {
			return fmt.Errorf("unsupported update field: %s", k)
		}
		if col == "enabled" {
			if b, ok := v.(bool); ok {
				v = boolToInt(b)
			}
		}
		setClauses = append(setClauses, col+"=?")
		args = append(args, v)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "updated_at=?")
	args = append(args, now)
	args = append(args, slug)

	query := "UPDATE agents SET " + strings.Join(setClauses, ", ") + " WHERE slug=?"
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAgent removes the agent with the given slug.
// Returns ErrNotFound if the slug does not exist.
func (s *Store) DeleteAgent(ctx context.Context, slug string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE slug = ?`, slug)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateAgentRun inserts a new agent run record.
func (s *Store) CreateAgentRun(ctx context.Context, run AgentRun) error {
	var finishedAt *string
	if run.FinishedAt != nil {
		s := run.FinishedAt.UTC().Format(time.RFC3339)
		finishedAt = &s
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO agent_runs (id, agent_id, trigger_src, status, output, started_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.AgentID, run.TriggerSrc, run.Status, run.Output,
		run.StartedAt.UTC().Format(time.RFC3339),
		finishedAt,
	)
	return err
}

// GetAgentRun retrieves a specific agent run by ID. Returns ErrNotFound if it does not exist.
func (s *Store) GetAgentRun(ctx context.Context, runID string) (*AgentRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, trigger_src, status, output, started_at, finished_at
		 FROM agent_runs WHERE id = ?`, runID)
	r, err := scanAgentRun(row)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetLatestAgentRun returns the most recent run for the given agent ID, ordered by started_at.
// Returns ErrNotFound if no runs exist.
func (s *Store) GetLatestAgentRun(ctx context.Context, agentID string) (*AgentRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, trigger_src, status, output, started_at, finished_at
		 FROM agent_runs WHERE agent_id = ? ORDER BY started_at DESC LIMIT 1`, agentID)
	r, err := scanAgentRun(row)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetRunningAgentRun returns a run with status="running" for the given agent ID.
// Returns ErrNotFound if no running run exists.
func (s *Store) GetRunningAgentRun(ctx context.Context, agentID string) (*AgentRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, trigger_src, status, output, started_at, finished_at
		 FROM agent_runs WHERE agent_id = ? AND status = 'running' LIMIT 1`, agentID)
	r, err := scanAgentRun(row)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateAgentRun updates the status, output, and finished_at of a run.
func (s *Store) UpdateAgentRun(ctx context.Context, runID string, status, output string, finishedAt time.Time) error {
	finishedStr := finishedAt.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_runs SET status=?, output=?, finished_at=? WHERE id=?`,
		status, output, finishedStr, runID,
	)
	return err
}

// scanAgent scans a row into an Agent.
func scanAgent(s scanner) (Agent, error) {
	var (
		a        Agent
		enabledI int
		createdS string
		updatedS string
	)
	err := s.Scan(
		&a.ID, &a.Name, &a.Slug, &a.Description, &a.Icon,
		&a.Prompt, &a.Trigger, &a.Schedule, &a.OutputFmt, &a.Model,
		&enabledI, &createdS, &updatedS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	a.Enabled = enabledI != 0
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdS)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedS)
	return a, nil
}

// scanAgentRun scans a row into an AgentRun.
func scanAgentRun(s scanner) (AgentRun, error) {
	var (
		r           AgentRun
		startedS    string
		finishedS   sql.NullString
	)
	err := s.Scan(
		&r.ID, &r.AgentID, &r.TriggerSrc, &r.Status, &r.Output,
		&startedS, &finishedS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentRun{}, ErrNotFound
	}
	if err != nil {
		return AgentRun{}, err
	}
	r.StartedAt, _ = time.Parse(time.RFC3339, startedS)
	if finishedS.Valid && finishedS.String != "" {
		t, _ := time.Parse(time.RFC3339, finishedS.String)
		r.FinishedAt = &t
	}
	return r, nil
}
