package input

import (
	"gnym/review"
	"os"
)

type FileDiffSource struct {
	Path string
}

func (f *FileDiffSource) GetDiff() (review.Diff, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return review.Diff{}, err
	}

	return review.Diff{
		Content: string(data),
	}, nil
}
