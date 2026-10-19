package schemas

import (
	"encoding/json"
	"errors"
	"gnym/review"
	"gnym/reviewer"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type offlineLoader struct{}

func (offlineLoader) Load(string) (any, error) {
	return nil, errors.New("external schema loading is disabled")
}

func loadSchema(t *testing.T, name string) (*jsonschema.Schema, map[string]any) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	compiler.UseLoader(offlineLoader{})
	url := "https://gnym.dev/schemas/" + name
	if err := compiler.AddResource(url, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatal(err)
	}
	return schema, document
}

func decode(t *testing.T, source string) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestSchemasEnforceStructuralContract(t *testing.T) {
	tests := []struct {
		name, file, instance string
		valid                bool
	}{
		{"payload file wide", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","severity":"low","message":"note"}]}`, true},
		{"payload line range", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","side":"new","line":1,"end_line":2,"severity":"critical","message":"note"}]}`, true},
		{"payload missing field", "reviewer-payload.schema.json", `{"comments":[]}`, false},
		{"payload null array", "reviewer-payload.schema.json", `{"summary":"ok","comments":null}`, false},
		{"payload unknown field", "reviewer-payload.schema.json", `{"summary":"ok","comments":[],"reviewer":"spoofed"}`, false},
		{"payload legacy severity", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","severity":"warning","message":"note"}]}`, false},
		{"payload lone side", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","side":"new","severity":"low","message":"note"}]}`, false},
		{"payload end without line", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","end_line":2,"severity":"low","message":"note"}]}`, false},
		{"payload noninteger line", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","side":"new","line":1.5,"severity":"low","message":"note"}]}`, false},
		{"payload line overflow", "reviewer-payload.schema.json", `{"summary":"ok","comments":[{"file":"a.go","side":"new","line":9223372036854775808,"severity":"low","message":"note"}]}`, false},
		{"complete run", "review-run.schema.json", `{"schema_version":"1","status":"complete","reviews":[{"reviewer":"r","created_at":"2026-01-02T03:04:05Z","summary":"ok","comments":[]}],"failures":[]}`, true},
		{"partial run", "review-run.schema.json", `{"schema_version":"1","status":"partial","reviews":[{"reviewer":"r","created_at":"2026-01-02T03:04:05.1Z","summary":"ok","comments":[]}],"failures":[{"reviewer":"x","code":"reviewer_error","message":"bad"}]}`, true},
		{"failed run", "review-run.schema.json", `{"schema_version":"1","status":"failed","reviews":[],"failures":[{"reviewer":"r","code":"invalid_output","message":"bad"}]}`, true},
		{"unsupported version", "review-run.schema.json", `{"schema_version":"2","status":"failed","reviews":[],"failures":[{"reviewer":"r","code":"invalid_output","message":"bad"}]}`, false},
		{"inconsistent complete", "review-run.schema.json", `{"schema_version":"1","status":"complete","reviews":[],"failures":[]}`, false},
		{"inconsistent failed", "review-run.schema.json", `{"schema_version":"1","status":"failed","reviews":[{"reviewer":"r","created_at":"2026-01-02T03:04:05Z","summary":"ok","comments":[]}],"failures":[]}`, false},
		{"unknown failure code", "review-run.schema.json", `{"schema_version":"1","status":"failed","reviews":[],"failures":[{"reviewer":"r","code":"timeout","message":"bad"}]}`, false},
		{"unknown nested field", "review-run.schema.json", `{"schema_version":"1","status":"complete","reviews":[{"reviewer":"r","created_at":"2026-01-02T03:04:05Z","summary":"ok","comments":[],"extra":true}],"failures":[]}`, false},
	}
	compiled := map[string]*jsonschema.Schema{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := compiled[test.file]
			if schema == nil {
				schema, _ = loadSchema(t, test.file)
				compiled[test.file] = schema
			}
			err := schema.Validate(decode(t, test.instance))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
			var runtimeErr error
			if test.file == "reviewer-payload.schema.json" {
				var payload reviewer.Payload
				runtimeErr = json.Unmarshal([]byte(test.instance), &payload)
				if runtimeErr == nil {
					runtimeErr = reviewer.ValidatePayload(payload)
				}
			} else {
				var run review.Run
				runtimeErr = json.Unmarshal([]byte(test.instance), &run)
				if runtimeErr == nil {
					runtimeErr = run.Validate()
				}
			}
			if (runtimeErr == nil) != test.valid {
				t.Fatalf("schema/runtime validity drift: valid=%v, runtime error=%v", test.valid, runtimeErr)
			}
		})
	}
}

func TestSchemaContractDoesNotDrift(t *testing.T) {
	_, payload := loadSchema(t, "reviewer-payload.schema.json")
	_, run := loadSchema(t, "review-run.schema.json")
	if payload["$schema"] != "https://json-schema.org/draft/2020-12/schema" || run["$schema"] != payload["$schema"] {
		t.Fatal("schemas must use Draft 2020-12")
	}
	payloadComment := definition(t, payload, "comment")
	runComment := definition(t, run, "comment")
	if !reflect.DeepEqual(payloadComment, runComment) {
		t.Fatal("shared comment definitions diverged")
	}

	comment := reviewer.Comment{File: "a.go", Side: "new", Line: 1, EndLine: 2, Severity: reviewer.SeverityLow, Message: "ok"}
	runValue := review.Run{SchemaVersion: "1", Status: review.StatusPartial,
		Results:  []reviewer.Result{{Reviewer: "r", CreatedAt: "2026-01-02T03:04:05Z", Summary: "ok", Comments: []reviewer.Comment{comment}}},
		Failures: []review.Failure{{Reviewer: "x", Code: review.FailureInvalidOutput, Message: "bad"}},
	}
	payloadValue := jsonObject(t, reviewer.Payload{Summary: "ok", Comments: []reviewer.Comment{comment}})
	runObject := jsonObject(t, runValue)
	assertObjectFields(t, payload, payloadValue, []string{"summary", "comments"})
	assertObjectFields(t, payloadComment, payloadValue["comments"].([]any)[0].(map[string]any), []string{"file", "severity", "message"})
	assertObjectFields(t, run, runObject, []string{"schema_version", "status", "reviews", "failures"})
	assertObjectFields(t, definition(t, run, "result"), runObject["reviews"].([]any)[0].(map[string]any), []string{"reviewer", "created_at", "summary", "comments"})
	assertObjectFields(t, definition(t, run, "failure"), runObject["failures"].([]any)[0].(map[string]any), []string{"reviewer", "code", "message"})

	assertEnum(t, property(t, payloadComment, "severity"), []string{
		string(reviewer.SeverityLow), string(reviewer.SeverityMedium),
		string(reviewer.SeverityHigh), string(reviewer.SeverityCritical),
	})
	assertEnum(t, property(t, run, "status"), []string{
		string(review.StatusComplete), string(review.StatusPartial), string(review.StatusFailed),
	})
	assertEnum(t, property(t, definition(t, run, "failure"), "code"), []string{
		string(review.FailureInvalidOutput), string(review.FailureReviewerError),
	})
	for _, document := range []map[string]any{payload, run} {
		assertLocalReferences(t, document, document)
	}
}

func TestSemanticOnlyRulesRemainInGoValidation(t *testing.T) {
	schema, _ := loadSchema(t, "reviewer-payload.schema.json")
	instances := []struct {
		name, source string
		payload      reviewer.Payload
	}{
		{"blank text", `{"summary":" ","comments":[]}`, reviewer.Payload{Summary: " ", Comments: []reviewer.Comment{}}},
		{"unsafe path", `{"summary":"ok","comments":[{"file":"../a.go","severity":"low","message":"ok"}]}`, reviewer.Payload{Summary: "ok", Comments: []reviewer.Comment{{File: "../a.go", Severity: reviewer.SeverityLow, Message: "ok"}}}},
		{"reversed range", `{"summary":"ok","comments":[{"file":"a.go","side":"new","line":2,"end_line":1,"severity":"low","message":"ok"}]}`, reviewer.Payload{Summary: "ok", Comments: []reviewer.Comment{{File: "a.go", Side: "new", Line: 2, EndLine: 1, Severity: reviewer.SeverityLow, Message: "ok"}}}},
	}
	for _, instance := range instances {
		t.Run(instance.name, func(t *testing.T) {
			if err := schema.Validate(decode(t, instance.source)); err != nil {
				t.Fatalf("rule unexpectedly moved into JSON Schema: %v", err)
			}
			if err := reviewer.ValidatePayload(instance.payload); err == nil {
				t.Fatal("Go semantic validation accepted invalid payload")
			}
		})
	}

	runSchema, _ := loadSchema(t, "review-run.schema.json")
	duplicate := `{"schema_version":"1","status":"partial","reviews":[{"reviewer":"r","created_at":"2026-01-02T03:04:05Z","summary":"ok","comments":[]}],"failures":[{"reviewer":"r","code":"reviewer_error","message":"bad"}]}`
	if err := runSchema.Validate(decode(t, duplicate)); err != nil {
		t.Fatalf("identity uniqueness unexpectedly moved into JSON Schema: %v", err)
	}
	var run review.Run
	if err := json.Unmarshal([]byte(duplicate), &run); err != nil {
		t.Fatal(err)
	}
	if err := run.Validate(); err == nil {
		t.Fatal("Go semantic validation accepted duplicate reviewer identity")
	}
}

func definition(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	return schema["$defs"].(map[string]any)[name].(map[string]any)
}

func property(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	return schema["properties"].(map[string]any)[name].(map[string]any)
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func assertObjectFields(t *testing.T, schema, object map[string]any, requiredFields []string) {
	t.Helper()
	want, got := sortedKeys(object), sortedKeys(schema["properties"].(map[string]any))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema properties %v differ from Go JSON fields %v", got, want)
	}
	required := make([]string, 0, len(schema["required"].([]any)))
	for _, field := range schema["required"].([]any) {
		required = append(required, field.(string))
	}
	sort.Strings(required)
	sort.Strings(requiredFields)
	if !reflect.DeepEqual(required, requiredFields) {
		t.Fatalf("required fields %v differ from contract fields %v", required, requiredFields)
	}
	if schema["additionalProperties"] != false {
		t.Fatal("object must reject additional properties")
	}
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func assertEnum(t *testing.T, schema map[string]any, want []string) {
	t.Helper()
	got := make([]string, 0, len(schema["enum"].([]any)))
	for _, value := range schema["enum"].([]any) {
		got = append(got, value.(string))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enum %v differs from Go values %v", got, want)
	}
}

func assertLocalReferences(t *testing.T, root map[string]any, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if reference, ok := typed["$ref"]; ok {
			path, ok := reference.(string)
			if !ok || !strings.HasPrefix(path, "#/") {
				t.Fatalf("nonlocal reference %v", reference)
			}
			var target any = root
			for _, part := range strings.Split(strings.TrimPrefix(path, "#/"), "/") {
				target = target.(map[string]any)[part]
			}
			if _, ok := target.(map[string]any); !ok {
				t.Fatalf("reference %s does not resolve", path)
			}
		}
		for _, child := range typed {
			assertLocalReferences(t, root, child)
		}
	case []any:
		for _, child := range typed {
			assertLocalReferences(t, root, child)
		}
	}
}
