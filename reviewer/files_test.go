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
		{"rename with spaces", "diff --git a/old name b/new name\nsimilarity index 100%\nrename from old name\nrename to new name", []string{"old name", "new name"}},
		{"copy retains components", "diff --git a/a/source b/b/target\ncopy from a/source\ncopy to b/target", []string{"a/source", "b/target"}},
		{"binary", "diff --git a/pic.png b/pic.png\nBinary files a/pic.png and b/pic.png differ", []string{"pic.png"}},
		{"mode only", "diff --git a/script b/script\nold mode 100644\nnew mode 100755", []string{"script"}},
		{"quoted paths", `diff --git "a/hello\040world" "b/hello\040world"`, []string{"hello world"}},
		{"quoted utf8", `diff --git "a/caf\303\251.go" "b/caf\303\251.go"`, []string{"café.go"}},
		{"standalone timestamp", "--- a/a name\t2026-01-01\n+++ b/a name\t2026-01-01", []string{"a name"}},
		{"plain standalone", "--- dir/file\n+++ dir/file", []string{"dir/file"}},
		{"body metadata", "diff --git a/real b/real\n@@ -1,2 +1,2 @@\n shared\n--- a/fake\n+++ b/fake", []string{"real"}},
		{"binary body", "diff --git a/pic b/pic\nGIT binary patch\n--- a/fake\n+++ b/fake\nrename from fake2", []string{"pic"}},
		{"successive sections", "diff --git a/a b/a\n@@ -1 +1 @@\n-old\n+new\ndiff --git a/b b/b", []string{"a", "b"}},
		{"unsafe", "--- a/../bad\n+++ /absolute", nil},
		{"combined", "diff --cc combined\n--- a/fake\n+++ b/fake", nil},
		{"custom prefixes", "diff --git old/a new/a", nil},
		{"ambiguous spaces", "diff --git a/a b/inside b/a b/inside", nil},
		{"malformed quote", `diff --git "a/bad\q" "b/bad\q"`, nil},
		{"unpaired", "--- a/nope", nil},
		{"empty", "", nil},
		{"arbitrary", "just text", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := map[string]struct{}{}
			for _, name := range test.want {
				want[name] = struct{}{}
			}
			if got := DiffFiles(test.diff); !reflect.DeepEqual(got, want) {
				t.Fatalf("DiffFiles() = %#v, want %#v", got, want)
			}
		})
	}
}
