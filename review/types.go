package review

type Diff struct {
	Content string
}

type ReviewerConfig struct {
	Name     string
	Provider string
	Model    string
	Prompt   string
}

type Comment struct {
	File     string
	Line     int
	Severity string
	Message  string
}

type ReviewerResult struct {
	Reviewer string
	Summary  string
	Comments []Comment
}

type ReviewersRun struct {
	Results []ReviewerResult
}
