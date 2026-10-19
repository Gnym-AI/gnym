package reviewer

type Stub struct{}

func (s *Stub) Review(request Request) (Payload, error) {
	return Payload{
		Summary:  "Stub review completed",
		Comments: []Comment{},
	}, nil
}
