package reviewer

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var utcTimestamp = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)

// Accept validates membership and adds trusted reviewer identity and acceptance time.
func Accept(name string, payload Payload, files map[string]struct{}, now time.Time) (Result, error) {
	fail := func(err error) (Result, error) { return Result{}, reviewerError(name, err) }
	if err := require("reviewer", "must be nonblank", strings.TrimSpace(name) != ""); err != nil {
		return fail(err)
	}
	if err := ValidatePayload(payload); err != nil {
		return fail(err)
	}
	for index, comment := range payload.Comments {
		if _, ok := files[comment.File]; !ok {
			return fail(fieldError(fmt.Sprintf("comments[%d].file", index), "not represented in diff metadata"))
		}
	}
	result := Result{
		Reviewer: name, CreatedAt: now.UTC().Format(time.RFC3339Nano),
		Summary: payload.Summary, Comments: append([]Comment{}, payload.Comments...),
	}
	if err := ValidateResult(result); err != nil {
		return fail(err)
	}
	return result, nil
}

func reviewerError(name string, err error) error {
	characters := []rune(name)
	if len(characters) > 40 {
		name = string(characters[:40]) + "..."
	}
	return fmt.Errorf("reviewer %q: %w", name, err)
}

// ValidateResult checks accepted-result structure without repeating membership.
func ValidateResult(result Result) error {
	return validate(result, validateResultReviewer, validateResultTime, validateResultPayload)
}

func validateResultReviewer(result Result) error {
	return require("reviewer", "must be nonblank", strings.TrimSpace(result.Reviewer) != "")
}

func validateResultTime(result Result) error {
	if !utcTimestamp.MatchString(result.CreatedAt) {
		return fieldError("created_at", "must be UTC RFC3339")
	}
	if _, err := time.Parse(time.RFC3339Nano, result.CreatedAt); err != nil {
		return fieldError("created_at", "must be UTC RFC3339")
	}
	return nil
}

func validateResultPayload(result Result) error {
	return ValidatePayload(Payload{Summary: result.Summary, Comments: result.Comments})
}
