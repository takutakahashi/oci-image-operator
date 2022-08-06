package detect

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

type Detect struct {
	gh *github.Github
	w  io.Writer
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
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(outputFilePath)
	if err != nil {
		return nil, err
	}
	return &Detect{gh: gh, w: f}, nil
}

func (d *Detect) Run() error {
	for {
		time.Sleep(1 * time.Minute)
		if err := d.Execute(); err != nil {
			logrus.Error(err)
			continue
		}
		if _, err := http.Get("http://localhost:8080/"); err != nil {
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
	buf, err := json.Marshal(&df)
	if err != nil {
		return err
	}
	_, err = d.w.Write(buf)
	return err

}
