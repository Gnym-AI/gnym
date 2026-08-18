package input

import (
	"os"
	"testing"
)

func TestReadDiff(t *testing.T) {
	content := "diff --git a/main.go b/main.go\n+hello world"

	file, err := os.CreateTemp(t.TempDir(), "*.diff")
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	result, err := ReadDiff(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if result != content {
		t.Errorf("Expected %q, got %q", content, result)
	}
}

func TestReadDiffReturnsErrorForMissingFile(t *testing.T) {
	_, err := ReadDiff("/this/file/does/not/exist.diff")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
