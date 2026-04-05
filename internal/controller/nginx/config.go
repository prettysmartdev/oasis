// Package nginx generates and applies NGINX configuration for the oasis gateway.
// Configuration is produced programmatically (no string templating) and applied
// via SIGHUP for graceful reload without dropping connections.
package nginx

// Configurator generates NGINX configuration from the app registry state
// and signals NGINX to reload when the configuration changes.
type Configurator struct{}

// New creates a new Configurator.
func New() *Configurator {
	return &Configurator{}
}
