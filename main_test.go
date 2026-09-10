package main

import (
	"bytes"
	"gnym/config"
	"gnym/input"
	"gnym/reviewer"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunReview(t *testing.T) {
	directory := t.TempDir()
	diffPath := filepath.Join(directory, "changes.diff")
	writeMainTestFile(t, diffPath, "diff --git a/example.go b/example.go\n")
	outputPath := filepath.Join(directory, "review.json")
	configPath := filepath.Join(directory, "gnym.json")
	writeMainTestFile(t, configPath, `{
  "reviewers": [{"name": "correctness", "type": "stub"}],
  "sink": {"type": "file", "options": {"path": "`+outputPath+`"}}
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"review",
		"--config", configPath,
		"--diff-source", "file://" + diffPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr.String())
	}

	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(contents), `"Reviewer": "correctness"`) {
		t.Errorf("review output = %s", contents)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	err := run([]string{"version"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := stdout.String(), "gnym v0.1.0\n"; got != want {
		t.Errorf("run() output = %q, want %q", got, want)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name       string
		arguments  []string
		wantErrSub string
	}{
		{name: "missing command", wantErrSub: "expected a command"},
		{name: "unknown command", arguments: []string{"other"}, wantErrSub: `unknown command "other"`},
		{name: "version arguments", arguments: []string{"version", "extra"}, wantErrSub: "does not accept arguments"},
		{name: "missing config", arguments: []string{"review", "--diff-source", "file://changes.diff"}, wantErrSub: "requires --config"},
		{name: "missing diff source", arguments: []string{"review", "--config", "gnym.json"}, wantErrSub: "requires --diff-source"},
		{name: "positional argument", arguments: []string{"review", "extra", "--config", "gnym.json", "--diff-source", "file://changes.diff"}, wantErrSub: "does not accept positional arguments"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.arguments, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("run() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func TestBuildDiffSourceSupportsFileReferences(t *testing.T) {
	tests := []struct {
		reference string
		wantPath  string
	}{
		{reference: "file:changes.diff", wantPath: "changes.diff"},
		{reference: "file://changes.diff", wantPath: "changes.diff"},
		{reference: "file:///tmp/changes.diff", wantPath: "/tmp/changes.diff"},
	}

	for _, test := range tests {
		t.Run(test.reference, func(t *testing.T) {
			source, err := buildDiffSource(test.reference)
			if err != nil {
				t.Fatalf("buildDiffSource() error = %v", err)
			}
			fileSource, ok := source.(*input.FileDiffSource)
			if !ok {
				t.Fatalf("buildDiffSource() type = %T", source)
			}
			if fileSource.Path != test.wantPath {
				t.Errorf("buildDiffSource() path = %q, want %q", fileSource.Path, test.wantPath)
			}
		})
	}
}

func TestBuildDiffSourceRejectsUnsupportedType(t *testing.T) {
	tests := []struct {
		reference  string
		wantErrSub string
	}{
		{reference: "changes.diff", wantErrSub: "must include a scheme"},
		{reference: "file://", wantErrSub: "requires a path"},
		{reference: "file://host/path", wantErrSub: "invalid file diff source"},
		{reference: "github://Gnym-AI/gnym/pull/42", wantErrSub: `diff source type "github" is not registered`},
	}

	for _, test := range tests {
		t.Run(test.reference, func(t *testing.T) {
			_, err := buildDiffSource(test.reference)
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("buildDiffSource() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func TestBuildReviewers(t *testing.T) {
	configured := []config.Reviewer{
		{
			Name:    "correctness",
			Type:    "stub",
			Prompt:  "Review for correctness.",
			Options: []byte(`{"model": "test-model"}`),
		},
	}

	router, configs, err := buildReviewers(configured)
	if err != nil {
		t.Fatalf("buildReviewers() error = %v", err)
	}
	if _, ok := router["correctness"].(*reviewer.Stub); !ok {
		t.Fatalf("buildReviewers() reviewer type = %T", router["correctness"])
	}
	wantConfigs := []reviewer.Config{
		{Name: "correctness", Provider: "stub", Model: "test-model", Prompt: "Review for correctness."},
	}
	if !reflect.DeepEqual(configs, wantConfigs) {
		t.Errorf("buildReviewers() configs = %#v, want %#v", configs, wantConfigs)
	}
}

func TestBuildReviewersRejectsUnsupportedType(t *testing.T) {
	tests := []struct {
		name       string
		configured config.Reviewer
		wantErrSub string
	}{
		{
			name:       "unsupported type",
			configured: config.Reviewer{Name: "security", Type: "anthropic"},
			wantErrSub: `reviewer type "anthropic" is not registered`,
		},
		{
			name:       "unknown option",
			configured: config.Reviewer{Name: "correctness", Type: "stub", Options: []byte(`{"other": true}`)},
			wantErrSub: `reviewer "correctness": options`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := buildReviewers([]config.Reviewer{test.configured})
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("buildReviewers() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func TestReviewerRouterRejectsUnregisteredReviewer(t *testing.T) {
	_, err := (reviewerRouter{}).Review(reviewer.Request{Config: reviewer.Config{Name: "missing"}})
	if err == nil || !strings.Contains(err.Error(), `reviewer "missing" is not registered`) {
		t.Fatalf("Review() error = %v", err)
	}
}

func TestBuildSinkValidatesFileOptions(t *testing.T) {
	tests := []struct {
		name       string
		configured config.Sink
		wantErrSub string
	}{
		{
			name:       "missing path",
			configured: config.Sink{Type: "file"},
			wantErrSub: "requires options.path",
		},
		{
			name:       "unknown option",
			configured: config.Sink{Type: "file", Options: []byte(`{"other": true}`)},
			wantErrSub: "unknown field",
		},
		{
			name:       "unsupported type",
			configured: config.Sink{Type: "github"},
			wantErrSub: `sink type "github" is not registered`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildSink(test.configured)
			if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("buildSink() error = %v, want error containing %q", err, test.wantErrSub)
			}
		})
	}
}

func writeMainTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
