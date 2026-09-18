package sink

import (
	"encoding/json"
	"gnym/review"
	"os"
)

type File struct {
	Path string
}

func (f *File) Save(run review.Run) error {
	data, err := json.MarshalIndent(run.Results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.Path, data, 0644)
}
