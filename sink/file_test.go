package sink

import (
	"encoding/json"
	"errors"
	"gnym/review"
	"gnym/reviewer"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleRun() review.Run {
	return review.Run{
		SchemaVersion: "1", Status: review.StatusComplete,
		Results: []reviewer.Result{{Reviewer: "test-reviewer", CreatedAt: "2026-09-12T12:00:00Z", Summary: "could be better", Comments: []reviewer.Comment{
			{File: "file.go", Side: "new", Line: 1, EndLine: 9000, Severity: reviewer.SeverityMedium, Message: "this is a comment"},
			{File: "file.go", Severity: reviewer.SeverityHigh, Message: "this is another comment"},
		}}},
	}
}

func TestFileWritesEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.json")
	fileSink := File{Path: path}
	if err := fileSink.Save(sampleRun()); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(contents, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"schema_version":"1","status":"complete","reviews":[{"reviewer":"test-reviewer","created_at":"2026-09-12T12:00:00Z","summary":"could be better","comments":[{"file":"file.go","side":"new","line":1,"end_line":9000,"severity":"medium","message":"this is a comment"},{"file":"file.go","severity":"high","message":"this is another comment"}]}],"failures":[]}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("output = %s", contents)
	}
}

func TestFileAcceptsConstructedPartialAndFailedRuns(t *testing.T) {
	for _, status := range []review.Status{review.StatusPartial, review.StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			r := sampleRun()
			r.Status = status
			r.Failures = []review.Failure{{Reviewer: "other", Code: review.FailureInvalidOutput, Message: "invalid payload"}}
			if status == review.StatusFailed {
				r.Results = nil
			}
			path := filepath.Join(t.TempDir(), "review.json")
			if err := (&File{Path: path}).Save(r); err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var output struct {
				Status   string            `json:"status"`
				Reviews  []json.RawMessage `json:"reviews"`
				Failures []json.RawMessage `json:"failures"`
			}
			if err := json.Unmarshal(contents, &output); err != nil {
				t.Fatal(err)
			}
			if output.Status != string(status) || output.Reviews == nil || len(output.Reviews) != len(r.Results) || len(output.Failures) != 1 {
				t.Fatalf("invalid shape: %s", contents)
			}
		})
	}
}

func TestFileRejectsInvalidRunBeforeWriting(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, test := range []struct {
			name       string
			invalidate func(*review.Run)
		}{
			{"version", func(r *review.Run) { r.SchemaVersion = "2" }},
			{"timestamp", func(r *review.Run) { r.Results[0].CreatedAt = "" }},
			{"comment", func(r *review.Run) { r.Results[0].Comments[1].Severity = "error" }},
			{"duplicate", func(r *review.Run) { r.Results = append(r.Results, r.Results[0]) }},
			{"cardinality", func(r *review.Run) { r.Status = review.StatusFailed }},
		} {
			t.Run(test.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "review.json")
				if existing {
					if err := os.WriteFile(path, []byte("preserve"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				r := sampleRun()
				test.invalidate(&r)
				if err := (&File{Path: path}).Save(r); err == nil {
					t.Fatal("accepted invalid run")
				}
				contents, err := os.ReadFile(path)
				if existing {
					if err != nil || string(contents) != "preserve" {
						t.Fatalf("modified existing output: %q %v", contents, err)
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("created output: %s %v", contents, err)
				}
			})
		}
	}
}

func TestFileReturnsWriteError(t *testing.T) {
	err := (&File{Path: t.TempDir()}).Save(sampleRun())
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("expected write error, got %v", err)
	}
}
