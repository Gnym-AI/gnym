package reviewer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func fieldError(field, rule string) error { return fmt.Errorf("%s: %s", field, rule) }

func require(field, rule string, valid bool) error {
	if !valid {
		return fieldError(field, rule)
	}
	return nil
}

func validate[T any](value T, rules ...func(T) error) error {
	for _, rule := range rules {
		if err := rule(value); err != nil {
			return err
		}
	}
	return nil
}

// ValidPath checks a repository-relative path without filesystem access.
func ValidPath(path string) bool {
	if path == "" || !utf8.ValidString(path) || strings.Contains(path, "\\") {
		return false
	}
	if len(path) >= 2 && path[1] == ':' && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) {
		return false
	}
	for _, character := range path {
		if unicode.IsControl(character) {
			return false
		}
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

// ValidatePayload validates provider-owned review content.
func ValidatePayload(p Payload) error { return validate(p, validateSummary, validateComments) }

func validateSummary(p Payload) error {
	return require("summary", "must be nonblank", strings.TrimSpace(p.Summary) != "")
}
func validateComments(payload Payload) error {
	for index, comment := range payload.Comments {
		if err := validateComment(comment); err != nil {
			return fmt.Errorf("comments[%d].%w", index, err)
		}
	}
	return nil
}

func validateComment(comment Comment) error {
	return validate(comment, validateCommentFile, validateCommentMessage, validateCommentSeverity, validateCommentLocation)
}

func validateCommentFile(comment Comment) error {
	return require("file", "invalid repository-relative path", ValidPath(comment.File))
}

func validateCommentMessage(c Comment) error {
	return require("message", "must be nonblank", strings.TrimSpace(c.Message) != "")
}
func validateCommentSeverity(comment Comment) error {
	return require("severity", "unsupported value",
		comment.Severity == SeverityLow || comment.Severity == SeverityMedium ||
			comment.Severity == SeverityHigh || comment.Severity == SeverityCritical)
}

func validateCommentLocation(comment Comment) error {
	if comment.Side == "" && comment.Line == 0 && comment.EndLine == 0 {
		return nil
	}
	if comment.Side != "new" && comment.Side != "old" {
		return fieldError("side", "must be old or new with a line")
	}
	if comment.Line <= 0 {
		return fieldError("line", "must be positive with a side")
	}
	if comment.EndLine < 0 || comment.EndLine > 0 && comment.EndLine < comment.Line {
		return fieldError("end_line", "must be at least line")
	}
	return nil
}
