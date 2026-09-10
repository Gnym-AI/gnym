package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Load reads, validates, and resolves a JSON configuration file.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := ensureEndOfFile(decoder); err != nil {
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	if err := config.loadPrompts(filepath.Dir(path)); err != nil {
		return Config{}, err
	}

	return config, nil
}

func ensureEndOfFile(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode config: multiple JSON values")
		}
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}

func (c *Config) loadPrompts(configDirectory string) error {
	for index := range c.Reviewers {
		reviewer := &c.Reviewers[index]
		if reviewer.PromptFile == "" {
			continue
		}

		promptPath := reviewer.PromptFile
		if !filepath.IsAbs(promptPath) {
			promptPath = filepath.Join(configDirectory, promptPath)
		}

		contents, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("reviewer %q prompt file %q: %w", reviewer.Name, reviewer.PromptFile, err)
		}
		if strings.TrimSpace(string(contents)) == "" {
			return fmt.Errorf("reviewer %q prompt file %q is empty", reviewer.Name, reviewer.PromptFile)
		}

		reviewer.Prompt = string(contents)
	}

	return nil
}
