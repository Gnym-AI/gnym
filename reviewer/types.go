package reviewer

import (
	"encoding/json"
	"fmt"
	"gnym/internal/strictjson"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Config struct{ Name, Provider, Model, Prompt string }
type Request struct {
	Diff   string
	Config Config
}
type Comment struct {
	File     string   `json:"file"`
	Side     string   `json:"side,omitempty"`
	Line     int64    `json:"line,omitempty"`
	EndLine  int64    `json:"end_line,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

func (c *Comment) UnmarshalJSON(data []byte) error {
	var v Comment
	if err := strictjson.Object(data, []string{"file", "severity", "message"}, map[string]any{"file": &v.File, "severity": &v.Severity, "message": &v.Message, "side": &v.Side, "line": &v.Line, "end_line": &v.EndLine}); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	if _, ok := fields["line"]; ok && v.Line <= 0 {
		return fieldError("line", "must be positive")
	}
	if _, ok := fields["end_line"]; ok && v.EndLine <= 0 {
		return fieldError("end_line", "must be positive")
	}
	if _, ok := fields["side"]; ok && v.Side == "" {
		return fieldError("side", "must be old or new")
	}
	*c = v
	return nil
}

// Payload contains only provider-owned review content.
type Payload struct {
	Summary  string    `json:"summary"`
	Comments []Comment `json:"comments"`
}

func (p *Payload) UnmarshalJSON(data []byte) error {
	var comments commentList
	var v Payload
	if err := strictjson.Object(data, []string{"summary", "comments"}, map[string]any{"summary": &v.Summary, "comments": &comments}); err != nil {
		return err
	}
	v.Comments = []Comment(comments)
	*p = v
	return nil
}

type Result struct {
	Reviewer  string    `json:"reviewer"`
	CreatedAt string    `json:"created_at"`
	Summary   string    `json:"summary"`
	Comments  []Comment `json:"comments"`
}

func (r *Result) UnmarshalJSON(data []byte) error {
	var comments commentList
	var v Result
	if err := strictjson.Object(data, []string{"reviewer", "created_at", "summary", "comments"}, map[string]any{"reviewer": &v.Reviewer, "created_at": &v.CreatedAt, "summary": &v.Summary, "comments": &comments}); err != nil {
		return err
	}
	v.Comments = []Comment(comments)
	*r = v
	return nil
}

type commentList []Comment

func (c *commentList) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expected array")
	}
	v := make(commentList, len(raw))
	for i, item := range raw {
		if err := json.Unmarshal(item, &v[i]); err != nil {
			return fmt.Errorf("comment[%d]: %w", i, err)
		}
	}
	*c = v
	return nil
}
func (p Payload) MarshalJSON() ([]byte, error) {
	type wire Payload
	v := wire(p)
	v.Comments = append([]Comment{}, p.Comments...)
	return json.Marshal(v)
}
func (r Result) MarshalJSON() ([]byte, error) {
	type wire Result
	v := wire(r)
	v.Comments = append([]Comment{}, r.Comments...)
	return json.Marshal(v)
}
