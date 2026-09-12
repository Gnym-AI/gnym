package review

import (
	"encoding/json"
	"fmt"
	"gnym/internal/strictjson"
	"gnym/reviewer"
	"strings"
)

type Diff struct{ Content string }
type Status string

const (
	StatusComplete Status = "complete"
	StatusPartial  Status = "partial"
	StatusFailed   Status = "failed"
)

type FailureCode string

const (
	FailureInvalidOutput FailureCode = "invalid_output"
	FailureReviewerError FailureCode = "reviewer_error"
)

type Failure struct {
	Reviewer string      `json:"reviewer"`
	Code     FailureCode `json:"code"`
	Message  string      `json:"message"`
}

func (f *Failure) UnmarshalJSON(data []byte) error {
	var v Failure
	if err := strictjson.Object(data, []string{"reviewer", "code", "message"}, map[string]any{"reviewer": &v.Reviewer, "code": &v.Code, "message": &v.Message}); err != nil {
		return err
	}
	*f = v
	return nil
}

// Run is the versioned aggregate. Results retains its Go name while its wire
// representation uses reviews. Status describes reviewer outcomes, not delivery.
type Run struct {
	SchemaVersion string            `json:"schema_version"`
	Status        Status            `json:"status"`
	Results       []reviewer.Result `json:"reviews"`
	Failures      []Failure         `json:"failures"`
}

func (r *Run) UnmarshalJSON(data []byte) error {
	var v Run
	if err := strictjson.Object(data, []string{"schema_version", "status", "reviews", "failures"}, map[string]any{"schema_version": &v.SchemaVersion, "status": &v.Status, "reviews": &v.Results, "failures": &v.Failures}); err != nil {
		return err
	}
	*r = v
	return nil
}

// MarshalJSON keeps all required collections as arrays, including zero findings.
func (r Run) MarshalJSON() ([]byte, error) {
	type wire Run
	v := wire(r)
	v.Results = append([]reviewer.Result{}, r.Results...)
	v.Failures = append([]Failure{}, r.Failures...)
	for i := range v.Results {
		v.Results[i].Comments = append([]reviewer.Comment{}, v.Results[i].Comments...)
	}
	return json.Marshal(v)
}

// Validate checks aggregate semantics without filesystem access or diff parsing.
func (r Run) Validate() error {
	if r.SchemaVersion != "1" {
		return fmt.Errorf("schema_version: unsupported version")
	}
	valid := false
	switch r.Status {
	case StatusComplete:
		valid = len(r.Results) > 0 && len(r.Failures) == 0
	case StatusPartial:
		valid = len(r.Results) > 0 && len(r.Failures) > 0
	case StatusFailed:
		valid = len(r.Results) == 0 && len(r.Failures) > 0
	}
	if !valid {
		return fmt.Errorf("status: inconsistent reviewer outcome cardinalities")
	}
	seen := map[string]bool{}
	identity := func(name string) error {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("reviewer: must be nonblank")
		}
		if seen[name] {
			return fmt.Errorf("reviewer %q: duplicate identity", name)
		}
		seen[name] = true
		return nil
	}
	for i, result := range r.Results {
		if err := identity(result.Reviewer); err != nil {
			return err
		}
		if err := reviewer.ValidateResult(result); err != nil {
			return fmt.Errorf("reviews[%d] reviewer %q: %w", i, result.Reviewer, err)
		}
	}
	for i, failure := range r.Failures {
		if err := identity(failure.Reviewer); err != nil {
			return err
		}
		if failure.Code != FailureInvalidOutput && failure.Code != FailureReviewerError {
			return fmt.Errorf("failures[%d] reviewer %q: code: unsupported value", i, failure.Reviewer)
		}
		if strings.TrimSpace(failure.Message) == "" {
			return fmt.Errorf("failures[%d] reviewer %q: message: must be nonblank", i, failure.Reviewer)
		}
	}
	return nil
}
