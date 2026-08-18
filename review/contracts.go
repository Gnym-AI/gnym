package review

type DiffSource interface {
	GetDiff() (Diff, error)
}

type Reviewer interface {
	Review(diff Diff, config ReviewerConfig) (ReviewerResult, error)
}

type CommentSink interface {
	Save(run ReviewersRun) error
}
