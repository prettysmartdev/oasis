package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prettysmartdev/oasis/internal/controller/db"
)

// writeFakeChatClaude creates a fake claude binary that echoes "# chat response" to stdout.
func writeFakeChatClaude(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "claude")
	script := "#!/bin/sh\necho '# chat response'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writeFakeChatClaude: %v", err)
	}
	return binPath
}

// TestCreateChatMessageReturns200 verifies that a valid POST returns 200 with both messages.
func TestCreateChatMessageReturns200(t *testing.T) {
	fakeBin := writeFakeChatClaude(t)
	t.Setenv("OASIS_CLAUDE_BIN", fakeBin)

	h := newTestHandler(t)
	h.SetChatTimeout(5 * time.Second)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/chat/messages", `{"message":"hello"}`)
	if rec.Code != 200 {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UserMessage      chatMessageJSON `json:"userMessage"`
		AssistantMessage chatMessageJSON `json:"assistantMessage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.UserMessage.Role != "user" {
		t.Errorf("userMessage.role: got %q, want %q", resp.UserMessage.Role, "user")
	}
	if resp.AssistantMessage.Role != "assistant" {
		t.Errorf("assistantMessage.role: got %q, want %q", resp.AssistantMessage.Role, "assistant")
	}
	if resp.UserMessage.ID == "" {
		t.Error("userMessage.id must not be empty")
	}
	if resp.AssistantMessage.ID == "" {
		t.Error("assistantMessage.id must not be empty")
	}
}

// TestCreateChatMessageEmptyBody verifies that an empty message returns 400 INVALID_MESSAGE.
func TestCreateChatMessageEmptyBody(t *testing.T) {
	h := newTestHandler(t)
	h.SetChatTimeout(5 * time.Second)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/chat/messages", `{"message":""}`)
	if rec.Code != 400 {
		t.Fatalf("status: got %d, want 400; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["code"] != "INVALID_MESSAGE" {
		t.Errorf("code: got %v, want %q", resp["code"], "INVALID_MESSAGE")
	}
}

// TestCreateChatMessageExecutorUnavailable verifies that 503 EXECUTOR_UNAVAILABLE is returned
// when no claude binary is available.
func TestCreateChatMessageExecutorUnavailable(t *testing.T) {
	// Point PATH at an empty dir so LookPath("claude") fails.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("OASIS_CLAUDE_BIN", "")

	h := newTestHandler(t)
	h.SetChatTimeout(5 * time.Second)
	mux := serveMux(h)

	rec := doRequest(t, mux, "POST", "/api/v1/chat/messages", `{"message":"hi"}`)
	if rec.Code != 503 {
		t.Fatalf("status: got %d, want 503; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["code"] != "EXECUTOR_UNAVAILABLE" {
		t.Errorf("code: got %v, want %q", resp["code"], "EXECUTOR_UNAVAILABLE")
	}
}

// TestListChatMessagesEmpty verifies that GET /api/v1/chat/messages returns an empty list on a fresh store.
func TestListChatMessagesEmpty(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	rec := doRequest(t, mux, "GET", "/api/v1/chat/messages", "")
	if rec.Code != 200 {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("'items' is not a slice: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
	if resp["total"] != float64(0) {
		t.Errorf("total: got %v, want 0", resp["total"])
	}
}

// TestListChatMessagesOrder verifies that messages are returned oldest-first.
func TestListChatMessagesOrder(t *testing.T) {
	h := newTestHandler(t)
	mux := serveMux(h)

	base := time.Now().UTC().Truncate(time.Second)
	older := db.ChatMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   "older message",
		CreatedAt: base,
	}
	newer := db.ChatMessage{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   "newer message",
		CreatedAt: base.Add(time.Minute),
	}

	ctx := context.Background()
	if err := h.store.CreateChatMessage(ctx, older); err != nil {
		t.Fatalf("CreateChatMessage older: %v", err)
	}
	if err := h.store.CreateChatMessage(ctx, newer); err != nil {
		t.Fatalf("CreateChatMessage newer: %v", err)
	}

	rec := doRequest(t, mux, "GET", "/api/v1/chat/messages", "")
	if rec.Code != 200 {
		t.Fatalf("status: got %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Items []chatMessageJSON `json:"items"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != older.ID {
		t.Errorf("first item: got %q, want older %q", resp.Items[0].ID, older.ID)
	}
	if resp.Items[1].ID != newer.ID {
		t.Errorf("second item: got %q, want newer %q", resp.Items[1].ID, newer.ID)
	}
}
