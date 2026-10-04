package config

import "fmt"

// JSONConfig selects the JSON package the generated server uses.
type JSONConfig struct {
	// Version is "v1" for encoding/json, the default, or "v2" for
	// encoding/json/v2. handler.Server.SetJSONVersion overrides it at setup.
	Version string `yaml:"version,omitempty"`
}

// Check reports an unknown version.
func (c JSONConfig) Check() error {
	switch c.Version {
	case "", "v1", "v2":
		return nil
	default:
		return fmt.Errorf("unknown version %q: use v1 or v2", c.Version)
	}
}

// IsV2 reports whether the generated server defaults to encoding/json/v2.
func (c JSONConfig) IsV2() bool {
	return c.Version == "v2"
}
