package review

import (
	"encoding/json"
	"gnym/reviewer"
	"strings"
	"testing"
)

func validResult() reviewer.Result {
	return reviewer.Result{Reviewer: "r", CreatedAt: "2026-09-12T12:00:00Z", Summary: "s", Comments: []reviewer.Comment{}}
}
func TestRunValidation(t *testing.T) {
	good := func() Run {
		return Run{SchemaVersion: "1", Status: StatusComplete, Results: []reviewer.Result{validResult()}}
	}
	failure := Failure{Reviewer: "f", Code: FailureReviewerError, Message: "failed"}
	for _, r := range []Run{good(), {SchemaVersion: "1", Status: StatusPartial, Results: []reviewer.Result{validResult()}, Failures: []Failure{failure}}, {SchemaVersion: "1", Status: StatusFailed, Failures: []Failure{failure}}} {
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		edit func(*Run)
	}{
		{"version", func(r *Run) { r.SchemaVersion = "2" }}, {"missing version", func(r *Run) { r.SchemaVersion = "" }}, {"empty", func(r *Run) { r.Results = nil }},
		{"partial without failure", func(r *Run) { r.Status = StatusPartial }}, {"failed with review", func(r *Run) { r.Status = StatusFailed; r.Failures = []Failure{failure} }},
		{"complete with failure", func(r *Run) { r.Failures = []Failure{failure} }}, {"unknown status", func(r *Run) { r.Status = "other" }},
		{"duplicate reviews", func(r *Run) { r.Results = append(r.Results, validResult()) }}, {"blank reviewer", func(r *Run) { r.Results[0].Reviewer = " " }},
		{"duplicate across", func(r *Run) {
			r.Status = StatusPartial
			r.Failures = []Failure{{Reviewer: "r", Code: FailureInvalidOutput, Message: "m"}}
		}},
		{"duplicate failures", func(r *Run) { r.Status = StatusFailed; r.Results = nil; r.Failures = []Failure{failure, failure} }},
		{"bad code", func(r *Run) { r.Status = StatusPartial; f := failure; f.Code = "other"; r.Failures = []Failure{f} }},
		{"blank failure", func(r *Run) { r.Status = StatusPartial; f := failure; f.Message = "\n"; r.Failures = []Failure{f} }},
		{"blank failure identity", func(r *Run) { r.Status = StatusPartial; f := failure; f.Reviewer = ""; r.Failures = []Failure{f} }},
		{"offset timestamp", func(r *Run) { r.Results[0].CreatedAt = "2026-09-12T12:00:00+00:00" }}, {"malformed timestamp", func(r *Run) { r.Results[0].CreatedAt = "badZ" }},
		{"blank summary", func(r *Run) { r.Results[0].Summary = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := good()
			tt.edit(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("accepted invalid run")
			}
		})
	}
}
func TestRunJSONBoundary(t *testing.T) {
	result := `{"reviewer":"r","created_at":"2026-09-12T12:00:00Z","summary":"s","comments":[]}`
	valid := `{"schema_version":"1","status":"complete","reviews":[` + result + `],"failures":[]}`
	for _, raw := range []string{`null`, `{}`, strings.Replace(valid, `"failures":[]`, `"failures":null`, 1), strings.Replace(valid, `"comments":[]`, `"comments":null`, 1), strings.Replace(valid, `"summary":"s",`, "", 1), strings.Replace(valid, `"reviewer":"r",`, "", 1), strings.Replace(valid, `"created_at":"2026-09-12T12:00:00Z",`, "", 1), strings.Replace(valid, `"status":"complete",`, "", 1), strings.Replace(valid, `"failures":[]`, `"failures":[],"extra":1`, 1), strings.Replace(valid, `"summary":"s"`, `"summary":"s","extra":1`, 1), strings.Replace(valid, `"failures":[]`, `"failures":[{"reviewer":"f","message":"m"}]`, 1), strings.Replace(valid, `"reviews":[`+result+`]`, `"reviews":[null]`, 1)} {
		var r Run
		if err := json.Unmarshal([]byte(raw), &r); err == nil {
			t.Errorf("accepted malformed JSON %s", raw)
		}
	}
	var r Run
	if err := json.Unmarshal([]byte(valid), &r); err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Results[0].Comments = nil
	r.Failures = nil
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") || !strings.Contains(string(b), `"failures":[]`) || !strings.Contains(string(b), `"comments":[]`) {
		t.Fatalf("required arrays: %s", b)
	}
	if r.Results[0].Comments != nil {
		t.Fatal("marshaling mutated caller")
	}
}
