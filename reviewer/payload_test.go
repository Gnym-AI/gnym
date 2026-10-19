package reviewer

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func decodePayload(data []byte) (Payload, error) {
	var payload Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return Payload{}, err
	}
	if err := ValidatePayload(payload); err != nil {
		return Payload{}, err
	}
	return payload, nil
}

func TestPayloadBoundaryAcceptsAndPreservesValidContent(t *testing.T) {
	raw := []byte(`{"summary":" keep summary ","comments":[{"file":"src/a:b.go","severity":"low","message":"file wide"},{"file":"src/b.go","side":"old","line":12,"end_line":14,"severity":"critical","message":" keep message "}]}`)
	got, err := decodePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := Payload{Summary: " keep summary ", Comments: []Comment{
		{File: "src/a:b.go", Severity: SeverityLow, Message: "file wide"},
		{File: "src/b.go", Side: "old", Line: 12, EndLine: 14, Severity: SeverityCritical, Message: " keep message "},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload = %#v, want %#v", got, want)
	}

	for _, severity := range []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical} {
		payload := Payload{Summary: "summary", Comments: []Comment{{File: "main.go", Severity: severity, Message: "message"}}}
		if err := ValidatePayload(payload); err != nil {
			t.Errorf("severity %q rejected: %v", severity, err)
		}
	}
}

func TestPayloadCommentsNormalizeAndLocationsOmit(t *testing.T) {
	decoded, err := decodePayload([]byte(`{"summary":"none","comments":[]}`))
	if err != nil || decoded.Comments == nil {
		t.Fatalf("decoded comments = %#v, error = %v", decoded.Comments, err)
	}

	encoded, err := json.Marshal(Payload{Summary: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"summary":"none","comments":[]}` {
		t.Fatalf("encoded payload = %s", encoded)
	}
	comment, err := json.Marshal(Comment{File: "main.go", Severity: SeverityLow, Message: "message"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(comment), "side") || strings.Contains(string(comment), "line") {
		t.Fatalf("file-wide comment contains a location: %s", comment)
	}
}

func TestPayloadBoundaryRejectsInvalidJSON(t *testing.T) {
	invalidUTF8 := append([]byte(`{"summary":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`","comments":[]}`)...)
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{"null payload", []byte(`null`), "expected object"},
		{"array payload", []byte(`[]`), "expected object"},
		{"missing summary", []byte(`{"comments":[]}`), "summary: required"},
		{"missing comments", []byte(`{"summary":"s"}`), "comments: required"},
		{"null comments", []byte(`{"summary":"s","comments":null}`), "comments: null"},
		{"wrong summary type", []byte(`{"summary":3,"comments":[]}`), "summary: incompatible"},
		{"unknown payload field", []byte(`{"summary":"s","comments":[],"extra":true}`), "unknown field"},
		{"spoofed reviewer", []byte(`{"summary":"s","comments":[],"reviewer":"spoof"}`), "unknown field"},
		{"spoofed time", []byte(`{"summary":"s","comments":[],"created_at":"spoof"}`), "unknown field"},
		{"null comment", []byte(`{"summary":"s","comments":[null]}`), "comment[0]"},
		{"missing comment field", []byte(`{"summary":"s","comments":[{"file":"a","severity":"low"}]}`), "comment[0]: message: required"},
		{"unknown comment field", []byte(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","extra":1}]}`), "comment[0]: unknown field"},
		{"null location", []byte(`{"summary":"s","comments":[{"file":"a","severity":"low","message":"m","side":null}]}`), "comment[0]: side: null"},
		{"end line overflow", []byte(`{"summary":"s","comments":[{"file":"a","side":"new","line":1,"end_line":9223372036854775808,"severity":"low","message":"m"}]}`), "comment[0]: end_line"},
		{"invalid UTF-8", invalidUTF8, "invalid UTF-8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodePayload(test.raw)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
			if len(err.Error()) > 200 {
				t.Fatalf("diagnostic length = %d", len(err.Error()))
			}
		})
	}
}

