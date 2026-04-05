package nginx

import "testing"

// TestConfiguratorNew confirms New returns a non-nil *Configurator.
func TestConfiguratorNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

// TestConfiguratorType confirms the returned value is of type *Configurator.
func TestConfiguratorType(t *testing.T) {
	c := New()
	var _ *Configurator = c
}
