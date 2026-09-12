package review

import (
	"errors"
	"gnym/reviewer"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeDiffSource struct {
	diff  Diff
	err   error
	calls int
}

func (f *fakeDiffSource) GetDiff() (Diff, error) {
	f.calls++
	return f.diff, f.err
}

type reviewCall struct {
	diff   string
	config reviewer.Config
}

type fakeReviewer struct {
	results []reviewer.Payload
	errors  []error
	calls   []reviewCall
}

func (f *fakeReviewer) Review(request reviewer.Request) (reviewer.Payload, error) {
	f.calls = append(f.calls, reviewCall{diff: request.Diff, config: request.Config})
	callIndex := len(f.calls) - 1

	var result reviewer.Payload
	if callIndex < len(f.results) {
		result = f.results[callIndex]
	}

	var err error
	if callIndex < len(f.errors) {
		err = f.errors[callIndex]
	}

	return result, err
}

type fakeCommentSink struct {
	runs []Run
	err  error
}

func (f *fakeCommentSink) Save(run Run) error {
	f.runs = append(f.runs, run)
	return f.err
}

func TestCoordinatorRun(t *testing.T) {
	diff := Diff{Content: "diff --git a/main.go b/main.go"}
	configs := []reviewer.Config{
		{Name: "correctness", Provider: "openai", Model: "model-a", Prompt: "Find bugs"},
		{Name: "security", Provider: "openai", Model: "model-b", Prompt: "Find vulnerabilities"},
	}
	results := []reviewer.Payload{
		{
			Summary: "One issue",
			Comments: []reviewer.Comment{{
				File: "main.go", Side: "old", Line: 10, EndLine: 9000, Severity: reviewer.SeverityHigh, Message: "Check this error",
			}},
		},
		{Summary: "No issues"},
	}

	diffSource := &fakeDiffSource{diff: diff}
	diffReviewer := &fakeReviewer{results: results}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, diffReviewer, configs, sink)

	before := time.Now()
	if err := coordinator.Run(); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if diffSource.calls != 1 {
		t.Errorf("DiffSource.GetDiff() calls = %d, want 1", diffSource.calls)
	}

	wantReviewCalls := []reviewCall{
		{diff: diff.Content, config: configs[0]},
		{diff: diff.Content, config: configs[1]},
	}
	if !reflect.DeepEqual(diffReviewer.calls, wantReviewCalls) {
		t.Errorf("Reviewer.Review() calls = %#v, want %#v", diffReviewer.calls, wantReviewCalls)
	}

	after := time.Now()
	if len(sink.runs) != 1 {
		t.Fatalf("sink calls = %d, want 1", len(sink.runs))
	}
	run := sink.runs[0]
	if run.SchemaVersion != "1" || run.Status != StatusComplete || run.Failures == nil || len(run.Failures) != 0 || len(run.Results) != 2 {
		t.Fatalf("unexpected run: %#v", run)
	}
	for i, result := range run.Results {
		if result.Reviewer != configs[i].Name || result.Summary != results[i].Summary || result.Comments == nil {
			t.Errorf("accepted result[%d] = %#v", i, result)
		}
		stamp, err := time.Parse(time.RFC3339Nano, result.CreatedAt)
		if err != nil || !strings.HasSuffix(result.CreatedAt, "Z") || stamp.Before(before) || stamp.After(after) {
			t.Errorf("acceptance time %q outside run interval: %v", result.CreatedAt, err)
		}
	}
	if !reflect.DeepEqual(run.Results[0].Comments, results[0].Comments) {
		t.Errorf("advisory location changed: %#v", run.Results[0].Comments)
	}
}

func TestCoordinatorRunDiffSourceErrorStopsExecution(t *testing.T) {
	wantErr := errors.New("could not get diff")
	diffSource := &fakeDiffSource{err: wantErr}
	diffReviewer := &fakeReviewer{}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(
		diffSource,
		diffReviewer,
		[]reviewer.Config{{Name: "correctness"}},
		sink,
	)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	if diffSource.calls != 1 {
		t.Errorf("DiffSource.GetDiff() calls = %d, want 1", diffSource.calls)
	}
	if len(diffReviewer.calls) != 0 {
		t.Errorf("Reviewer.Review() calls = %d, want 0", len(diffReviewer.calls))
	}
	if len(sink.runs) != 0 {
		t.Errorf("CommentSink.Save() calls = %d, want 0", len(sink.runs))
	}
}

