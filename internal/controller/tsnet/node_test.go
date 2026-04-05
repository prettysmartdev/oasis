package tsnet

import "testing"

// TestNodeNew confirms New returns a non-nil *Node.
func TestNodeNew(t *testing.T) {
	n := New()
	if n == nil {
		t.Fatal("New() returned nil")
	}
}

// TestNodeType confirms the returned value is of type *Node.
func TestNodeType(t *testing.T) {
	n := New()
	var _ *Node = n
}
