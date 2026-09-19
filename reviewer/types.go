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

	// Legacy values remain until the runtime migration in GNY-21.
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Config struct {
	Name     string
	Provider string
	Model    string
	Prompt   string
}

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
	var decoded Comment
	var line, endLine positiveInt64
	fields := map[string]any{
		"file": &decoded.File, "side": &decoded.Side, "line": &line,
		"end_line": &endLine, "severity": &decoded.Severity, "message": &decoded.Message,
	}
	if err := strictjson.Object(data, []string{"file", "severity", "message"}, fields); err != nil {
		return err
	}
	decoded.Line, decoded.EndLine = int64(line), int64(endLine)
	*c = decoded
	return nil
}

// Payload contains only provider-owned review content.
type Payload struct {
	Summary  string    `json:"summary"`
	Comments []Comment `json:"comments"`
}

func (p *Payload) UnmarshalJSON(data []byte) error {
	var decoded Payload
	var comments commentList
	fields := map[string]any{"summary": &decoded.Summary, "comments": &comments}
	if err := strictjson.Object(data, []string{"summary", "comments"}, fields); err != nil {
		return err
	}
	decoded.Comments = []Comment(comments)
	*p = decoded
	return nil
}

func (p Payload) MarshalJSON() ([]byte, error) {
	type wire Payload
	decoded := wire(p)
	decoded.Comments = append([]Comment{}, p.Comments...)
	return json.Marshal(decoded)
}

type commentList []Comment

func (c *commentList) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expected array")
	}
	comments := make(commentList, len(raw))
	for index, item := range raw {
		if err := json.Unmarshal(item, &comments[index]); err != nil {
			return fmt.Errorf("comment[%d]: %w", index, err)
		}
	}
	*c = comments
	return nil
}

type Result struct {
	Reviewer  string
	CreatedAt string
	Summary   string
	Comments  []Comment
}
