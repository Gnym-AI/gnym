package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	directory := t.TempDir()
	prompt := "# Correctness\n\nReview this diff for correctness.\n"
	writeTestFile(t, filepath.Join(directory, "prompts", "correctness.md"), prompt)
	configPath := filepath.Join(directory, "gnym.json")
	writeTestFile(t, configPath, `{
  "reviewers": [{
    "name": "correctness",
    "type": "stub",
    "prompt_file": "prompts/correctness.md",
    "options": {"model": "test-model"}
  }],
  "sink": {
    "type": "file",
    "options": {"path": "review-results.json"}
  }
}`)

	got, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Reviewers[0].Prompt != prompt {
		t.Errorf("Load() prompt = %q, want %q", got.Reviewers[0].Prompt, prompt)
	}
	if string(got.Reviewers[0].Options) != `{"model": "test-model"}` {
		t.Errorf("Load() reviewer options = %s", got.Reviewers[0].Options)
	}
}

func TestLoadAllowsReviewerWithoutPromptFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gnym.json")
	writeTestFile(t, configPath, `{
  "reviewers": [{"name": "stub", "type": "stub"}],
  "sink": {"type": "file", "options": {"path": "results.json"}}
}`)

	got, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Reviewers[0].Prompt != "" {
		t.Errorf("Load() prompt = %q, want empty prompt", got.Reviewers[0].Prompt)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		contents   string
		wantErrSub string
	}{
		{
			name:       "no reviewers",
			contents:   `{"reviewers": [], "sink": {"type": "file"}}`,
			wantErrSub: "no reviewers configured",
		},
		{
			name:       "reviewer without name",
			contents:   `{"reviewers": [{"type": "stub"}], "sink": {"type": "file"}}`,
			wantErrSub: "reviewer 1 has no name",
		},
		{
			name:       "reviewer without type",
			contents:   `{"reviewers": [{"name": "correctness"}], "sink": {"type": "file"}}`,
			wantErrSub: `reviewer "correctness" has no type`,
		},
		{
			name:       "duplicate reviewer name",
			contents:   `{"reviewers": [{"name": "same", "type": "stub"}, {"name": "same", "type": "stub"}], "sink": {"type": "file"}}`,
			wantErrSub: `duplicate reviewer name "same"`,
		},
		{
			name:       "sink without type",
			contents:   `{"reviewers": [{"name": "stub", "type": "stub"}], "sink": {}}`,
			wantErrSub: "sink has no type",
		},
		{
			name:       "unknown field",
			contents:   `{"reviewers": [{"name": "stub", "type": "stub", "unexpected": true}], "sink": {"type": "file"}}`,
			wantErrSub: "unknown field",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "gnym.json")
			writeTestFile(t, configPath, test.contents)

			_, err := Load(configPath)
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("Load() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func TestLoadRejectsMissingOrEmptyPromptFile(t *testing.T) {
	tests := []struct {
		name       string
		promptPath string
		create     bool
		contents   string
		wantErrSub string
	}{
		{
			name:       "missing",
			promptPath: "prompts/missing.md",
			wantErrSub: `reviewer "correctness" prompt file "prompts/missing.md"`,
		},
		{
			name:       "empty",
			promptPath: "prompts/empty.md",
			create:     true,
			contents:   " \n\t",
			wantErrSub: "is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			if test.create {
				writeTestFile(t, filepath.Join(directory, test.promptPath), test.contents)
			}
			configPath := filepath.Join(directory, "gnym.json")
			writeTestFile(t, configPath, `{
  "reviewers": [{"name": "correctness", "type": "stub", "prompt_file": "`+test.promptPath+`"}],
  "sink": {"type": "file"}
}`)

			_, err := Load(configPath)
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("Load() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func TestConfigFileCanOnlyContainOneObject(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "gnym.json")
	writeTestFile(t, configPath, `{
  "reviewers": [{"name": "stub", "type": "stub"}],
  "sink": {"type": "file", "options": {"path": "results.json"}}
}
{
  "reviewers": [{"name": "stub", "type": "stub"}],
  "sink": {"type": "file", "options": {"path": "results.json"}}
}`)

	_, err := Load(configPath)
	errorMessage := "decode config: multiple JSON values"

	if err == nil || !strings.Contains(err.Error(), errorMessage) {
		t.Fatalf("Load() error = %v, want error containing %q", err, "only one object")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
