package schemas_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// This complements runtime fixture tests by checking every schema leaf and
// reference that those fixtures rely on. It is not a JSON Schema interpreter.
func TestIndependentSchemaLeafConstraintsAndReferences(t *testing.T) {
	for _, file := range []string{"reviewer-payload.schema.json", "review-run.schema.json"} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var root map[string]any
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatal(err)
			}
			at := func(path string) any {
				t.Helper()
				var value any = root
				for _, key := range strings.Split(path, "/") {
					object, ok := value.(map[string]any)
					if !ok {
						t.Fatalf("schema %s is not an object at %s", file, key)
					}
					value, ok = object[key]
					if !ok {
						t.Fatalf("schema %s missing %s", file, path)
					}
				}
				return value
			}
			checks := map[string]any{
				"$defs/comment/properties/file":    map[string]any{"type": "string", "minLength": float64(1)},
				"$defs/comment/properties/message": map[string]any{"type": "string", "minLength": float64(1)},
				"$defs/comment/properties/side":    map[string]any{"enum": []any{"old", "new"}},
			}
			if file == "reviewer-payload.schema.json" {
				checks["properties/summary"] = map[string]any{"type": "string", "minLength": float64(1)}
				checks["properties/comments/items/$ref"] = "#/$defs/comment"
			} else {
				checks["properties/reviews/items/$ref"] = "#/$defs/result"
				checks["properties/failures/items/$ref"] = "#/$defs/failure"
				checks["$defs/result/properties/comments/items/$ref"] = "#/$defs/comment"
				for _, path := range []string{"$defs/result/properties/summary", "$defs/result/properties/reviewer", "$defs/failure/properties/reviewer", "$defs/failure/properties/message"} {
					checks[path] = map[string]any{"type": "string", "minLength": float64(1)}
				}
				checks["$defs/result/properties/created_at"] = map[string]any{"type": "string", "format": "date-time", "pattern": "Z$"}
			}
			for path, want := range checks {
				if got := at(path); !reflect.DeepEqual(got, want) {
					t.Errorf("%s got %#v, want %#v", path, got, want)
				}
			}
			// Resolve references recursively: stale or external references cannot
			// silently leave parts of the public contract unchecked/offline.
			var walk func(any)
			walk = func(value any) {
				switch v := value.(type) {
				case map[string]any:
					if ref, exists := v["$ref"]; exists {
						s, ok := ref.(string)
						if !ok || !strings.HasPrefix(s, "#/") {
							t.Fatalf("nonlocal reference %#v", ref)
						}
						if _, ok := at(strings.TrimPrefix(s, "#/")).(map[string]any); !ok {
							t.Fatalf("reference %s does not resolve to schema", s)
						}
					}
					for _, child := range v {
						walk(child)
					}
				case []any:
					for _, child := range v {
						walk(child)
					}
				}
			}
			walk(root)
		})
	}
}
