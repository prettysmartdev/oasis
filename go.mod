// TODO: update [owner] to the actual GitHub org/user before first release
module github.com/[owner]/oasis

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.8.1
	modernc.org/sqlite v1.29.10
)

// tailscale.com/tsnet is added when the tsnet integration is wired up.
// go get tailscale.com@latest and run go mod tidy before that work item.
