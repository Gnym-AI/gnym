package reviewer

type Stub struct{}

func (s *Stub) Review(request Request) (Result, error) {
	return Result{
		Reviewer: request.Config.Name,
		Summary:  "Stub reivew completed",
		Comments: []Comment{
			{
				File:     "stub.go",
				Line:     1,
				Severity: SeverityWarning,
				Message:  "This is a stub review",
			},
		},
	}, nil
}
