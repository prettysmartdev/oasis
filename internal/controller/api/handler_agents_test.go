package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// --- Helpers -----------------------------------------------------------------

const validAgentBody = `{
	"name": "Test Agent",
	"slug": "test-agent",
	"prompt": "Do something.",
	"trigger": "tap",
	"outputFmt": "markdown",
	"enabled": true
}`

// --- Tests -------------------------------------------------------------------

// TestCreateAgentHappyPath verifies POST /api/v1/agents returns 201 with the created agent.
func TestCreateAgentHappyPath(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Errorf("status: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["slug"] != "test-agent" {
		t.Errorf("slug: got %v, want %q", resp["slug"], "test-agent")
	}
	if resp["trigger"] != "tap" {
		t.Errorf("trigger: got %v, want %q", resp["trigger"], "tap")
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Errorf("expected non-empty id in response")
	}
}

// TestCreateAgentDuplicateSlug verifies POST /api/v1/agents returns 409 with SLUG_CONFLICT.
func TestCreateAgentDuplicateSlug(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("first create: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	rec2 := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec2.Code != 409 {
		t.Errorf("duplicate create: got %d, want 409; body: %s", rec2.Code, rec2.Body.String())
	}
	var errResp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp["code"] != "SLUG_CONFLICT" {
		t.Errorf("code: got %v, want SLUG_CONFLICT", errResp["code"])
	}
}

