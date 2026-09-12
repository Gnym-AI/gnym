package reviewer

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAccept(t *testing.T) {
	p := Payload{Summary: " keep text ", Comments: []Comment{{File: "src/a:b.go", Side: "old", Line: 999, EndLine: math.MaxInt64, Severity: SeverityHigh, Message: " keep message "}}}
	now := time.Date(2026, 9, 12, 12, 0, 0, 123, time.FixedZone("other", 3600))
	got, err := Accept("configured", p, DiffFiles("diff --git a/src/a:b.go b/src/a:b.go\n"), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reviewer != "configured" || got.CreatedAt != "2026-09-12T11:00:00.000000123Z" || got.Summary != p.Summary || !reflect.DeepEqual(got.Comments, p.Comments) {
		t.Fatalf("content/stamping changed: %#v", got)
	}
	for _, diff := range []string{"", "not a diff"} {
		r, err := Accept("r", Payload{Summary: "nothing"}, DiffFiles(diff), now)
		if err != nil || r.Comments == nil {
			t.Fatalf("empty findings: %#v %v", r, err)
		}
	}
	_, err = Accept("r", p, DiffFiles("arbitrary"), now)
	if err == nil || !strings.Contains(err.Error(), "comments[0].file: not represented") {
		t.Fatalf("membership diagnostic: %v", err)
	}
}
func TestPayloadValidation(t *testing.T) {
	valid := Comment{File: "main.go", Severity: SeverityLow, Message: "message"}
	tests := []struct {
		name string
		edit func(*Comment)
	}{
		{"empty file", func(c *Comment) { c.File = "" }}, {"absolute", func(c *Comment) { c.File = "/a" }}, {"drive", func(c *Comment) { c.File = "C:a" }},
		{"traversal", func(c *Comment) { c.File = "a/../b" }}, {"dot", func(c *Comment) { c.File = "./b" }}, {"empty component", func(c *Comment) { c.File = "a//b" }},
		{"backslash", func(c *Comment) { c.File = `a\b` }}, {"control", func(c *Comment) { c.File = "a\nb" }}, {"utf8", func(c *Comment) { c.File = string([]byte{255}) }},
		{"blank message", func(c *Comment) { c.Message = " \t" }}, {"severity", func(c *Comment) { c.Severity = "warning" }},
		{"side alone", func(c *Comment) { c.Side = "new" }}, {"line alone", func(c *Comment) { c.Line = 1 }}, {"end alone", func(c *Comment) { c.EndLine = 1 }},
		{"bad side", func(c *Comment) { c.Side = "other"; c.Line = 1 }}, {"negative", func(c *Comment) { c.Side = "old"; c.Line = -1 }},
		{"reversed", func(c *Comment) { c.Side = "new"; c.Line = 2; c.EndLine = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.edit(&c)
			_, err := Accept("r", Payload{Summary: "summary", Comments: []Comment{valid, c}}, map[string]struct{}{"main.go": {}}, time.Now())
			if err == nil || !strings.Contains(err.Error(), "comments[1]") {
				t.Fatalf("expected whole result rejection with index: %v", err)
			}
		})
	}
	for _, severity := range []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical} {
		c := valid
		c.Severity = severity
		if err := ValidatePayload(Payload{Summary: "s", Comments: []Comment{c}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidatePayload(Payload{Summary: "\n"}); err == nil {
		t.Fatal("accepted blank summary")
	}
}
func TestPayloadJSONBoundary(t *testing.T) {
	invalid := []string{`{}`, `null`, `[]`, `{"summary":"s"}`, `{"summary":"s","comments":null}`, `{"summary":3,"comments":[]}`, `{"summary":"s","comments":[],"reviewer":"spoof"}`, `{"summary":"s","comments":[],"created_at":"spoof"}`, `{"Summary":"s","comments":[]}`,
		`{"summary":"s","comments":[null]}`, `{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","line":0}]}`, `{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":null}]}`, `{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","line":9223372036854775808}]}`, `{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","line":1.5}]}`, `{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","extra":1}]}`}
	for _, raw := range invalid {
		var p Payload
		if err := json.Unmarshal([]byte(raw), &p); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	var p Payload
	if err := json.Unmarshal([]byte(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":"new","line":9223372036854775807}]}`), &p); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePayload(p); err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{Payload{Summary: "s"}, Result{Summary: "s"}} {
		b, err := json.Marshal(v)
		if err != nil || !strings.Contains(string(b), `"comments":[]`) {
			t.Fatalf("nil comments: %s %v", b, err)
		}
	}
	b, _ := json.Marshal(Comment{File: "a", Severity: SeverityLow, Message: "m"})
	if strings.Contains(string(b), "line") || strings.Contains(string(b), "side") {
		t.Fatalf("file-wide fields: %s", b)
	}
}
