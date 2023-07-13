package detect

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

type Detect struct {
	gh   *github.Github
	w    io.Writer
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

func (d *Detect) Execute() error {
	ctx := context.TODO()
	branches, err := d.gh.BranchHash(ctx)
	if err != nil {
		logrus.Error("error while getting branches")
		return err
	}
	tags, err := d.gh.TagHash(ctx)
	if err != nil {
		logrus.Error("error while getting tags")
		return err
	}
	df := detect.DetectFile{
		Branches: branches,
		Tags:     tags,
	}
	_, err = d.base.UpdateImage(ctx, &df)
	return err
}
