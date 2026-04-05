// Package api implements the oasis management HTTP API handlers.
package api

// Handler holds the dependencies for the management API.
// Routes are registered in future work items; all requests currently return 501.
type Handler struct{}

// New creates a new Handler.
func New() *Handler {
	return &Handler{}
}
