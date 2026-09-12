package review_test

import (
	"fmt"
	"gnym/review"
	"gnym/reviewer"
	"testing"
)

func TestIndependentRunStatusCardinalities(t *testing.T) {
	for _, status := range []review.Status{review.StatusComplete, review.StatusPartial, review.StatusFailed} {
		for _, reviews := range []int{0, 1, 2} {
			for _, failures := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("%s/%d reviews/%d failures", status, reviews, failures), func(t *testing.T) {
					r := review.Run{SchemaVersion: "1", Status: status}
					for i := 0; i < reviews; i++ {
						r.Results = append(r.Results, reviewer.Result{Reviewer: fmt.Sprintf("r%d", i), Summary: "s", CreatedAt: "2026-09-12T12:00:00Z"})
					}
					for i := 0; i < failures; i++ {
						r.Failures = append(r.Failures, review.Failure{Reviewer: fmt.Sprintf("f%d", i), Code: review.FailureReviewerError, Message: "m"})
					}
					wantValid := false
					switch status {
					case review.StatusComplete:
						wantValid = reviews != 0 && failures == 0
					case review.StatusPartial:
						wantValid = reviews != 0 && failures != 0
					case review.StatusFailed:
						wantValid = reviews == 0 && failures != 0
					}
					if err := r.Validate(); (err == nil) != wantValid {
						t.Fatalf("Validate() got %v, want valid %v", err, wantValid)
					}
				})
			}
		}
	}
}
