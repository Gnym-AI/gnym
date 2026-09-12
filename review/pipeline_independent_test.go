package review_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"gnym/input"
	"gnym/review"
	"gnym/reviewer"
	"gnym/sink"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type providerFunc func(reviewer.Request) (reviewer.Payload, error)

func (f providerFunc) Review(r reviewer.Request) (reviewer.Payload, error) { return f(r) }

func TestIndependentPipelinePreservesWireLocationsAndOrder(t *testing.T) {
	dir := t.TempDir()
	diffPath, outPath := filepath.Join(dir, "input.diff"), filepath.Join(dir, "output.json")
	diff := "diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-old\n+new"
	if err := os.WriteFile(diffPath, []byte(diff), 0600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	provider := providerFunc(func(r reviewer.Request) (reviewer.Payload, error) {
		calls = append(calls, r.Config.Name)
		if r.Diff != diff {
			t.Fatalf("request diff got %q, want %q", r.Diff, diff)
		}
		if r.Config.Name == "second" {
			return reviewer.Payload{Summary: " no findings "}, nil
		}
		var p reviewer.Payload
		err := json.Unmarshal([]byte(`{"summary":" findings ","comments":[{"file":"a","severity":"critical","message":" retained ","side":"old","line":9007199254740993,"end_line":9223372036854775807},{"file":"a","severity":"medium","message":"file-wide"}]}`), &p)
		return p, err
	})
	coordinator := review.NewCoordinator(&input.FileDiffSource{Path: diffPath}, provider, []reviewer.Config{{Name: "first"}, {Name: "second"}}, &sink.File{Path: outPath})
	before := time.Now()
	if err := coordinator.Run(); err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("calls got %v, want first, second", calls)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// Decode wire independently, preserving numbers above float64's exact range.
	var wire map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&wire); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"schema_version": "1", "status": "complete", "failures": []any{},
		"reviews": []any{
			map[string]any{"reviewer": "first", "summary": " findings ", "comments": []any{
				map[string]any{"file": "a", "severity": "critical", "message": " retained ", "side": "old", "line": json.Number("9007199254740993"), "end_line": json.Number("9223372036854775807")},
				map[string]any{"file": "a", "severity": "medium", "message": "file-wide"},
			}},
			map[string]any{"reviewer": "second", "summary": " no findings ", "comments": []any{}},
		},
	}
	for i, raw := range wire["reviews"].([]any) {
		result := raw.(map[string]any)
		stamp, ok := result["created_at"].(string)
		if !ok {
			t.Fatal("missing acceptance timestamp")
		}
		accepted, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil || !strings.HasSuffix(stamp, "Z") || accepted.Before(before) || accepted.After(after) {
			t.Fatalf("acceptance time got %q (%v), want UTC within run", stamp, err)
		}
		want["reviews"].([]any)[i].(map[string]any)["created_at"] = stamp
	}
	if !reflect.DeepEqual(wire, want) {
		t.Fatalf("wire got %#v, want %#v", wire, want)
	}
}

func TestIndependentPipelineFailurePreservesExistingOutput(t *testing.T) {
	for _, failure := range []string{"provider", "structure", "membership"} {
		for _, at := range []int{0, 1} {
			t.Run(failure+string(rune('0'+at)), func(t *testing.T) {
				dir := t.TempDir()
				diffPath, outPath := filepath.Join(dir, "in"), filepath.Join(dir, "out")
				if err := os.WriteFile(diffPath, []byte("diff --git a/a b/a"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(outPath, []byte("previous output"), 0600); err != nil {
					t.Fatal(err)
				}
				calls := 0
				sentinel := errors.New("provider unavailable")
				provider := providerFunc(func(r reviewer.Request) (reviewer.Payload, error) {
					i := calls
					calls++
					if i != at {
						return reviewer.Payload{Summary: "ok"}, nil
					}
					switch failure {
					case "provider":
						return reviewer.Payload{}, sentinel
					case "structure":
						return reviewer.Payload{Summary: " "}, nil
					default:
						return reviewer.Payload{Summary: "s", Comments: []reviewer.Comment{{File: "missing", Severity: reviewer.SeverityLow, Message: "m"}}}, nil
					}
				})
				configs := []reviewer.Config{{Name: "first"}, {Name: "second"}, {Name: "third"}}
				err := review.NewCoordinator(&input.FileDiffSource{Path: diffPath}, provider, configs, &sink.File{Path: outPath}).Run()
				if err == nil || !strings.Contains(err.Error(), configs[at].Name) {
					t.Fatalf("error got %v, want reviewer failure", err)
				}
				if failure == "provider" && !errors.Is(err, sentinel) {
					t.Fatalf("error got %v, want wrapped sentinel", err)
				}
				if calls != at+1 {
					t.Fatalf("calls got %d, want %d", calls, at+1)
				}
				data, err := os.ReadFile(outPath)
				if err != nil || string(data) != "previous output" {
					t.Fatalf("output got %q (%v), want preserved", data, err)
				}
			})
		}
	}
}
