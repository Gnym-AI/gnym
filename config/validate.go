package config

import (
	"fmt"
	"strings"
)

// Validate checks configuration shared by every reviewer and sink type.
// Provider-specific options are validated by the selected factory.
func (c Config) Validate() error {
	if len(c.Reviewers) == 0 {
		return fmt.Errorf("config: no reviewers configured")
	}

	names := make(map[string]struct{}, len(c.Reviewers))
	for index, reviewer := range c.Reviewers {
		if strings.TrimSpace(reviewer.Name) == "" {
			return fmt.Errorf("config: reviewer %d has no name", index+1)
		}
		if strings.TrimSpace(reviewer.Type) == "" {
			return fmt.Errorf("config: reviewer %q has no type", reviewer.Name)
		}
		if _, exists := names[reviewer.Name]; exists {
			return fmt.Errorf("config: duplicate reviewer name %q", reviewer.Name)
		}
		names[reviewer.Name] = struct{}{}
	}

	if strings.TrimSpace(c.Sink.Type) == "" {
		return fmt.Errorf("config: sink has no type")
	}

	return nil
}
