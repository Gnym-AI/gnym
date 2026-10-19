package review_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gnym/review"
	"gnym/reviewer"
	"gnym/sink"
)

type staticDiff struct {
	content string
	err     error
	calls   int
}

func (s *staticDiff) GetDiff() (review.Diff, error) {
	s.calls++
	return review.Diff{Content: s.content}, s.err
}

type scriptedReviewer struct {
	payloads []reviewer.Payload
	errors   []error
	calls    []reviewer.Request
}

func (s *scriptedReviewer) Review(request reviewer.Request) (reviewer.Payload, error) {
	s.calls = append(s.calls, request)
	index := len(s.calls) - 1
	if index < len(s.errors) && s.errors[index] != nil {
		return reviewer.Payload{}, s.errors[index]
	}
	return s.payloads[index], nil
}

type envelope struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	Reviews       []struct {
		Reviewer  string `json:"reviewer"`
		CreatedAt string `json:"created_at"`
		Summary   string `json:"summary"`
		Comments  []struct {
			File     string `json:"file"`
			Side     string `json:"side,omitempty"`
			Line     int64  `json:"line,omitempty"`
			EndLine  int64  `json:"end_line,omitempty"`
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"comments"`
	} `json:"reviews"`
	Failures []struct {
		Reviewer string `json:"reviewer"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"failures"`
}

func TestCoordinatorAndFileSinkProduceIndependentWireEnvelope(t *testing.T) {
	diff := &staticDiff{content: "diff --git a/first.go b/first.go\ndiff --git a/second.go b/second.go"}
	providers := &scriptedReviewer{payloads: []reviewer.Payload{
		{Summary: "first summary", Comments: []reviewer.Comment{{File: "first.go", Side: "new", Line: 999, EndLine: 1001, Severity: reviewer.SeverityHigh, Message: "outside hunk"}}},
		{Summary: "second summary", Comments: []reviewer.Comment{}},
	}}
	configs := []reviewer.Config{{Name: "first"}, {Name: "second"}}
	path := filepath.Join(t.TempDir(), "result.json")
	if err := review.NewCoordinator(diff, providers, configs, &sink.File{Path: path}).Run(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var got envelope
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("decode actual output: %v\n%s", err, data)
	}
	if got.SchemaVersion != "1" || got.Status != "complete" || got.Failures == nil || len(got.Failures) != 0 {
		t.Fatalf("envelope header = %#v", got)
	}
	if names := []string{got.Reviews[0].Reviewer, got.Reviews[1].Reviewer}; !reflect.DeepEqual(names, []string{"first", "second"}) {
		t.Fatalf("review order = %v", names)
	}
	for index, result := range got.Reviews {
		stamp, err := time.Parse(time.RFC3339Nano, result.CreatedAt)
		if err != nil || stamp.Location() != time.UTC || result.Summary != providers.payloads[index].Summary || result.Comments == nil {
			t.Fatalf("review[%d] = %#v, timestamp error = %v", index, result, err)
		}
	}
	comment := got.Reviews[0].Comments[0]
	if comment.Line != 999 || comment.EndLine != 1001 || comment.Side != "new" {
		t.Fatalf("advisory location changed: %#v", comment)
	}
	if diff.calls != 1 || len(providers.calls) != 2 || providers.calls[0].Diff != diff.content || providers.calls[1].Diff != diff.content {
		t.Fatalf("pipeline calls: diff=%d reviewer=%#v", diff.calls, providers.calls)
	}
}

func TestCoordinatorFailurePreservesExistingFile(t *testing.T) {
	wantProviderErr := errors.New("provider unavailable")
	tests := []struct {
		name     string
		provider *scriptedReviewer
		wantErr  error
		field    string
	}{
		{
			name: "second provider error",
			provider: &scriptedReviewer{
				payloads: []reviewer.Payload{{Summary: "first"}, {Summary: "unused"}},
				errors:   []error{nil, wantProviderErr},
			},
			wantErr: wantProviderErr,
		},
		{
			name: "second invalid payload",
			provider: &scriptedReviewer{payloads: []reviewer.Payload{
				{Summary: "first"},
				{Summary: "second", Comments: []reviewer.Comment{{File: "missing.go", Severity: reviewer.SeverityLow, Message: "note"}}},
			}},
			field: "comments[0].file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "result.json")
			const existing = "do not replace"
			if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
				t.Fatal(err)
			}
			configs := []reviewer.Config{{Name: "first"}, {Name: "second"}, {Name: "never"}}
			coordinator := review.NewCoordinator(&staticDiff{content: "diff --git a/real.go b/real.go"}, test.provider, configs, &sink.File{Path: path})
			err := coordinator.Run()
			if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || test.field != "" && !strings.Contains(err.Error(), test.field) {
				t.Fatalf("Run() error = %v", err)
			}
			if !strings.Contains(err.Error(), `reviewer "second"`) || len(test.provider.calls) != 2 {
				t.Fatalf("failure attribution/calls = %v / %d", err, len(test.provider.calls))
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil || string(data) != existing {
				t.Fatalf("output = %q, error = %v", data, readErr)
			}
		})
	}
}
