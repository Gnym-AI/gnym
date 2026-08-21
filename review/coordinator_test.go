package review

import (
	"errors"
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
	diff   Diff
	config ReviewerConfig
}

type fakeReviewer struct {
	results []ReviewerResult
	errors  []error
	calls   []reviewCall
}

func (f *fakeReviewer) Review(diff Diff, config ReviewerConfig) (ReviewerResult, error) {
	f.calls = append(f.calls, reviewCall{diff: diff, config: config})
	callIndex := len(f.calls) - 1

	var result ReviewerResult
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
	runs []ReviewersRun
	err  error
}

func (f *fakeCommentSink) Save(run ReviewersRun) error {
	f.runs = append(f.runs, run)
	return f.err
}

func TestCoordinatorRun(t *testing.T) {
	diff := Diff{Content: "diff --git a/main.go b/main.go"}
	configs := []ReviewerConfig{
		{Name: "correctness", Provider: "openai", Model: "model-a", Prompt: "Find bugs"},
		{Name: "security", Provider: "openai", Model: "model-b", Prompt: "Find vulnerabilities"},
	}
	results := []ReviewerResult{
		{
			Reviewer: "correctness",
			Summary:  "One issue",
			Comments: []Comment{{
				File: "main.go", Line: 10, Severity: "warning", Message: "Check this error",
			}},
		},
		{Reviewer: "security", Summary: "No issues"},
	}

	diffSource := &fakeDiffSource{diff: diff}
	reviewer := &fakeReviewer{results: results}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, reviewer, configs, sink)

	if err := coordinator.Run(); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if diffSource.calls != 1 {
		t.Errorf("DiffSource.GetDiff() calls = %d, want 1", diffSource.calls)
	}

	wantReviewCalls := []reviewCall{
		{diff: diff, config: configs[0]},
		{diff: diff, config: configs[1]},
	}
	if !reflect.DeepEqual(reviewer.calls, wantReviewCalls) {
		t.Errorf("Reviewer.Review() calls = %#v, want %#v", reviewer.calls, wantReviewCalls)
	}

	wantRuns := []ReviewersRun{{Results: results}}
	if !reflect.DeepEqual(sink.runs, wantRuns) {
		t.Errorf("CommentSink.Save() runs = %#v, want %#v", sink.runs, wantRuns)
	}
}

func TestCoordinatorRunDiffSourceErrorStopsExecution(t *testing.T) {
	wantErr := errors.New("could not get diff")
	diffSource := &fakeDiffSource{err: wantErr}
	reviewer := &fakeReviewer{}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(
		diffSource,
		reviewer,
		[]ReviewerConfig{{Name: "correctness"}},
		sink,
	)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	if diffSource.calls != 1 {
		t.Errorf("DiffSource.GetDiff() calls = %d, want 1", diffSource.calls)
	}
	if len(reviewer.calls) != 0 {
		t.Errorf("Reviewer.Review() calls = %d, want 0", len(reviewer.calls))
	}
	if len(sink.runs) != 0 {
		t.Errorf("CommentSink.Save() calls = %d, want 0", len(sink.runs))
	}
}

func TestCoordinatorRunReviewerErrorStopsExecution(t *testing.T) {
	wantErr := errors.New("review failed")
	diff := Diff{Content: "the diff"}
	configs := []ReviewerConfig{
		{Name: "first"},
		{Name: "failing"},
		{Name: "not-called"},
	}
	diffSource := &fakeDiffSource{diff: diff}
	reviewer := &fakeReviewer{
		results: []ReviewerResult{{Reviewer: "first"}, {}},
		errors:  []error{nil, wantErr},
	}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, reviewer, configs, sink)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	wantReviewCalls := []reviewCall{
		{diff: diff, config: configs[0]},
		{diff: diff, config: configs[1]},
	}
	if !reflect.DeepEqual(reviewer.calls, wantReviewCalls) {
		t.Errorf("Reviewer.Review() calls = %#v, want %#v", reviewer.calls, wantReviewCalls)
	}
	if len(sink.runs) != 0 {
		t.Errorf("CommentSink.Save() calls = %d, want 0", len(sink.runs))
	}
}

func TestCoordinatorRunCommentSinkErrorIsReturned(t *testing.T) {
	wantErr := errors.New("could not save comments")
	result := ReviewerResult{Reviewer: "correctness", Summary: "Done"}
	diffSource := &fakeDiffSource{diff: Diff{Content: "the diff"}}
	reviewer := &fakeReviewer{results: []ReviewerResult{result}}
	sink := &fakeCommentSink{err: wantErr}
	coordinator := NewCoordinator(
		diffSource,
		reviewer,
		[]ReviewerConfig{{Name: "correctness"}},
		sink,
	)

	err := coordinator.Run()

	if !errors.Is(err, wantErr) {
		t.Errorf("Run() error = %v, want %v", err, wantErr)
	}
	wantRuns := []ReviewersRun{{Results: []ReviewerResult{result}}}
	if !reflect.DeepEqual(sink.runs, wantRuns) {
		t.Errorf("CommentSink.Save() runs = %#v, want %#v", sink.runs, wantRuns)
	}
}

func TestCoordinatorRunWithZeroReviewers(t *testing.T) {
	diff := Diff{Content: "diff --git a/main.go b/main.go"}
	var configs []ReviewerConfig
	var results []ReviewerResult

	diffSource := &fakeDiffSource{diff: diff}
	reviewer := &fakeReviewer{results: results}
	sink := &fakeCommentSink{}
	coordinator := NewCoordinator(diffSource, reviewer, configs, sink)

	err := coordinator.Run()

	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}

	expectedError := errors.New("no reviewers configured")
	if err.Error() != expectedError.Error() {
		t.Errorf("Run() error = %v, want %v", err, "no reviewers configured")
	}
}
