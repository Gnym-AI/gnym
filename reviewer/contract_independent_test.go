package reviewer_test

import (
	"encoding/json"
	"fmt"
	"gnym/reviewer"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestIndependentMetadataBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name, diff string
		want       []string
	}{
		{"successive standalone files", "--- a/one\n+++ b/one\n@@ -1 +1 @@\n-old\n+new\n--- a/two\n+++ b/two\n@@ -0,0 +1 @@\n+x", []string{"one", "two"}},
		{"multiple hunks with fake headers", "--- a/real\n+++ b/real\n@@ -1 +1 @@\n-a\n+b\n@@ -9,2 +9,2 @@\n--- a/fake\n-rename from fake2\n+++ b/fake\n+copy to fake2", []string{"real"}},
		{"context and no newline marker", "diff --git a/real b/real\n@@ -1,2 +1,2 @@\n shared\n--- a/fake\n\\ No newline at end of file\n+++ b/fake\n\\ No newline at end of file", []string{"real"}},
		{"quoted utf8 bytes", `diff --git "a/caf\303\251.go" "b/caf\303\251.go"`, []string{"café.go"}},
		{"rename paths retain a component", "diff --git a/a/old b/b/new\nrename from a/old\nrename to b/new", []string{"a/old", "b/new"}},
		{"quoted standalone timestamps", "--- \"a/a b\"\t2026-01-01\n+++ \"b/a b\"\t2026-01-01", []string{"a b"}},
		{"binary body is not metadata", "diff --git a/pic b/pic\nGIT binary patch\n--- a/fake\n+++ b/fake\nrename from fake2", []string{"pic"}},
		{"combined followed by conventional", "diff --cc combined\n--- a/fake\n+++ b/fake\ndiff --git a/real b/real", []string{"real"}},
		{"malformed quoted path", `diff --git "a/bad\q" "b/bad\q"`, nil},
		{"unsafe extracted names", "--- a/./unsafe\n+++ b/../unsafe", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := map[string]struct{}{}
			for _, name := range tt.want {
				want[name] = struct{}{}
			}
			if got := reviewer.DiffFiles(tt.diff); !reflect.DeepEqual(got, want) {
				t.Fatalf("DiffFiles() got %#v, want %#v", got, want)
			}
		})
	}
}

func TestIndependentLocationPreservationAndMembership(t *testing.T) {
	diff := "diff --git a/old.go b/new.go\nrename from old.go\nrename to new.go\n--- a/old.go\n+++ b/new.go\n@@ -1 +1 @@\n-old\n+new\n@@ -10 +10 @@\n-old\n+new"
	comments := []reviewer.Comment{
		{File: "old.go", Side: "new", Line: 1, EndLine: 999, Severity: reviewer.SeverityCritical, Message: " cross-hunk and wrong side "},
		{File: "new.go", Side: "old", Line: 9223372036854775807, Severity: reviewer.SeverityLow, Message: "outside hunks"},
		{File: "new.go", Severity: reviewer.SeverityMedium, Message: "file-wide"},
	}
	now := time.Date(2026, 2, 1, 0, 5, 0, 123456789, time.FixedZone("east", 3600))
	p := reviewer.Payload{Summary: " preserve summary\n", Comments: comments}
	got, err := reviewer.Accept("configured", p, reviewer.DiffFiles(diff), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedAt != "2026-01-31T23:05:00.123456789Z" || got.Reviewer != "configured" || got.Summary != p.Summary || !reflect.DeepEqual(got.Comments, comments) {
		t.Fatalf("Accept() got %#v, want preserved content and trusted UTC metadata", got)
	}
	// An accepted result must not share mutable comment storage with the provider.
	p.Comments[0].Message = "mutated after acceptance"
	if got.Comments[0].Message != " cross-hunk and wrong side " {
		t.Fatal("provider mutation changed accepted result")
	}
	for _, file := range []string{"NEW.go", "absent.go"} {
		p := reviewer.Payload{Summary: "s", Comments: []reviewer.Comment{{File: file, Severity: reviewer.SeverityHigh, Message: "m"}}}
		r, err := reviewer.Accept("configured", p, reviewer.DiffFiles(diff), now)
		if err == nil || !strings.Contains(err.Error(), "comments[0].file: not represented") || !reflect.DeepEqual(r, reviewer.Result{}) {
			t.Fatalf("unknown file got %#v, %v; want whole-result membership failure", r, err)
		}
	}
}

func TestIndependentNumericLocationsMatchExactRationals(t *testing.T) {
	// math/big is an independent exact oracle: no float conversion and no copy
	// of production's decimal normalization algorithm.
	coefficients := []string{"0", "1", "10", "101", "0.001", "1.000", "12.50", "9007199254740993", "9223372036854775807", "9223372036854775808", "9223372036854775806.999"}
	for _, coefficient := range coefficients {
		for _, exponent := range []int{-20, -3, -1, 0, 1, 3, 20} {
			token := fmt.Sprintf("%se%+d", coefficient, exponent)
			t.Run(token, func(t *testing.T) {
				value, ok := new(big.Rat).SetString(token)
				if !ok {
					t.Fatal("invalid test oracle input")
				}
				wantValid := value.IsInt() && value.Sign() > 0 && value.Num().IsInt64()
				var p reviewer.Payload
				err := json.Unmarshal([]byte(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":"old","line":`+token+`}]}`), &p)
				if err == nil {
					err = reviewer.ValidatePayload(p)
				}
				if (err == nil) != wantValid {
					t.Fatalf("decode %s got error %v, want valid %v", token, err, wantValid)
				}
				if wantValid && p.Comments[0].Line != value.Num().Int64() {
					t.Fatalf("got %d, want %s", p.Comments[0].Line, value.Num())
				}
			})
		}
	}
}

func TestIndependentTimestampValidation(t *testing.T) {
	for _, stamp := range []string{"2026-02-30T01:00:00Z", "2026-09-12T24:00:00Z", "2026-09-12T12:00:60Z", "2026-09-12t12:00:00Z", "2026-09-12T12:00:00z", "2026-09-12T12:00:00,1Z", "2026-09-12T12:00:00+00:00"} {
		r := reviewer.Result{Reviewer: "r", Summary: "s", CreatedAt: stamp}
		if err := reviewer.ValidateResult(r); err == nil {
			t.Errorf("ValidateResult() accepted %q", stamp)
		}
	}
}

func TestIndependentPathDriveBoundaries(t *testing.T) {
	for _, path := range []string{"A:", "Z:", "a:", "z:", "A:file", "Z:file", "a:file", "z:file", "C:/file"} {
		if reviewer.ValidPath(path) {
			t.Errorf("ValidPath(%q) got true, want false for drive-qualified path", path)
		}
	}
	for _, path := range []string{"@:file", "[:file", "`:file", "{:file", "0:file", "dir/A:file", "a", " a ", "é.go"} {
		if !reviewer.ValidPath(path) {
			t.Errorf("ValidPath(%q) got false, want true for repository-relative name", path)
		}
	}
}