func TestCoordinatorRunReviewerErrorStopsExecution(t *testing.T) {
	wantErr := errors.New("review failed")
	diff := Diff{Content: "the diff"}
	configs := []reviewer.Config{
		{Name: "first"},
		{Name: "failing"},
		{Name: "not-called"},
	}
	diffSource := &fakeDiffSource{diff: diff}
	diffReviewer := &fakeReviewer{
		results: []reviewer.Payload{{Summary: "First completed"}, {}},
		errors:  []error{nil, wantErr},
	}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, diffReviewer, configs, sink)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	wantReviewCalls := []reviewCall{
		{diff: diff.Content, config: configs[0]},
		{diff: diff.Content, config: configs[1]},
	}
	if !reflect.DeepEqual(diffReviewer.calls, wantReviewCalls) {
		t.Errorf("Reviewer.Review() calls = %#v, want %#v", diffReviewer.calls, wantReviewCalls)
	}
	if len(sink.runs) != 0 {
		t.Errorf("CommentSink.Save() calls = %d, want 0", len(sink.runs))
	}
}

func TestCoordinatorRunCommentSinkErrorIsReturned(t *testing.T) {
	wantErr := errors.New("could not save comments")
	result := reviewer.Payload{Summary: "Done"}
	diffSource := &fakeDiffSource{diff: Diff{Content: "the diff"}}
	diffReviewer := &fakeReviewer{results: []reviewer.Payload{result}}
	sink := &fakeCommentSink{err: wantErr}
	coordinator := NewCoordinator(
		diffSource,
		diffReviewer,
		[]reviewer.Config{{Name: "correctness"}},
		sink,
	)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	if len(sink.runs) != 1 || len(sink.runs[0].Results) != 1 || sink.runs[0].Results[0].Summary != result.Summary {
		t.Errorf("CommentSink.Save() runs = %#v", sink.runs)
	}
}

func TestCoordinatorRunWithZeroReviewers(t *testing.T) {
	diff := Diff{Content: "diff --git a/main.go b/main.go"}
	var configs []reviewer.Config
	var results []reviewer.Payload

	diffSource := &fakeDiffSource{diff: diff}
	diffReviewer := &fakeReviewer{results: results}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, diffReviewer, configs, sink)

	err := coordinator.Run()

	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}

	expectedError := errors.New("no reviewers configured")
	if err.Error() != expectedError.Error() {
		t.Errorf("Run() error = %v, want %v", err, "no reviewers configured")
	}
	if diffSource.calls != 0 || len(diffReviewer.calls) != 0 || len(sink.runs) != 0 {
		t.Fatal("preflight failure invoked pipeline dependencies")
	}
}

func TestCoordinatorInvalidPayloadStopsBeforeDelivery(t *testing.T) {
	for _, invalidIndex := range []int{0, 1} {
		for _, test := range []struct {
			name    string
			payload reviewer.Payload
			field   string
		}{
			{"blank summary", reviewer.Payload{}, "summary"},
			{"unknown file", reviewer.Payload{Summary: "s", Comments: []reviewer.Comment{{File: "missing", Severity: reviewer.SeverityLow, Message: "m"}}}, "comments[0].file"},
			{"bad line structure", reviewer.Payload{Summary: "s", Comments: []reviewer.Comment{{File: "real", Line: 5, Severity: reviewer.SeverityLow, Message: "m"}}}, "comments[0].side"},
		} {
			t.Run(test.name+string(rune('0'+invalidIndex)), func(t *testing.T) {
				payloads := []reviewer.Payload{{Summary: "first"}, {Summary: "second"}, {Summary: "third"}}
				payloads[invalidIndex] = test.payload
				r := &fakeReviewer{results: payloads}
				sink := &fakeCommentSink{}
				configs := []reviewer.Config{{Name: "first"}, {Name: "second"}, {Name: "third"}}
				c := NewCoordinator(&fakeDiffSource{diff: Diff{Content: "diff --git a/real b/real"}}, r, configs, sink)
				err := c.Run()
				if err == nil || !strings.Contains(err.Error(), configs[invalidIndex].Name) || !strings.Contains(err.Error(), test.field) {
					t.Fatalf("validation diagnostic = %v", err)
				}
				if len(r.calls) != invalidIndex+1 || len(sink.runs) != 0 {
					t.Fatalf("review calls = %d, sink calls = %d", len(r.calls), len(sink.runs))
				}
			})
		}
	}
}
