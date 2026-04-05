// Package tsnet manages the Tailscale tsnet node for the oasis controller.
// The controller joins the user's tailnet as a named node (e.g. "oasis") using
// tsnet's userspace WireGuard implementation — no Tailscale daemon or
// CAP_NET_ADMIN capability is required.
package tsnet

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"tailscale.com/tsnet"
)

// Node represents the oasis Tailscale tsnet node.
type Node struct {
	srv      *tsnet.Server
	hostname string
	stateDir string
	mu       sync.Mutex
	started  bool
}

// New creates a zero-value Node. Use NewNode for a configured node.
func New() *Node {
	return &Node{}
}

// NewNode creates a Node configured with the given Tailscale hostname and state directory.
func NewNode(hostname, stateDir string) *Node {
	return &Node{
		hostname: hostname,
		stateDir: stateDir,
	}
}

// Start joins the tailnet and returns a net.Listener on the Tailscale interface.
// The tsnet.Server reads TS_AUTHKEY from the environment automatically on first run.
// TS_AUTHKEY is never logged by this package.
func (n *Node) Start(ctx context.Context) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return nil, fmt.Errorf("tsnet node already started")
	}

	srv := &tsnet.Server{
		Hostname: n.hostname,
		Dir:      n.stateDir,
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("start tsnet server: %w", err)
	}

	// Wait for the node to be running before returning a listener.
	if _, err := srv.Up(ctx); err != nil {
		srv.Close()
		return nil, fmt.Errorf("tsnet up: %w", err)
	}

	ln, err := srv.Listen("tcp", ":80")
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("tsnet listen: %w", err)
	}

	n.srv = srv
	n.started = true
	return ln, nil
}

// TailscaleIP returns the node's IPv4 Tailscale address.
// Returns an error if the node has not been started.
func (n *Node) TailscaleIP() (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started || n.srv == nil {
		return "", fmt.Errorf("tsnet node not started")
	}

	ip4, _ := n.srv.TailscaleIPs()
	if !ip4.IsValid() {
		return "", fmt.Errorf("no tailscale IPv4 address available")
	}
	return ip4.String(), nil
}

// Close gracefully shuts down the tsnet node.
func (n *Node) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.srv == nil {
		return nil
	}
	err := n.srv.Close()
	n.started = false
	n.srv = nil
	return err
}

// HTTPClient returns an HTTP client that dials over the tsnet interface.
// Returns a plain http.Client if the node is not started.
func (n *Node) HTTPClient() *http.Client {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.srv == nil {
		return &http.Client{}
	}
	return n.srv.HTTPClient()
}

// IsStarted reports whether the tsnet node is currently running.
func (n *Node) IsStarted() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.started
}
