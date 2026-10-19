package schemas

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestPublishedSchemaLeavesAndReferences(t *testing.T) {
	tests := []struct {
		file       string
		rootFields []string
		defs       []string
	}{
		{"reviewer-payload.schema.json", []string{"comments", "summary"}, []string{"comment"}},
		{"review-run.schema.json", []string{"failures", "reviews", "schema_version", "status"}, []string{"comment", "failure", "result"}},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			document := readSchemaDocument(t, test.file)
			if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("$schema = %v", document["$schema"])
			}
			if !strings.HasPrefix(document["$id"].(string), "https://gnym.dev/schemas/") {
				t.Fatalf("$id = %v", document["$id"])
			}
			assertKeys(t, document["properties"].(map[string]any), test.rootFields)
			assertKeys(t, document["$defs"].(map[string]any), test.defs)
			walkLocalReferences(t, document, document)
		})
	}

	payload := readSchemaDocument(t, "reviewer-payload.schema.json")
	run := readSchemaDocument(t, "review-run.schema.json")
	payloadComment := payload["$defs"].(map[string]any)["comment"]
	runComment := run["$defs"].(map[string]any)["comment"]
	if !reflect.DeepEqual(payloadComment, runComment) {
		t.Fatal("provider and run comment schemas differ")
	}
	commentProperties := payloadComment.(map[string]any)["properties"].(map[string]any)
	assertKeys(t, commentProperties, []string{"end_line", "file", "line", "message", "severity", "side"})
	assertStrings(t, commentProperties["severity"].(map[string]any)["enum"], []string{"critical", "high", "low", "medium"})
	assertStrings(t, commentProperties["side"].(map[string]any)["enum"], []string{"new", "old"})
	for _, field := range []string{"line", "end_line"} {
		leaf := commentProperties[field].(map[string]any)
		if leaf["type"] != "integer" || leaf["minimum"].(json.Number).String() != "1" || leaf["maximum"].(json.Number).String() != "9223372036854775807" {
			t.Fatalf("%s constraints = %#v", field, leaf)
		}
	}
	assertStrings(t, run["properties"].(map[string]any)["status"].(map[string]any)["enum"], []string{"complete", "failed", "partial"})
	failure := run["$defs"].(map[string]any)["failure"].(map[string]any)
	assertStrings(t, failure["properties"].(map[string]any)["code"].(map[string]any)["enum"], []string{"invalid_output", "reviewer_error"})
}

func readSchemaDocument(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	return document
}

func assertKeys(t *testing.T, object map[string]any, want []string) {
	t.Helper()
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

func assertStrings(t *testing.T, values any, want []string) {
	t.Helper()
	raw := values.([]any)
	got := make([]string, len(raw))
	for index, value := range raw {
		got[index] = value.(string)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
}

func walkLocalReferences(t *testing.T, root map[string]any, value any) {
	t.Helper()
	switch node := value.(type) {
	case map[string]any:
		if reference, ok := node["$ref"]; ok {
			path, ok := reference.(string)
			if !ok || !strings.HasPrefix(path, "#/") {
				t.Fatalf("nonlocal reference %v", reference)
			}
			var target any = root
			for _, component := range strings.Split(strings.TrimPrefix(path, "#/"), "/") {
				object, ok := target.(map[string]any)
				if !ok {
					t.Fatalf("reference %q crosses a non-object", path)
				}
				target, ok = object[component]
				if !ok {
					t.Fatalf("reference %q does not resolve", path)
				}
			}
		}
		for _, child := range node {
			walkLocalReferences(t, root, child)
		}
	case []any:
		for _, child := range node {
			walkLocalReferences(t, root, child)
		}
	}
}
