package upload

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/external"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

type Upload struct {
	gh  *github.Github
	in  io.Reader
	out io.Writer
}

func Init() (*Upload, error) {
	opt, err := github.GenOpt(nil)
	if err != nil {
		return nil, err
	}
	gh, err := github.Init(opt)
	if err != nil {
		return nil, err
	}
	return &Upload{gh: gh}, nil
}

func (u Upload) Run(ctx context.Context) error {
	for {
		time.Sleep(10 * time.Second)
		if err := u.Execute(ctx); err != nil {
			logrus.Error(err)
			continue
		} else {
			break
		}
	}
	return nil
}

func (u Upload) Execute(ctx context.Context) error {
	input, err := external.LoadUploadInput(u.in)
	if err != nil {
		return err
	}
	output, err := u.Output(ctx, input)
	if err != nil {
		return err
	}
	return external.ExportUploadExport(output, u.out)
}

func (u Upload) Output(ctx context.Context, input upload.Input) (upload.Output, error) {
	out := upload.Output{
		Builds: []upload.ImageBuild{},
	}
	// 1. trigger action
	// 2. wait for action result
	// 3. output if actions is succeeded
	for _, build := range input.Builds {
		u.gh.Dispatch(ctx, build.Tag)
	}
	return out, nil
}