// TestCreateAgentScheduleMissingField verifies trigger=schedule with missing schedule → 400 INVALID_SCHEDULE.
func TestCreateAgentScheduleMissingField(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"Sched Agent","slug":"sched-agent","prompt":"p","trigger":"schedule","outputFmt":"markdown"}`
	rec := doRequest(t, mux, "POST", "/api/v1/agents", body)
	if rec.Code != 400 {
		t.Errorf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	var errResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp["code"] != "INVALID_SCHEDULE" {
		t.Errorf("code: got %v, want INVALID_SCHEDULE", errResp["code"])
	}
}

// TestCreateAgentInvalidOutputFmt verifies invalid outputFmt → 400.
func TestCreateAgentInvalidOutputFmt(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"Bad Fmt","slug":"bad-fmt","prompt":"p","trigger":"tap","outputFmt":"pdf"}`
	rec := doRequest(t, mux, "POST", "/api/v1/agents", body)
	if rec.Code != 400 {
		t.Errorf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateAgentInvalidTrigger verifies invalid trigger → 400.
func TestCreateAgentInvalidTrigger(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"Bad Trigger","slug":"bad-trigger","prompt":"p","trigger":"push","outputFmt":"markdown"}`
	rec := doRequest(t, mux, "POST", "/api/v1/agents", body)
	if rec.Code != 400 {
		t.Errorf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// TestTriggerAgentRunOnRunningAgent verifies 409 RUN_IN_PROGRESS with runId
// when a run is already in progress.
func TestTriggerAgentRunOnRunningAgent(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	// Create an agent.
	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d; body: %s", rec.Code, rec.Body.String())
	}
	var agent map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &agent); err != nil {
		t.Fatalf("unmarshal agent: %v", err)
	}
	agentID := agent["id"].(string)

	// Directly insert a "running" run into the DB (avoids race with the goroutine).
	existingRunID := "pre-existing-running-run"
	run := db.AgentRun{
		ID:         existingRunID,
		AgentID:    agentID,
		TriggerSrc: "tap",
		Status:     "running",
		StartedAt:  time.Now().UTC(),
	}
	if err := h.store.CreateAgentRun(context.Background(), run); err != nil {
		t.Fatalf("CreateAgentRun: %v", err)
	}

	// Trigger run — should get 409 because a run is already in progress.
	rec2 := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/run", "")
	if rec2.Code != 409 {
		t.Errorf("status: got %d, want 409; body: %s", rec2.Code, rec2.Body.String())
	}
	var errResp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp["code"] != "RUN_IN_PROGRESS" {
		t.Errorf("code: got %v, want RUN_IN_PROGRESS", errResp["code"])
	}
	if errResp["runId"] != existingRunID {
		t.Errorf("runId: got %v, want %q", errResp["runId"], existingRunID)
	}
}

// TestDeleteAgentThenGet verifies DELETE returns 204 and subsequent GET returns 404.
func TestDeleteAgentThenGet(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	del := doRequest(t, mux, "DELETE", "/api/v1/agents/test-agent", "")
	if del.Code != 204 {
		t.Errorf("delete: got %d, want 204; body: %s", del.Code, del.Body.String())
	}

	get := doRequest(t, mux, "GET", "/api/v1/agents/test-agent", "")
	if get.Code != 404 {
		t.Errorf("get after delete: got %d, want 404; body: %s", get.Code, get.Body.String())
	}
}

// TestEnableDisableAgent verifies that enable/disable toggle the enabled field.
func TestEnableDisableAgent(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	// Disable.
	dis := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/disable", "")
	if dis.Code != 200 {
		t.Errorf("disable: got %d, want 200; body: %s", dis.Code, dis.Body.String())
	}
	var disResp map[string]any
	if err := json.Unmarshal(dis.Body.Bytes(), &disResp); err != nil {
		t.Fatalf("unmarshal disable response: %v", err)
	}
	if disResp["enabled"] != false {
		t.Errorf("after disable: enabled = %v, want false", disResp["enabled"])
	}

	// Enable.
	en := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/enable", "")
	if en.Code != 200 {
		t.Errorf("enable: got %d, want 200; body: %s", en.Code, en.Body.String())
	}
	var enResp map[string]any
	if err := json.Unmarshal(en.Body.Bytes(), &enResp); err != nil {
		t.Fatalf("unmarshal enable response: %v", err)
	}
	if enResp["enabled"] != true {
		t.Errorf("after enable: enabled = %v, want true", enResp["enabled"])
	}
}

// TestListAgentsEmpty verifies GET /api/v1/agents returns empty items on a fresh DB.
func TestListAgentsEmpty(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "GET", "/api/v1/agents", "")
	if rec.Code != 200 {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("'items' is not a slice: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// TestGetAgentNotFound verifies GET /api/v1/agents/{slug} returns 404 for missing slugs.
func TestGetAgentNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "GET", "/api/v1/agents/no-such-agent", "")
	if rec.Code != 404 {
		t.Errorf("status: got %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateAgentScheduleValid verifies a valid schedule agent is accepted.
func TestCreateAgentScheduleValid(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"Daily Digest","slug":"daily-digest","prompt":"Summarise news.","trigger":"schedule","schedule":"0 8 * * *","outputFmt":"markdown"}`
	rec := doRequest(t, mux, "POST", "/api/v1/agents", body)
	if rec.Code != 201 {
		t.Errorf("status: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

// TestAgentWebhookTrigger verifies POST /api/v1/agents/{slug}/webhook returns 202.
func TestAgentWebhookTrigger(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	wh := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/webhook", "")
	if wh.Code != 202 {
		t.Errorf("webhook trigger: got %d, want 202; body: %s", wh.Code, wh.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(wh.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["runId"] == nil || resp["runId"] == "" {
		t.Errorf("expected non-empty runId in response")
	}
}

// TestGetAgentRunNotFound verifies GET /api/v1/agents/runs/{runId} returns 404 for missing runs.
func TestGetAgentRunNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "GET", "/api/v1/agents/runs/no-such-run", "")
	if rec.Code != 404 {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

// TestTriggerAgentRunReturns202 verifies POST /api/v1/agents/{slug}/run returns 202 with runId.
func TestTriggerAgentRunReturns202(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	run := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/run", "")
	if run.Code != 202 {
		t.Errorf("status: got %d, want 202; body: %s", run.Code, run.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(run.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["runId"] == nil || resp["runId"] == "" {
		t.Errorf("expected non-empty runId in response")
	}
}

// TestAgentOutputFmtDefaultsToMarkdown verifies that outputFmt defaults to "markdown" when omitted.
func TestAgentOutputFmtDefaultsToMarkdown(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	body := `{"name":"Default Fmt","slug":"default-fmt","prompt":"p","trigger":"tap"}`
	rec := doRequest(t, mux, "POST", "/api/v1/agents", body)
	if rec.Code != 201 {
		t.Errorf("status: got %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["outputFmt"] != "markdown" {
		t.Errorf("outputFmt: got %v, want %q", resp["outputFmt"], "markdown")
	}
}

// TestReadOnlyHandlerAgentsWriteBlocked verifies write operations are blocked on tsnet handler.
func TestReadOnlyHandlerAgentsWriteBlocked(t *testing.T) {
	h := newReadOnlyHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code == 201 {
		t.Errorf("read-only handler must not allow agent creation, got 201")
	}
}

// TestGetLatestAgentRunNotFound verifies 404 when agent has no runs.
func TestGetLatestAgentRunNotFound(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	latest := doRequest(t, mux, "GET", "/api/v1/agents/test-agent/runs/latest", "")
	if latest.Code != 404 {
		t.Errorf("status: got %d, want 404; body: %s", latest.Code, latest.Body.String())
	}
}

// TestAgentRunsLatestAfterTrigger verifies the latest run endpoint returns data after a trigger.
func TestAgentRunsLatestAfterTrigger(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/agents", validAgentBody)
	if rec.Code != 201 {
		t.Fatalf("create agent: got %d", rec.Code)
	}

	// Trigger a run.
	run := doRequest(t, mux, "POST", "/api/v1/agents/test-agent/run", "")
	if run.Code != 202 {
		t.Fatalf("trigger run: got %d; body: %s", run.Code, run.Body.String())
	}

	// Fetch the run by ID.
	var triggerResp map[string]any
	if err := json.Unmarshal(run.Body.Bytes(), &triggerResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	runID := triggerResp["runId"].(string)

	// Poll until status != "running" (in tests the goroutine completes very quickly).
	var runResp map[string]any
	for range 20 {
		rec2 := doRequest(t, mux, "GET", "/api/v1/agents/runs/"+runID, "")
		if rec2.Code != 200 {
			t.Fatalf("get run: got %d; body: %s", rec2.Code, rec2.Body.String())
		}
		if err := json.Unmarshal(rec2.Body.Bytes(), &runResp); err != nil {
			t.Fatalf("unmarshal run: %v", err)
		}
		if runResp["status"] != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if runResp["status"] == "running" {
		t.Error("run status is still 'running' after polling — expected completion")
	}
	if !strings.HasPrefix(runResp["output"].(string), "#") {
		t.Errorf("markdown output should start with '#', got: %v", runResp["output"])
	}
}
