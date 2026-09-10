package sink

import (
	"encoding/json"
	"gnym/review"
	"gnym/reviewer"
	"os"
	"path/filepath"
	"testing"
)

func assertFile(t *testing.T, expectedPath string, expectedRun review.Run) {
	t.Helper()

	expectedContents, err := json.MarshalIndent(expectedRun.Results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	actualContents, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(actualContents) != string(expectedContents) {
		t.Errorf("expected file contents to be %s, but got %s", string(expectedContents), string(actualContents))
	}
}

func TestFileWritesContentToJson(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")
	fileSink := File{
		Path: path,
	}

	reviewerRun := review.Run{
		Results: []reviewer.Result{
			{
				Reviewer: "test-reviewer",
				Summary:  "could be better",
				Comments: []reviewer.Comment{
					{
						File:     "file.go",
						Line:     1,
						Severity: reviewer.SeverityWarning,
						Message:  "this is a comment",
					},
					{
						File:     "file.go",
						Line:     2,
						Severity: reviewer.SeverityError,
						Message:  "this is another comment",
					},
				},
			},
		},
	}

	err := fileSink.Save(reviewerRun)
	if err != nil {
		t.Fatal(err)
	}

	assertFile(t, path, reviewerRun)
}

func TestFileReturnsErrorIfNotWritten(t *testing.T) {
	fileSink := File{}
	err := fileSink.Save(review.Run{})
	if err == nil {
		t.Fatal("expected error, but got nil")
	}
}
