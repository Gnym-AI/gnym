package review

import (
	"encoding/json"
	"fmt"
	"gnym/internal/strictjson"
	"gnym/reviewer"
	"strings"
)

type Diff struct {
	Content string
}

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
	var decoded Failure
	fields := map[string]any{
		"reviewer": &decoded.Reviewer, "code": &decoded.Code, "message": &decoded.Message,
	}
	if err := strictjson.Object(data, []string{"reviewer", "code", "message"}, fields); err != nil {
		return err
	}
	*f = decoded
	return nil
}

type Run struct {
	SchemaVersion string            `json:"schema_version"`
	Status        Status            `json:"status"`
	Results       []reviewer.Result `json:"reviews"`
	Failures      []Failure         `json:"failures"`
}

func (r *Run) UnmarshalJSON(data []byte) error {
	var decoded Run
	var reviews []resultJSON
	fields := map[string]any{
		"schema_version": &decoded.SchemaVersion, "status": &decoded.Status,
		"reviews": &reviews, "failures": &decoded.Failures,
	}
	if err := strictjson.Object(data, []string{"schema_version", "status", "reviews", "failures"}, fields); err != nil {
		return err
	}
	decoded.Results = make([]reviewer.Result, len(reviews))
	for index, result := range reviews {
		decoded.Results[index] = result.result()
	}
	*r = decoded
	return nil
}

// MarshalJSON keeps required collections as arrays without mutating the caller.
func (r Run) MarshalJSON() ([]byte, error) {
	reviews := make([]resultJSON, len(r.Results))
	for index, result := range r.Results {
		reviews[index] = newResultJSON(result)
	}
	return json.Marshal(struct {
		SchemaVersion string       `json:"schema_version"`
		Status        Status       `json:"status"`
		Reviews       []resultJSON `json:"reviews"`
		Failures      []Failure    `json:"failures"`
	}{r.SchemaVersion, r.Status, reviews, append([]Failure{}, r.Failures...)})
}

type resultJSON struct {
	Reviewer  string             `json:"reviewer"`
	CreatedAt string             `json:"created_at"`
	Summary   string             `json:"summary"`
	Comments  []reviewer.Comment `json:"comments"`
}

func (r *resultJSON) UnmarshalJSON(data []byte) error {
	var decoded resultJSON
	fields := map[string]any{
		"reviewer": &decoded.Reviewer, "created_at": &decoded.CreatedAt,
		"summary": &decoded.Summary, "comments": &decoded.Comments,
	}
	if err := strictjson.Object(data, []string{"reviewer", "created_at", "summary", "comments"}, fields); err != nil {
		return err
	}
	*r = decoded
	return nil
}

func newResultJSON(result reviewer.Result) resultJSON {
	return resultJSON{
		Reviewer: result.Reviewer, CreatedAt: result.CreatedAt, Summary: result.Summary,
		Comments: append([]reviewer.Comment{}, result.Comments...),
	}
}

func (r resultJSON) result() reviewer.Result {
	return reviewer.Result{
		Reviewer: r.Reviewer, CreatedAt: r.CreatedAt, Summary: r.Summary,
		Comments: append([]reviewer.Comment{}, r.Comments...),
	}
}

// Validate checks aggregate semantics without repeating diff membership.
func (r Run) Validate() error {
	for _, rule := range []func(Run) error{
		validateVersion, validateOutcomeShape, validateResults, validateFailures, validateIdentities,
	} {
		if err := rule(r); err != nil {
			return err
		}
	}
	return nil
}

func validateVersion(run Run) error {
	if run.SchemaVersion != "1" {
		return fmt.Errorf("schema_version: unsupported version")
	}
	return nil
}

func validateOutcomeShape(run Run) error {
	valid := run.Status == StatusComplete && len(run.Results) > 0 && len(run.Failures) == 0 ||
		run.Status == StatusPartial && len(run.Results) > 0 && len(run.Failures) > 0 ||
		run.Status == StatusFailed && len(run.Results) == 0 && len(run.Failures) > 0
	if !valid {
		return fmt.Errorf("status: inconsistent reviewer outcome cardinalities")
	}
	return nil
}

func validateResults(run Run) error {
	for index, result := range run.Results {
		if err := reviewer.ValidateResult(result); err != nil {
			return fmt.Errorf("reviews[%d]: %w", index, err)
		}
	}
	return nil
}

func validateFailures(run Run) error {
	for index, failure := range run.Failures {
		if strings.TrimSpace(failure.Reviewer) == "" {
			return fmt.Errorf("failures[%d].reviewer: must be nonblank", index)
		}
		if failure.Code != FailureInvalidOutput && failure.Code != FailureReviewerError {
			return fmt.Errorf("failures[%d].code: unsupported value", index)
		}
		if strings.TrimSpace(failure.Message) == "" {
			return fmt.Errorf("failures[%d].message: must be nonblank", index)
		}
	}
	return nil
}

func validateIdentities(run Run) error {
	seen := make(map[string]struct{}, len(run.Results)+len(run.Failures))
	for index, result := range run.Results {
		if _, duplicate := seen[result.Reviewer]; duplicate {
			return fmt.Errorf("reviews[%d].reviewer: duplicate identity", index)
		}
		seen[result.Reviewer] = struct{}{}
	}
	for index, failure := range run.Failures {
		if _, duplicate := seen[failure.Reviewer]; duplicate {
			return fmt.Errorf("failures[%d].reviewer: duplicate identity", index)
		}
		seen[failure.Reviewer] = struct{}{}
	}
	return nil
}
