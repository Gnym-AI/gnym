package review

import "gnym/reviewer"

type DiffSource interface {
	GetDiff() (Diff, error)
}

type Reviewer interface {
	Review(request reviewer.Request) (reviewer.Payload, error)
}

type CommentSink interface {
	Save(run Run) error
}
