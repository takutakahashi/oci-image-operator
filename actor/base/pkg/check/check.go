package check

import (
	"context"
	"io"
	"io/ioutil"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	imageutil "github.com/takutakahashi/oci-image-operator/pkg/image"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheckInput struct {
	Revisions []Revision `json:"revisions"`
}

type CheckOutput struct {
	Revisions []Revision `json:"revisions"`
}

type Revision struct {
	Registry         string                            `json:"registry"`
	ResolvedRevision string                            `json:"resolved_revision"`
	Exist            buildv1beta1.ImageConditionStatus `json:"exist"`
}

type Check struct {
	c   client.Client
	ch  chan bool
	opt CheckOpt
	in  io.Writer
	out io.Reader
}

type CheckOpt struct {
	ImageName      string
	ImageNamespace string
	ImageTarget    string
}

func (c CheckInput) Export(w io.Writer) error {
	return base.ParseJSON(&c, w)
}

func ImportOutput(r io.Reader) (CheckOutput, error) {
	c := CheckOutput{}
	err := base.MarshalJSON(&c, r)
	return c, err
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
		conds := imageutil.GetCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeDetected)
		return GetCheckInput(c.opt.ImageTarget, conds).Export(c.in)
	}
	if !c.ActorOutputExists() {
		output, err := ImportOutput(c.out)
		if err != nil {
			return err
		}
		if err := c.UpdateImage(ctx, image, output); err != nil {
			return err
		}
	}
	return nil
}

func (c *Check) UpdateImage(ctx context.Context, image *buildv1beta1.Image, output CheckOutput) error {
	for _, rev := range output.Revisions {
		for i, c := range image.Status.Conditions {
			if c.ResolvedRevision == rev.ResolvedRevision {
				image.Status.Conditions[i].Status = rev.Exist
			}
		}
	}
	return nil
}

func (c *Check) ActorInputExists() bool {
	return fileExists(imageutil.InWorkDir("input"))
}
func (c *Check) ActorOutputExists() bool {
	return fileExists(imageutil.InWorkDir("output"))
}
func fileExists(filename string) bool {
	//FIXME: waste of memory
	_, err := ioutil.ReadFile(filename)
	return err == nil
}

func GetCheckInput(registry string, conds []buildv1beta1.ImageCondition) CheckInput {
	prs := []Revision{}
	for _, c := range conds {
		prs = append(prs, Revision{Registry: registry, ResolvedRevision: c.ResolvedRevision})
	}
	return CheckInput{
		Revisions: prs,
	}
}

func GetCheckOutput() CheckOutput {
	return CheckOutput{Revisions: []Revision{}}
}
