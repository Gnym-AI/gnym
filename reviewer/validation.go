package reviewer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func fieldError(field, rule string) error { return fmt.Errorf("%s: %s", field, rule) }

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
func ValidatePayload(payload Payload) error {
	if strings.TrimSpace(payload.Summary) == "" {
		return fieldError("summary", "must be nonblank")
	}
	for index, comment := range payload.Comments {
		prefix := fmt.Sprintf("comments[%d]", index)
		if !ValidPath(comment.File) {
			return fieldError(prefix+".file", "invalid repository-relative path")
		}
		if strings.TrimSpace(comment.Message) == "" {
			return fieldError(prefix+".message", "must be nonblank")
		}
		switch comment.Severity {
		case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		default:
			return fieldError(prefix+".severity", "unsupported value")
		}
		if comment.Side == "" && comment.Line == 0 && comment.EndLine == 0 {
			continue
		}
		if comment.Side != "new" && comment.Side != "old" {
			return fieldError(prefix+".side", "must be old or new with a line")
		}
		if comment.Line <= 0 {
			return fieldError(prefix+".line", "must be positive with a side")
		}
		if comment.EndLine < 0 || comment.EndLine > 0 && comment.EndLine < comment.Line {
			return fieldError(prefix+".end_line", "must be at least line")
		}
	}
	return nil
}
