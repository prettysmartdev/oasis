package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ChatMessage represents a single turn in the persistent chat history.
type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`    // "user" | "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateChatMessage inserts a new chat message record.
func (s *Store) CreateChatMessage(ctx context.Context, msg ChatMessage) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO chat_messages (id, role, content, created_at)
VALUES (?, ?, ?, ?)`,
		msg.ID, msg.Role, msg.Content,
		msg.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// ListChatMessages returns all chat messages ordered by creation time (oldest first).
func (s *Store) ListChatMessages(ctx context.Context) ([]ChatMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, role, content, created_at FROM chat_messages ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var (
			m        ChatMessage
			createdS string
		)
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &createdS); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdS)
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// scanChatMessage scans a single chat message row.
func scanChatMessage(s scanner) (ChatMessage, error) {
	var (
		m        ChatMessage
		createdS string
	)
	err := s.Scan(&m.ID, &m.Role, &m.Content, &createdS)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatMessage{}, ErrNotFound
	}
	if err != nil {
		return ChatMessage{}, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdS)
	return m, nil
}
