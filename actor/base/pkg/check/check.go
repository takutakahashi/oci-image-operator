package check

import (
	"context"
	"io/ioutil"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheckFile struct {
	Revisions []Revision
}

type Revision struct {
	Revision         string
	ResolvedRevision string
}

type Check struct {
	c   client.Client
	ch  chan bool
	opt CheckOpt
}

type CheckOpt struct {
	ImageName      string
	ImageNamespace string
}

func Init(cfg *rest.Config, opt CheckOpt) (*Check, error) {
	if cfg == nil {
		cfg = ctrl.GetConfigOrDie()
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Check{
		c:   c,
		opt: opt,
	}, nil
}

func (c *Check) Run(ctx context.Context) error {
	image, err := base.GetImage(ctx, c.c, c.opt.ImageName, c.opt.ImageNamespace)
	if err != nil {
		return err
	}
	if !c.ActorInputExists() {
		return parseJSON()
	}
	if !c.ActorOutputExists() {
		return nil
	}
	return nil
}

func (c *Check) ActorInputExists() bool {
	return fileExists("/tmp/actor-base/input")
}
func (c *Check) ActorOutputExists() bool {
	return fileExists("/tmp/actor-base/output")
}
func fileExists(filename string) bool {
	_, err := ioutil.ReadFile(filename)
	return err == nil
}

func parseJSON(image *buildv1beta1.Image) error {
	f := GetCheckFile(image)
}

func GetCheckFile(image *buildv1beta1.Image) CheckFile {
	prs := []Revision{}
	for _, c := range image.Status.Conditions {
		if c.TagPolicy == buildv1beta1.ImageTagPolicyTypeBranchHash {
			prs = append(prs, Revision{Revision: c.Revision, ResolvedRevision: c.ResolvedRevision})

		}
	}
	return CheckFile{
		Revisions: prs,
	}
}
