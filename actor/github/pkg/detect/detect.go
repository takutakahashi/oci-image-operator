package detect

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

type Detect struct {
	gh   *github.Github
	base *detect.Detect
}

func NewDetect(base *detect.Detect) (*Detect, error) {
	opt, err := github.GenOpt(nil)
	if err != nil {
		return nil, err
	}
	gh, err := github.Init(opt)
	if err != nil {
		return nil, err
	}
	return &Detect{gh: gh, base: base}, nil
}

func (d *Detect) Run() error {
	for {
		time.Sleep(1 * time.Minute)
		if err := d.Execute(); err != nil {
			logrus.Error(err)
			continue
		}
	}
}
func (d *Detect) Output(ctx context.Context) (*detect.DetectFile, error) {
	branches, err := d.gh.BranchHash(ctx)
	if err != nil {
		logrus.Error("error while getting branches")
		return nil, err
	}
	tags, err := d.gh.TagHash(ctx)
	if err != nil {
		logrus.Error("error while getting tags")
		return nil, err
	}
	df := &detect.DetectFile{
		Branches: branches,
		Tags:     tags,
	}
	return df, nil

}
func (d *Detect) Execute() error {
	ctx := context.TODO()
	df, err := d.Output(ctx)
	if err != nil {
		return err
	}
	_, err = d.base.UpdateImage(ctx, df)
	return err
}
