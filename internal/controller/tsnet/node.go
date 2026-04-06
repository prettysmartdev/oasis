// Package tsnet manages the Tailscale tsnet node for the oasis controller.
// The controller joins the user's tailnet as a named node (e.g. "oasis") using
// tsnet's userspace WireGuard implementation — no Tailscale daemon or
// CAP_NET_ADMIN capability is required.
package tsnet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"tailscale.com/tsnet"
)

// Node represents the oasis Tailscale tsnet node.
type Node struct {
	srv      *tsnet.Server
	hostname string
	stateDir string
	tsAPIKey string // optional Tailscale API key for hostname conflict resolution
	mu       sync.Mutex
	started  bool
}

// HostnameConflictError is returned by Start when the configured hostname is
// already in use by another node on the tailnet and automatic resolution failed.
type HostnameConflictError struct {
	// Configured is the hostname oasis was asked to use.
	Configured string
	// Actual is the hostname Tailscale actually assigned (e.g. "oasis-1").
	Actual string
}

// Error implements the error interface.
func (e *HostnameConflictError) Error() string {
	return fmt.Sprintf(
		"hostname %q is already in use on your tailnet (was assigned %q instead); "+
			"remove the conflicting node at https://login.tailscale.com/admin/machines "+
			"or set OASIS_HOSTNAME to a different value",
		e.Configured, e.Actual,
	)
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

// SetTailscaleAPIKey sets an optional Tailscale API key used to delete a
// conflicting node automatically when a hostname conflict is detected on
// startup. The key must have "Devices" write permission on the tailnet.
// If unset, a HostnameConflictError is returned without attempting deletion.
func (n *Node) SetTailscaleAPIKey(key string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tsAPIKey = key
}

// Start joins the tailnet and returns a net.Listener on the Tailscale interface.
// The tsnet.Server reads TS_AUTHKEY from the environment automatically on first run.
// TS_AUTHKEY is never logged by this package.
//
// If the configured hostname is already in use by another tailnet node, Start
// attempts to delete the conflicting node via the Tailscale API (requires
// SetTailscaleAPIKey to have been called with a key that has Devices write
// permission). If deletion succeeds, Start retries the connection. If it fails
// or no API key is set, Start returns a *HostnameConflictError.
func (n *Node) Start(ctx context.Context) (net.Listener, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return nil, fmt.Errorf("tsnet node already started")
	}

	srv, conflictErr, err := n.attemptStart(ctx)
	if err != nil {
		return nil, err
	}

	if conflictErr != nil {
		if n.tsAPIKey != "" && n.resolveConflict(ctx, conflictErr) {
			// Deletion succeeded — wait briefly for the change to propagate to
			// the coordination server, then retry the connection.
			srv.Close()
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			srv, conflictErr, err = n.attemptStart(ctx)
			if err != nil {
				return nil, err
			}
		}
		if conflictErr != nil {
			srv.Close()
			return nil, conflictErr
		}
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

// attemptStart creates a fresh tsnet.Server, starts it, waits for it to be
// running, and checks for a hostname conflict. It returns the started server
// (always non-nil on non-error return), a *HostnameConflictError if the
// assigned hostname differs from the configured one, and any hard error.
// The caller is responsible for closing the server on conflict or error.
func (n *Node) attemptStart(ctx context.Context) (*tsnet.Server, *HostnameConflictError, error) {
	srv := &tsnet.Server{
		Hostname: n.hostname,
		Dir:      n.stateDir,
	}

	if err := srv.Start(); err != nil {
		return nil, nil, fmt.Errorf("start tsnet server: %w", err)
	}

	if _, err := srv.Up(ctx); err != nil {
		srv.Close()
		return nil, nil, fmt.Errorf("tsnet up: %w", err)
	}

	conflictErr := n.detectHostnameConflict(ctx, srv)
	return srv, conflictErr, nil
}

// detectHostnameConflict uses the tsnet LocalClient to compare the hostname
// that Tailscale actually assigned against n.hostname. Returns a
// *HostnameConflictError if they differ, nil otherwise.
func (n *Node) detectHostnameConflict(ctx context.Context, srv *tsnet.Server) *HostnameConflictError {
	lc, err := srv.LocalClient()
	if err != nil {
		slog.Warn("tsnet: could not get local client for hostname check", "err", err)
		return nil
	}

	st, err := lc.Status(ctx)
	if err != nil {
		slog.Warn("tsnet: could not get status for hostname check", "err", err)
		return nil
	}

	if st.Self == nil || st.Self.DNSName == "" {
		return nil
	}

	// DNSName is like "oasis-1.tailnet-name.ts.net." — extract the hostname part.
	dnsName := strings.TrimSuffix(st.Self.DNSName, ".")
	actual := dnsName
	if idx := strings.IndexByte(dnsName, '.'); idx >= 0 {
		actual = dnsName[:idx]
	}

	if actual == n.hostname {
		return nil
	}

	slog.Warn("tsnet: hostname conflict detected",
		"configured", n.hostname,
		"assigned", actual,
		"dns_name", st.Self.DNSName,
	)
	return &HostnameConflictError{Configured: n.hostname, Actual: actual}
}

// resolveConflict attempts to delete the device that is occupying n.hostname
// from the tailnet via the Tailscale admin API. Returns true if the deletion
// request was accepted, false otherwise.
func (n *Node) resolveConflict(ctx context.Context, _ *HostnameConflictError) bool {
	slog.Info("tsnet: attempting to delete conflicting device via Tailscale API",
		"hostname", n.hostname,
	)

	deviceID, err := n.findConflictingDeviceID(ctx)
	if err != nil {
		slog.Warn("tsnet: could not find conflicting device", "err", err)
		return false
	}
	if deviceID == "" {
		slog.Warn("tsnet: conflicting device not found in tailnet device list")
		return false
	}

	if err := n.deleteDevice(ctx, deviceID); err != nil {
		slog.Warn("tsnet: failed to delete conflicting device", "hostname", n.hostname, "err", err)
		return false
	}

	slog.Info("tsnet: conflicting device deleted", "hostname", n.hostname, "id", deviceID)
	return true
}

// findConflictingDeviceID queries the Tailscale API and returns the NodeID of
// the device whose hostname equals n.hostname. Returns ("", nil) if not found.
func (n *Node) findConflictingDeviceID(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.tailscale.com/api/v2/tailnet/-/devices", nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(n.tsAPIKey, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list devices: HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Devices []struct {
			NodeID   string `json:"nodeId"`
			Hostname string `json:"hostname"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode devices: %w", err)
	}

	for _, dev := range result.Devices {
		if dev.Hostname == n.hostname {
			return dev.NodeID, nil
		}
	}
	return "", nil
}

// deleteDevice calls DELETE /api/v2/devices/{nodeID} on the Tailscale API.
func (n *Node) deleteDevice(ctx context.Context, nodeID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		"https://api.tailscale.com/api/v2/devices/"+nodeID, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(n.tsAPIKey, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
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
