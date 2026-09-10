package reviewer

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Config struct {
	Name     string
	Provider string
	Model    string
	Prompt   string
}

type Request struct {
	Diff   string
	Config Config
}

type Comment struct {
	File     string
	Line     int
	Severity Severity
	Message  string
}

type Result struct {
	Reviewer string
	Summary  string
	Comments []Comment
}
