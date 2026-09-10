package config

import "encoding/json"

// Config describes a review pipeline loaded from a JSON configuration file.
type Config struct {
	Reviewers []Reviewer `json:"reviewers"`
	Sink      Sink       `json:"sink"`
}

// Reviewer describes one named review and its provider-specific options.
// Prompt is populated from PromptFile when the configuration is loaded.
type Reviewer struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	PromptFile string          `json:"prompt_file,omitempty"`
	Options    json.RawMessage `json:"options,omitempty"`
	Prompt     string          `json:"-"`
}

// Sink describes where a completed review run should be written.
type Sink struct {
	Type    string          `json:"type"`
	Options json.RawMessage `json:"options,omitempty"`
}
