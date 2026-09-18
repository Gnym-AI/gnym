package review

import (
	"errors"
	"gnym/reviewer"
	"reflect"
	"testing"
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
	results []reviewer.Result
	errors  []error
	calls   []reviewCall
}

func (f *fakeReviewer) Review(request reviewer.Request) (reviewer.Result, error) {
	f.calls = append(f.calls, reviewCall{diff: request.Diff, config: request.Config})
	callIndex := len(f.calls) - 1

	var result reviewer.Result
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
	results := []reviewer.Result{
		{
			Reviewer: "correctness",
			Summary:  "One issue",
			Comments: []reviewer.Comment{{
				File: "main.go", Line: 10, Severity: "warning", Message: "Check this error",
			}},
		},
		{Reviewer: "security", Summary: "No issues"},
	}

	diffSource := &fakeDiffSource{diff: diff}
	diffReviewer := &fakeReviewer{results: results}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, diffReviewer, configs, sink)

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

	wantRuns := []Run{{Results: results}}
	if !reflect.DeepEqual(sink.runs, wantRuns) {
		t.Errorf("CommentSink.Save() runs = %#v, want %#v", sink.runs, wantRuns)
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
		results: []reviewer.Result{{Reviewer: "first"}, {}},
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
	result := reviewer.Result{Reviewer: "correctness", Summary: "Done"}
	diffSource := &fakeDiffSource{diff: Diff{Content: "the diff"}}
	diffReviewer := &fakeReviewer{results: []reviewer.Result{result}}
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
	wantRuns := []Run{{Results: []reviewer.Result{result}}}
	if !reflect.DeepEqual(sink.runs, wantRuns) {
		t.Errorf("CommentSink.Save() runs = %#v, want %#v", sink.runs, wantRuns)
	}
}

func TestCoordinatorRunWithZeroReviewers(t *testing.T) {
	diff := Diff{Content: "diff --git a/main.go b/main.go"}
	var configs []reviewer.Config
	var results []reviewer.Result

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
}
