package review

import (
	"errors"
	"fmt"
	"gnym/reviewer"
	"time"
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

	files := reviewer.DiffFiles(diff.Content)
	results := make([]reviewer.Result, 0, len(c.reviewers))
	for _, config := range c.reviewers {
		request := reviewer.Request{Diff: diff.Content, Config: config}
		payload, err := c.reviewer.Review(request)
		if err != nil {
			return fmt.Errorf("reviewer %q: %w", config.Name, err)
		}
		result, err := reviewer.Accept(config.Name, payload, files, time.Now())
		if err != nil {
			return err
		}

		results = append(results, result)
	}

	run := Run{
		SchemaVersion: "1",
		Status:        StatusComplete,
		Results:       results,
		Failures:      []Failure{},
	}
	if err := run.Validate(); err != nil {
		return err
	}

	return c.sink.Save(run)
}
