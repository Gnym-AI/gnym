package reviewer_test

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"gnym/reviewer"
)

func TestProviderNumbersAgainstIndependentInt64Oracle(t *testing.T) {
	tests := []struct {
		token string
		want  int64
		valid bool
	}{
		{token: "1", want: 1, valid: true},
		{token: "1.000", want: 1, valid: true},
		{token: "9007199254740993.0", want: 9007199254740993, valid: true},
		{token: "9223372036854775807", want: math.MaxInt64, valid: true},
		{token: "9.223372036854775807e18", want: math.MaxInt64, valid: true},
		{token: "0"},
		{token: "-1"},
		{token: "1.0001"},
		{token: "9.223372036854775808e18"},
		{token: "1e100000"},
	}
	for _, test := range tests {
		t.Run(test.token, func(t *testing.T) {
			source := fmt.Sprintf(`{"summary":"ok","comments":[{"file":"a.go","side":"new","line":%s,"end_line":%s,"severity":"high","message":"note"}]}`, test.token, test.token)
			var payload reviewer.Payload
			err := json.Unmarshal([]byte(source), &payload)
			if err == nil {
				err = reviewer.ValidatePayload(payload)
			}
			if !test.valid {
				if err == nil {
					t.Fatalf("token %s accepted", test.token)
				}
				return
			}
			if err != nil {
				t.Fatalf("token %s rejected: %v", test.token, err)
			}
			comment := payload.Comments[0]
			if comment.Line != test.want || comment.EndLine != test.want {
				t.Fatalf("decoded location = (%d, %d), want (%d, %d)", comment.Line, comment.EndLine, test.want, test.want)
			}
		})
	}
}

func TestDiffMembershipBoundariesAndAdvisoryLocations(t *testing.T) {
	diff := strings.Join([]string{
		`diff --git "a/old\040name.go" "b/new\040name.go"`,
		`similarity index 92%`,
		`rename from old name.go`,
		`rename to new name.go`,
		`@@ -2,2 +40,2 @@`,
		` --- a/body-decoy.go`,
		` +++ b/body-decoy.go`,
		`diff --git a/image.png b/image.png`,
		`GIT binary patch`,
		`--- a/binary-decoy.go`,
		`+++ b/binary-decoy.go`,
		`diff --git a/source.go b/copied.go`,
		`copy from source.go`,
		`copy to copied.go`,
	}, "\n")
	wantFiles := map[string]struct{}{
		"old name.go": {}, "new name.go": {}, "image.png": {},
		"source.go": {}, "copied.go": {},
	}
	files := reviewer.DiffFiles(diff)
	if !reflect.DeepEqual(files, wantFiles) {
		t.Fatalf("DiffFiles() = %#v, want %#v", files, wantFiles)
	}

	comments := []reviewer.Comment{
		{File: "old name.go", Side: "new", Line: math.MaxInt64, Severity: reviewer.SeverityCritical, Message: "wrong side and outside hunk"},
		{File: "copied.go", Side: "old", Line: 1, EndLine: 9000, Severity: reviewer.SeverityMedium, Message: "cross-hunk range"},
		{File: "image.png", Severity: reviewer.SeverityLow, Message: "file-wide"},
	}
	now := time.Date(2026, 10, 19, 14, 15, 16, 17, time.FixedZone("offset", -4*60*60))
	got, err := reviewer.Accept("configured-reviewer", reviewer.Payload{Summary: " keep ", Comments: comments}, files, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reviewer != "configured-reviewer" || got.CreatedAt != "2026-10-19T18:15:16.000000017Z" {
		t.Fatalf("trusted metadata = (%q, %q)", got.Reviewer, got.CreatedAt)
	}
	if got.Summary != " keep " || !reflect.DeepEqual(got.Comments, comments) {
		t.Fatalf("accepted content changed: %#v", got)
	}

	for _, file := range []string{"body-decoy.go", "binary-decoy.go", "NEW NAME.GO"} {
		payload := reviewer.Payload{Summary: "ok", Comments: []reviewer.Comment{{File: file, Severity: reviewer.SeverityLow, Message: "note"}}}
		_, err := reviewer.Accept("configured-reviewer", payload, files, now)
		if err == nil || !strings.Contains(err.Error(), "not represented in diff metadata") {
			t.Fatalf("file %q error = %v", file, err)
		}
	}
}

func TestArbitraryInputAllowsOnlyZeroFindings(t *testing.T) {
	for _, diff := range []string{"", "ordinary text\n--- a/unpaired.go", "diff --cc combined.go\n+++ b/unsupported.go", `diff --git "a/unclosed b/unsupported.go`} {
		files := reviewer.DiffFiles(diff)
		result, err := reviewer.Accept("r", reviewer.Payload{Summary: "no findings"}, files, time.Unix(0, 0))
		if err != nil || result.Comments == nil {
			t.Fatalf("zero findings for %q = %#v, %v", diff, result, err)
		}
		payload := reviewer.Payload{Summary: "finding", Comments: []reviewer.Comment{{File: "unsupported.go", Severity: reviewer.SeverityHigh, Message: "note"}}}
		if _, err := reviewer.Accept("r", payload, files, time.Unix(0, 0)); err == nil {
			t.Fatalf("unestablished file accepted for %q", diff)
		}
	}
}

func TestDriveQualifiedPathBoundaries(t *testing.T) {
	tests := map[string]bool{
		"A:": false, "z:": false,
		"A:/file.go": false, "Z:/file.go": false,
		"a:/file.go": false, "z:/file.go": false,
		"@:/file.go": true, "[:/file.go": true,
		"`:/file.go": true, "{:/file.go": true,
	}
	for path, want := range tests {
		if got := reviewer.ValidPath(path); got != want {
			t.Errorf("ValidPath(%q) = %v, want %v", path, got, want)
		}
	}
}
