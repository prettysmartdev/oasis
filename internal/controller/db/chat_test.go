package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestCreateChatMessagePersistsAllFields verifies that all fields round-trip through the DB.
func TestCreateChatMessagePersistsAllFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	msg := ChatMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   "Hello, world!",
		CreatedAt: now,
	}
	if err := s.CreateChatMessage(ctx, msg); err != nil {
		t.Fatalf("CreateChatMessage error: %v", err)
	}

	msgs, err := s.ListChatMessages(ctx)
	if err != nil {
		t.Fatalf("ListChatMessages error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	got := msgs[0]
	if got.ID != msg.ID {
		t.Errorf("ID: got %q, want %q", got.ID, msg.ID)
	}
	if got.Role != msg.Role {
		t.Errorf("Role: got %q, want %q", got.Role, msg.Role)
	}
	if got.Content != msg.Content {
		t.Errorf("Content: got %q, want %q", got.Content, msg.Content)
	}
	if !got.CreatedAt.Equal(msg.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, msg.CreatedAt)
	}
}

// TestListChatMessagesOrderedByCreatedAt verifies messages are returned in ASC order by created_at.
func TestListChatMessagesOrderedByCreatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)

	older := ChatMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   "First message",
		CreatedAt: base,
	}
	newer := ChatMessage{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   "Second message",
		CreatedAt: base.Add(time.Minute),
	}

	if err := s.CreateChatMessage(ctx, older); err != nil {
		t.Fatalf("CreateChatMessage older: %v", err)
	}
	if err := s.CreateChatMessage(ctx, newer); err != nil {
		t.Fatalf("CreateChatMessage newer: %v", err)
	}

	msgs, err := s.ListChatMessages(ctx)
	if err != nil {
		t.Fatalf("ListChatMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != older.ID {
		t.Errorf("first message: got ID %q, want %q (older)", msgs[0].ID, older.ID)
	}
	if msgs[1].ID != newer.ID {
		t.Errorf("second message: got ID %q, want %q (newer)", msgs[1].ID, newer.ID)
	}
}

// TestChatMessageRoundTrip verifies multiple messages of different roles are stored and returned.
func TestChatMessageRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	messages := []ChatMessage{
		{
			ID:        uuid.New().String(),
			Role:      "user",
			Content:   "User message one",
			CreatedAt: base,
		},
		{
			ID:        uuid.New().String(),
			Role:      "user",
			Content:   "User message two",
			CreatedAt: base.Add(time.Second),
		},
		{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   "Assistant response",
			CreatedAt: base.Add(2 * time.Second),
		},
	}

	for _, m := range messages {
		if err := s.CreateChatMessage(ctx, m); err != nil {
			t.Fatalf("CreateChatMessage %q: %v", m.Role, err)
		}
	}

	got, err := s.ListChatMessages(ctx)
	if err != nil {
		t.Fatalf("ListChatMessages error: %v", err)
	}
	if len(got) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(got))
	}
	for i, want := range messages {
		if got[i].ID != want.ID {
			t.Errorf("messages[%d].ID: got %q, want %q", i, got[i].ID, want.ID)
		}
		if got[i].Role != want.Role {
			t.Errorf("messages[%d].Role: got %q, want %q", i, got[i].Role, want.Role)
		}
		if got[i].Content != want.Content {
			t.Errorf("messages[%d].Content: got %q, want %q", i, got[i].Content, want.Content)
		}
	}
}
