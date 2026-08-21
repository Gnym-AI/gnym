package input

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), 0644)
	if err != nil {
		t.Fatalf("write test file %q: %v", path, err)
	}
}

func TestFileContentsGoIntoDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")
	writeTestFile(t, path, "test contents")
	diffSource := FileDiffSource{
		Path: path,
	}

	diff, err := diffSource.GetDiff()
	if err != nil {
		t.Fatalf("reading from test file failed %q: %v", diffSource.Path, err)
	}

	if diff.Content != "test contents" {
		t.Fatalf("expected diff to be %q, got %q", "test contents", diff.Content)
	}
}

func TestExpectErrorIfFileDoesNotExist(t *testing.T) {
	diffSource := FileDiffSource{
		Path: "nonexistentfile",
	}

	_, err := diffSource.GetDiff()
	if err == nil {
		t.Fatalf("expected error reading from nonexistent file")
	}
}
