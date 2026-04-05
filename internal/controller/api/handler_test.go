package api

import "testing"

// TestHandlerNew confirms New returns a non-nil *Handler.
func TestHandlerNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("New() returned nil")
	}
}

// TestHandlerType confirms the returned value is of type *Handler.
func TestHandlerType(t *testing.T) {
	h := New()
	// Compile-time assertion: h must be assignable to *Handler.
	var _ *Handler = h
}
