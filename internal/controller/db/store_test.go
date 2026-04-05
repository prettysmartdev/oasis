package db

import "testing"

// TestStoreNew confirms New returns a non-nil *Store without error.
func TestStoreNew(t *testing.T) {
	s, err := New("")
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("New() returned nil store")
	}
}

// TestStoreType confirms the returned value is of type *Store.
func TestStoreType(t *testing.T) {
	s, _ := New(":memory:")
	var _ *Store = s
}
