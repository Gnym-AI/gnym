package review

import "gnym/reviewer"

type Diff struct {
	Content string
}

type Run struct {
	Results []reviewer.Result
}
