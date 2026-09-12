package reviewer

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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
	for _, r := range path {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// ValidatePayload validates content structure; membership is checked by Accept.
func ValidatePayload(p Payload) error {
	if strings.TrimSpace(p.Summary) == "" {
		return fieldError("summary", "must be nonblank")
	}
	for i, c := range p.Comments {
		prefix := fmt.Sprintf("comments[%d]", i)
		if !ValidPath(c.File) {
			return fieldError(prefix+".file", "invalid repository-relative path")
		}
		if strings.TrimSpace(c.Message) == "" {
			return fieldError(prefix+".message", "must be nonblank")
		}
		switch c.Severity {
		case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		default:
			return fieldError(prefix+".severity", "unsupported value")
		}
		if c.Side == "" && c.Line == 0 && c.EndLine == 0 {
			continue
		}
		if c.Side != "old" && c.Side != "new" {
			return fieldError(prefix+".side", "must be old or new with a line")
		}
		if c.Line <= 0 {
			return fieldError(prefix+".line", "must be positive with a side")
		}
		if c.EndLine != 0 && c.EndLine < c.Line {
			return fieldError(prefix+".end_line", "must be at least line")
		}
	}
	return nil
}

// Accept validates membership and supplies trusted identity and UTC acceptance time.
func Accept(name string, p Payload, files map[string]struct{}, now time.Time) (Result, error) {
	fail := func(err error) (Result, error) { return Result{}, fmt.Errorf("reviewer %q: %w", name, err) }
	if strings.TrimSpace(name) == "" {
		return fail(fieldError("reviewer", "must be nonblank"))
	}
	if err := ValidatePayload(p); err != nil {
		return fail(err)
	}
	for i, c := range p.Comments {
		if _, ok := files[c.File]; !ok {
			return fail(fieldError(fmt.Sprintf("comments[%d].file", i), "not represented in diff metadata"))
		}
	}
	comments := append([]Comment{}, p.Comments...)
	r := Result{Reviewer: name, CreatedAt: now.UTC().Format(time.RFC3339Nano), Summary: p.Summary, Comments: comments}
	if err := ValidateResult(r); err != nil {
		return fail(err)
	}
	return r, nil
}

var utcTimestamp = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)

// ValidateResult checks an accepted result without repeating diff membership.
func ValidateResult(r Result) error {
	if strings.TrimSpace(r.Reviewer) == "" {
		return fieldError("reviewer", "must be nonblank")
	}
	if !utcTimestamp.MatchString(r.CreatedAt) {
		return fieldError("created_at", "must be UTC RFC3339")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err != nil {
		return fieldError("created_at", "must be UTC RFC3339")
	}
	return ValidatePayload(Payload{Summary: r.Summary, Comments: r.Comments})
}
