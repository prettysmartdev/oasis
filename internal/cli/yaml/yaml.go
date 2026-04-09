// Package yaml provides YAML definition file parsing for oasis app and agent registration.
package yaml

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppDefinition represents an app parsed from a YAML definition file.
type AppDefinition struct {
	Name        string   `yaml:"name"`
	Slug        string   `yaml:"slug"`
	UpstreamURL string   `yaml:"upstreamUrl"`
	Description string   `yaml:"description"`
	Icon        string   `yaml:"icon"`
	Tags        []string `yaml:"tags"`
	// AccessType is "direct" (open upstream URL in a new tab) or "proxy"
	// (reverse-proxy through NGINX and open in an iFrame). Defaults to "proxy"
	// when omitted.
	AccessType string `yaml:"accessType"`
}

// AgentDefinition represents an agent parsed from a YAML definition file.
type AgentDefinition struct {
	Name        string `yaml:"name"`
	Slug        string `yaml:"slug"`
	Description string `yaml:"description"`
	Icon        string `yaml:"icon"`
	Prompt      string `yaml:"prompt"`
	Trigger     string `yaml:"trigger"`
	Schedule    string `yaml:"schedule"`
	OutputFmt   string `yaml:"outputFmt"`
}

// ParseAppFile reads and validates a YAML app definition file at path.
// Returns a descriptive error naming all missing required fields.
func ParseAppFile(path string) (*AppDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	var def AppDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML %s: %w", path, err)
	}

	var missing []string
	if def.Name == "" {
		missing = append(missing, "name")
	}
	if def.Slug == "" {
		missing = append(missing, "slug")
	}
	if def.UpstreamURL == "" {
		missing = append(missing, "upstreamUrl")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	if def.AccessType == "" {
		def.AccessType = "proxy"
	} else if def.AccessType != "direct" && def.AccessType != "proxy" {
		return nil, fmt.Errorf("accessType must be one of: direct, proxy (got %q)", def.AccessType)
	}
	return &def, nil
}

// ParseAgentFile reads and validates a YAML agent definition file at path.
// Returns a descriptive error naming all missing required fields.
// Defaults OutputFmt to "markdown" if not set.
func ParseAgentFile(path string) (*AgentDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	var def AgentDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML %s: %w", path, err)
	}

	var missing []string
	if def.Name == "" {
		missing = append(missing, "name")
	}
	if def.Slug == "" {
		missing = append(missing, "slug")
	}
	if def.Prompt == "" {
		missing = append(missing, "prompt")
	}
	if def.Trigger == "" {
		missing = append(missing, "trigger")
	}
	if def.Trigger == "schedule" && def.Schedule == "" {
		missing = append(missing, "schedule")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	if def.OutputFmt == "" {
		def.OutputFmt = "markdown"
	}
	return &def, nil
}