func TestPayloadValidationRejectsInvalidContent(t *testing.T) {
	valid := Comment{File: "main.go", Severity: SeverityLow, Message: "message"}
	tests := []struct {
		name string
		edit func(*Payload)
		want string
	}{
		{"blank summary", func(p *Payload) { p.Summary = " \n" }, "summary"},
		{"empty file", func(p *Payload) { p.Comments[1].File = "" }, "comments[1].file"},
		{"absolute path", func(p *Payload) { p.Comments[1].File = "/a" }, "comments[1].file"},
		{"drive path", func(p *Payload) { p.Comments[1].File = "C:/a" }, "comments[1].file"},
		{"dot path", func(p *Payload) { p.Comments[1].File = "./a" }, "comments[1].file"},
		{"traversal", func(p *Payload) { p.Comments[1].File = "a/../b" }, "comments[1].file"},
		{"empty component", func(p *Payload) { p.Comments[1].File = "a//b" }, "comments[1].file"},
		{"backslash", func(p *Payload) { p.Comments[1].File = `a\b` }, "comments[1].file"},
		{"control", func(p *Payload) { p.Comments[1].File = "a\nb" }, "comments[1].file"},
		{"invalid UTF-8 path", func(p *Payload) { p.Comments[1].File = string([]byte{0xff}) }, "comments[1].file"},
		{"blank message", func(p *Payload) { p.Comments[1].Message = "\t" }, "comments[1].message"},
		{"legacy severity", func(p *Payload) { p.Comments[1].Severity = SeverityWarning }, "comments[1].severity"},
		{"side alone", func(p *Payload) { p.Comments[1].Side = "new" }, "comments[1].line"},
		{"line alone", func(p *Payload) { p.Comments[1].Line = 1 }, "comments[1].side"},
		{"end alone", func(p *Payload) { p.Comments[1].EndLine = 1 }, "comments[1].side"},
		{"bad side", func(p *Payload) { p.Comments[1].Side = "right"; p.Comments[1].Line = 1 }, "comments[1].side"},
		{"negative line", func(p *Payload) { p.Comments[1].Side = "old"; p.Comments[1].Line = -1 }, "comments[1].line"},
		{"reversed range", func(p *Payload) { p.Comments[1].Side = "new"; p.Comments[1].Line = 2; p.Comments[1].EndLine = 1 }, "comments[1].end_line"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := Payload{Summary: "summary", Comments: []Comment{valid, valid}}
			test.edit(&payload)
			err := ValidatePayload(payload)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLocationNumbersAreExactPositiveInt64(t *testing.T) {
	valid := map[string]int64{
		"1": 1, "1.0": 1, "1e0": 1, "1E+1": 10, "0.1e1": 1, "12.300e1": 123,
		"9007199254740993.0": 9007199254740993, "9.223372036854775807e18": math.MaxInt64,
	}
	for raw, want := range valid {
		data := fmt.Sprintf(`{"summary":"s","comments":[{"file":"a","side":"new","line":%s,"end_line":%s,"severity":"high","message":"m"}]}`, raw, raw)
		payload, err := decodePayload([]byte(data))
		if err != nil || payload.Comments[0].Line != want || payload.Comments[0].EndLine != want {
			t.Fatalf("token %s decoded as %#v, error = %v", raw, payload.Comments, err)
		}
	}

	long := strings.Repeat("9", 10000)
	invalid := []string{"0", "-1", "1.5", "1e-1", "10.01", "9223372036854775808", "9.223372036854775808e18", `"1"`, "true", long}
	for _, raw := range invalid {
		data := fmt.Sprintf(`{"summary":"s","comments":[{"file":"a","side":"new","line":%s,"severity":"low","message":"m"}]}`, raw)
		_, err := decodePayload([]byte(data))
		if err == nil || !strings.Contains(err.Error(), "comment[0]") || !strings.Contains(err.Error(), "line") || len(err.Error()) > 200 {
			t.Fatalf("token length %d produced diagnostic %q", len(raw), err)
		}
	}
}
