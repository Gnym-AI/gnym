package review

import (
	"errors"
	"gnym/reviewer"
)

type Coordinator struct {
	diffSource DiffSource
	reviewer   Reviewer
	reviewers  []reviewer.Config
	sink       CommentSink
}

func (c *Coordinator) validateConfiguration() error {
	if len(c.reviewers) == 0 {
		return errors.New("no reviewers configured")
	}
	return nil
}

func NewCoordinator(
	diffSource DiffSource,
	reviewer Reviewer,
	reviewers []reviewer.Config,
	sink CommentSink,
) *Coordinator {
	return &Coordinator{
		diffSource: diffSource,
		reviewer:   reviewer,
		reviewers:  reviewers,
		sink:       sink,
	}
}

func (c *Coordinator) Run() error {
	err := c.validateConfiguration()
	if err != nil {
		return err
	}

	diff, err := c.diffSource.GetDiff()
	if err != nil {
		return err
	}

	results := make([]reviewer.Result, 0, len(c.reviewers))
	for _, config := range c.reviewers {
		request := reviewer.Request{diff.Content, config}
		result, err := c.reviewer.Review(request)
		if err != nil {
			return err
		}

		results = append(results, result)
	}

	run := Run{
		Results: results,
	}

	return c.sink.Save(run)
}
