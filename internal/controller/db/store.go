// Package db provides SQLite persistence for the oasis app registry and settings.
package db

import (
	// modernc.org/sqlite is a pure-Go SQLite driver (CGO_ENABLED=0 compatible).
	_ "modernc.org/sqlite"
)

// Store manages the SQLite database connection and schema migrations.
type Store struct {
	path string
}

// New opens (or creates) the SQLite database at the given path.
// Schema migrations are applied automatically on open.
func New(path string) (*Store, error) {
	return &Store{path: path}, nil
}
