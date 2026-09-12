package reviewer

import (
	"reflect"
	"testing"
)

func TestDiffFiles(t *testing.T) {
	tests := []struct {
		name, diff string
		want       []string
	}{
		{"modify", "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new", []string{"a.go"}},
		{"add", "--- /dev/null\n+++ b/new.go\n@@ -0,0 +1 @@\n+new", []string{"new.go"}},
		{"delete", "--- a/old.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-old", []string{"old.go"}},
		{"rename", "diff --git a/old name b/new name\nsimilarity index 100%\nrename from old name\nrename to new name", []string{"old name", "new name"}},
		{"copy", "diff --git a/a b/b\ncopy from a\ncopy to b", []string{"a", "b"}},
		{"binary", "diff --git a/pic.png b/pic.png\nBinary files a/pic.png and b/pic.png differ", []string{"pic.png"}},
		{"mode", "diff --git a/script b/script\nold mode 100644\nnew mode 100755", []string{"script"}},
		{"quoted", `diff --git "a/hello\040world" "b/hello\040world"`, []string{"hello world"}},
		{"quote escape", `diff --git "a/a\"b" "b/a\"b"`, []string{`a"b`}},
		{"timestamp", "--- a/a name\t2026-01-01\n+++ b/a name\t2026-01-01", []string{"a name"}},
		{"plain standalone", "--- dir/file\n+++ dir/file", []string{"dir/file"}},
		{"body metadata", "diff --git a/real b/real\n@@ -1 +1 @@\n--- a/fake\n+++ b/fake", []string{"real"}},
		{"body rename", "diff --git a/real b/real\n@@ -1 +1 @@\n-rename from fake\n+rename to fake", []string{"real"}},
		{"second section", "diff --git a/a b/a\n@@ -1 +1 @@\n-old\n+new\ndiff --git a/b b/b", []string{"a", "b"}},
		{"unsafe", "--- a/../bad\n+++ /absolute", nil},
		{"combined", "diff --cc a\n@@@ -1 -1 +1 @@@\n--- a/fake\n+++ b/fake", nil},
		{"custom", "diff --git old/a new/a", nil},
		{"ambiguous", "diff --git a/a b/inside b/a b/inside", nil},
		{"unpaired", "--- a/nope", nil},
		{"empty", "", nil}, {"arbitrary", "just text", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := map[string]struct{}{}
			for _, name := range tt.want {
				want[name] = struct{}{}
			}
			if got := DiffFiles(tt.diff); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v want %#v", got, want)
			}
		})
	}
}
