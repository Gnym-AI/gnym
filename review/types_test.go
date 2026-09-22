package review

import (
	"encoding/json"
	"gnym/reviewer"
	"reflect"
	"strings"
	"testing"
)

func acceptedResult(name string) reviewer.Result {
	return reviewer.Result{
		Reviewer: name, CreatedAt: "2026-09-21T12:00:00.123Z", Summary: "accepted",
		Comments: []reviewer.Comment{},
	}
}

func reviewerFailure(name string) Failure {
	return Failure{Reviewer: name, Code: FailureReviewerError, Message: "provider failed"}
}

func TestRunValidateAcceptsVersionOneOutcomeShapes(t *testing.T) {
	tests := []Run{
		{SchemaVersion: "1", Status: StatusComplete, Results: []reviewer.Result{acceptedResult("first"), acceptedResult("second")}},
		{SchemaVersion: "1", Status: StatusPartial, Results: []reviewer.Result{acceptedResult("first")}, Failures: []Failure{reviewerFailure("second")}},
		{SchemaVersion: "1", Status: StatusFailed, Failures: []Failure{{Reviewer: "first", Code: FailureInvalidOutput, Message: "invalid payload"}}},
	}
	for _, run := range tests {
		if err := run.Validate(); err != nil {
			t.Errorf("Validate(%s) = %v", run.Status, err)
		}
	}
}

func TestRunValidateRejectsInvalidAggregateSemantics(t *testing.T) {
	valid := func() Run {
		return Run{SchemaVersion: "1", Status: StatusComplete, Results: []reviewer.Result{acceptedResult("first")}}
	}
	failure := reviewerFailure("second")
	tests := []struct {
		name string
		edit func(*Run)
		want string
	}{
		{"missing version", func(r *Run) { r.SchemaVersion = "" }, "schema_version"},
		{"unsupported version", func(r *Run) { r.SchemaVersion = "2" }, "schema_version"},
		{"unknown status", func(r *Run) { r.Status = "unknown" }, "status"},
		{"complete empty", func(r *Run) { r.Results = nil }, "status"},
		{"complete with failure", func(r *Run) { r.Failures = []Failure{failure} }, "status"},
		{"partial without failure", func(r *Run) { r.Status = StatusPartial }, "status"},
		{"partial without review", func(r *Run) { r.Status = StatusPartial; r.Results = nil; r.Failures = []Failure{failure} }, "status"},
		{"failed with review", func(r *Run) { r.Status = StatusFailed; r.Failures = []Failure{failure} }, "status"},
		{"failed empty", func(r *Run) { r.Status = StatusFailed; r.Results = nil }, "status"},
		{"invalid nested result", func(r *Run) { r.Results[0].CreatedAt = "2026-09-21T12:00:00+00:00" }, "reviews[0]: created_at"},
		{"blank failure reviewer", func(r *Run) { r.Status = StatusPartial; failure.Reviewer = " \n"; r.Failures = []Failure{failure} }, "failures[0].reviewer"},
		{"unknown failure code", func(r *Run) { r.Status = StatusPartial; failure.Code = "timeout"; r.Failures = []Failure{failure} }, "failures[0].code"},
		{"blank failure message", func(r *Run) { r.Status = StatusPartial; failure.Message = "\t"; r.Failures = []Failure{failure} }, "failures[0].message"},
		{"duplicate reviews", func(r *Run) { r.Results = append(r.Results, acceptedResult("first")) }, "reviews[1].reviewer"},
		{"duplicate failures", func(r *Run) { r.Status = StatusFailed; r.Results = nil; r.Failures = []Failure{failure, failure} }, "failures[1].reviewer"},
		{"duplicate across outcomes", func(r *Run) { r.Status = StatusPartial; failure.Reviewer = "first"; r.Failures = []Failure{failure} }, "failures[0].reviewer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := valid()
			failure = reviewerFailure("second")
			test.edit(&run)
			if err := run.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunJSONBoundaryIsStrictAndNormalizesArrays(t *testing.T) {
	result := `{"reviewer":"first","created_at":"2026-09-21T12:00:00Z","summary":"accepted","comments":[]}`
	valid := `{"schema_version":"1","status":"complete","reviews":[` + result + `],"failures":[]}`
	invalid := []string{
		`null`, `{}`,
		strings.Replace(valid, `"schema_version":"1",`, "", 1),
		strings.Replace(valid, `"status":"complete",`, "", 1),
		strings.Replace(valid, `"reviews":[`+result+`],`, "", 1),
		strings.Replace(valid, `,"failures":[]`, "", 1),
		strings.Replace(valid, `"reviews":[`+result+`]`, `"reviews":null`, 1),
		strings.Replace(valid, `"failures":[]`, `"failures":null`, 1),
		strings.Replace(valid, `"comments":[]`, `"comments":null`, 1),
		strings.Replace(valid, `"failures":[]`, `"failures":[],"extra":true`, 1),
		strings.Replace(valid, `"reviewer":"first",`, "", 1),
		strings.Replace(valid, `"created_at":"2026-09-21T12:00:00Z",`, "", 1),
		strings.Replace(valid, `"summary":"accepted",`, "", 1),
		strings.Replace(valid, `,"comments":[]`, "", 1),
		strings.Replace(valid, `"summary":"accepted"`, `"summary":"accepted","extra":true`, 1),
		strings.Replace(valid, `"reviews":[`+result+`]`, `"reviews":[null]`, 1),
		`{"schema_version":"1","status":"failed","reviews":[],"failures":[{"reviewer":"first","code":"reviewer_error"}]}`,
		`{"schema_version":"1","status":"failed","reviews":[],"failures":[{"reviewer":"first","code":"reviewer_error","message":"failed","extra":true}]}`,
		`{"schema_version":"1","status":"failed","reviews":[],"failures":[null]}`,
	}
	for _, raw := range invalid {
		var run Run
		if err := json.Unmarshal([]byte(raw), &run); err == nil {
			t.Errorf("accepted malformed JSON: %s", raw)
		}
	}

	var decoded Run
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := []string{decoded.Results[0].Reviewer}; !reflect.DeepEqual(got, []string{"first"}) {
		t.Fatalf("review order = %v", got)
	}

	decoded.Results[0].Comments = nil
	decoded.Failures = nil
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "null") || !strings.Contains(string(encoded), `"reviews":[`) || !strings.Contains(string(encoded), `"comments":[]`) || !strings.Contains(string(encoded), `"failures":[]`) {
		t.Fatalf("required arrays were not normalized: %s", encoded)
	}
	if decoded.Results[0].Comments != nil || decoded.Failures != nil {
		t.Fatal("marshaling mutated the caller")
	}
}

func TestRunPreservesReviewAndFailureOrder(t *testing.T) {
	run := Run{
		SchemaVersion: "1", Status: StatusPartial,
		Results:  []reviewer.Result{acceptedResult("second"), acceptedResult("first")},
		Failures: []Failure{reviewerFailure("fourth"), reviewerFailure("third")},
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Run
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, run) {
		t.Fatalf("round trip reordered run: %#v", decoded)
	}
}
