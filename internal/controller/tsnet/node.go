// Package tsnet manages the Tailscale tsnet node for the oasis controller.
// The controller joins the user's tailnet as a named node (e.g. "oasis") using
// tsnet's userspace WireGuard implementation — no Tailscale daemon or
// CAP_NET_ADMIN capability is required.
package tsnet

// tsnet is imported in the integration layer when the node is started.
// See: https://pkg.go.dev/tailscale.com/tsnet

// Node represents the oasis Tailscale tsnet node.
type Node struct{}

// New creates a new Node. Call Start to join the tailnet.
func New() *Node {
	return &Node{}
}
