package schemas

import (
	"encoding/json"
	"gnym/review"
	"gnym/reviewer"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// These assertions guard the checked-in schemas' structural contract against
// the Go types. Semantic constraints remain covered in reviewer/review tests.
func TestSchemaContract(t *testing.T) {
	load := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var s map[string]any
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.UseNumber()
		if err = d.Decode(&s); err != nil {
			t.Fatal(err)
		}
		if s["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatal("unexpected schema dialect")
		}
		return s
	}
	payload := load("reviewer-payload.schema.json")
	run := load("review-run.schema.json")
	object := func(schema map[string]any, typ reflect.Type) {
		t.Helper()
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatal("object must reject unknown fields")
		}
		props := schema["properties"].(map[string]any)
		required := []string{}
		keys := []string{}
		for i := 0; i < typ.NumField(); i++ {
			tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")
			keys = append(keys, tag[0])
			if len(tag) == 1 {
				required = append(required, tag[0])
			}
		}
		gotKeys := []string{}
		for k := range props {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, gotKeys) {
			t.Fatalf("properties: %v != %v", gotKeys, keys)
		}
		gotRequired := []string{}
		for _, x := range schema["required"].([]any) {
			gotRequired = append(gotRequired, x.(string))
		}
		sort.Strings(required)
		sort.Strings(gotRequired)
		if !reflect.DeepEqual(required, gotRequired) {
			t.Fatalf("required: %v != %v", gotRequired, required)
		}
	}
	object(payload, reflect.TypeOf(reviewer.Payload{}))
	object(run, reflect.TypeOf(review.Run{}))
	defs := run["$defs"].(map[string]any)
	comment := defs["comment"].(map[string]any)
	object(comment, reflect.TypeOf(reviewer.Comment{}))
	object(defs["result"].(map[string]any), reflect.TypeOf(reviewer.Result{}))
	object(defs["failure"].(map[string]any), reflect.TypeOf(review.Failure{}))
	if !reflect.DeepEqual(comment, payload["$defs"].(map[string]any)["comment"]) {
		t.Fatal("comment schemas diverged")
	}
	props := comment["properties"].(map[string]any)
	wantSeverity := []any{string(reviewer.SeverityLow), string(reviewer.SeverityMedium), string(reviewer.SeverityHigh), string(reviewer.SeverityCritical)}
	if !reflect.DeepEqual(props["severity"].(map[string]any)["enum"], wantSeverity) {
		t.Fatal("severity drift")
	}
	for _, key := range []string{"line", "end_line"} {
		p := props[key].(map[string]any)
		if p["type"] != "integer" || p["minimum"] != json.Number("1") || p["maximum"] != json.Number("9223372036854775807") {
			t.Fatalf("%s integer bounds drift", key)
		}
	}
	deps := comment["dependentRequired"].(map[string]any)
	if !reflect.DeepEqual(deps, map[string]any{"side": []any{"line"}, "line": []any{"side"}, "end_line": []any{"side", "line"}}) {
		t.Fatal("location dependencies drift")
	}
	runProps := run["properties"].(map[string]any)
	if runProps["schema_version"].(map[string]any)["const"] != "1" {
		t.Fatal("version drift")
	}
	if !reflect.DeepEqual(runProps["status"].(map[string]any)["enum"], []any{string(review.StatusComplete), string(review.StatusPartial), string(review.StatusFailed)}) {
		t.Fatal("status drift")
	}
	codes := defs["failure"].(map[string]any)["properties"].(map[string]any)["code"].(map[string]any)["enum"]
	if !reflect.DeepEqual(codes, []any{string(review.FailureInvalidOutput), string(review.FailureReviewerError)}) {
		t.Fatal("failure code drift")
	}
	for _, s := range []map[string]any{payload, defs["result"].(map[string]any), run} {
		p := s["properties"].(map[string]any)
		for _, key := range []string{"comments", "reviews", "failures"} {
			if v, ok := p[key]; ok && v.(map[string]any)["type"] != "array" {
				t.Fatalf("%s must exclude null", key)
			}
		}
	}
	branches := run["oneOf"].([]any)
	if len(branches) != 3 {
		t.Fatal("missing cardinality cases")
	}
	for i, want := range []struct {
		status     string
		rmin, fmin string
		rmax, fmax any
	}{{"complete", "1", "0", nil, json.Number("0")}, {"partial", "1", "1", nil, nil}, {"failed", "0", "1", json.Number("0"), nil}} {
		p := branches[i].(map[string]any)["properties"].(map[string]any)
		r := p["reviews"].(map[string]any)
		f := p["failures"].(map[string]any)
		if p["status"].(map[string]any)["const"] != want.status || r["minItems"] != json.Number(want.rmin) || f["minItems"] != json.Number(want.fmin) || r["maxItems"] != want.rmax || f["maxItems"] != want.fmax {
			t.Fatal("cardinality drift")
		}
	}
}
