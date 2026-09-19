package reviewer

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAcceptAddsTrustedMetadataAndPreservesComments(t *testing.T) {
	diff := "diff --git a/old.go b/new.go\nrename from old.go\nrename to new.go\n@@ -1 +1 @@\n-old\n+new"
	comments := []Comment{
		{File: "old.go", Side: "new", Line: 1, EndLine: 999, Severity: SeverityHigh, Message: " cross-hunk and wrong side "},
		{File: "new.go", Side: "old", Line: math.MaxInt64, Severity: SeverityLow, Message: "outside hunks"},
		{File: "new.go", Severity: SeverityMedium, Message: "file-wide"},
	}
	payload := Payload{Summary: " preserve summary\n", Comments: comments}
	now := time.Date(2026, 9, 12, 12, 0, 0, 123, time.FixedZone("other", 3600))
	got, err := Accept("configured", payload, DiffFiles(diff), now)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Reviewer: "configured", CreatedAt: "2026-09-12T11:00:00.000000123Z", Summary: payload.Summary, Comments: comments}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Accept() = %#v, want %#v", got, want)
	}
	payload.Comments[0].Message = "mutated"
	if got.Comments[0].Message != " cross-hunk and wrong side " {
		t.Fatal("provider mutation changed accepted result")
	}

	for _, input := range []string{"", "arbitrary text"} {
		result, err := Accept("configured", Payload{Summary: "nothing"}, DiffFiles(input), now)
		if err != nil || result.Comments == nil {
			t.Fatalf("zero findings for %q = %#v, %v", input, result, err)
		}
	}
	for _, file := range []string{"NEW.go", "absent.go"} {
		invalid := Payload{Summary: "s", Comments: []Comment{{File: file, Severity: SeverityCritical, Message: "m"}}}
		result, err := Accept("configured", invalid, DiffFiles(diff), now)
		if err == nil || !strings.Contains(err.Error(), "comments[0].file: not represented in diff metadata") || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("unknown file result = %#v, error = %v", result, err)
		}
	}
}

func TestAcceptedResultValidation(t *testing.T) {
	valid := Result{Reviewer: "r", CreatedAt: "2026-09-12T11:00:00.123Z", Summary: "s"}
	if err := ValidateResult(valid); err != nil {
		t.Fatal(err)
	}

	invalidResults := []Result{
		{CreatedAt: valid.CreatedAt, Summary: "s"},
		{Reviewer: "r", CreatedAt: "2026-02-30T01:00:00Z", Summary: "s"},
		{Reviewer: "r", CreatedAt: "2026-09-12T11:00:00+00:00", Summary: "s"},
		{Reviewer: "r", CreatedAt: valid.CreatedAt, Summary: " \n"},
		{Reviewer: "r", CreatedAt: valid.CreatedAt, Summary: "s", Comments: []Comment{{File: "a", Severity: SeverityLow}}},
	}
	for _, result := range invalidResults {
		if err := ValidateResult(result); err == nil {
			t.Errorf("ValidateResult() accepted %#v", result)
		}
	}
}

func TestAcceptBoundsReviewerDiagnostic(t *testing.T) {
	name := strings.Repeat("x", 1000)
	_, err := Accept(name, Payload{Summary: " "}, nil, time.Now())
	if err == nil || len(err.Error()) > 200 || strings.Contains(err.Error(), name) {
		t.Fatalf("unbounded diagnostic: %v", err)
	}
}
