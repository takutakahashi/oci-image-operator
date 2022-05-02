package detect

import (
	"os"
	"path/filepath"

	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

type Detect struct {
	gh             *github.Github
	outputFilePath string
}

func NewDetect(outputFilePath string) (*Detect, error) {
	opt, err := github.GenOpt(nil)
	if err != nil {
		return nil, err
	}
	gh, err := github.Init(opt)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Base(outputFilePath), 0644); err != nil {
		return nil, err
	}
	return &Detect{gh: gh, outputFilePath: outputFilePath}, nil
}
