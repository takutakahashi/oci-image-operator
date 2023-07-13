package check

import (
	"context"
	"io"
	"os"

	"github.com/k0kubun/pp"
	"github.com/sirupsen/logrus"
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
	Revision         string                            `json:"revision"`
	Exist            buildv1beta1.ImageConditionStatus `json:"exist"`
}

type Check struct {
	c     client.Client
	image *buildv1beta1.Image
	opt   CheckOpt
}

type CheckOpt struct {
	ImageName      string
	ImageNamespace string
	ImageTarget    string
}

func (c CheckInput) Export(w io.Writer) error {
	if w == nil {
		var err error
		w, err = os.Create(base.InWorkDir("input"))
		if err != nil {
			return err
		}
	}
	logrus.Info("==== export input file ====")
	pp.Println(c)
	return base.ParseJSON(&c, w)
}

func ImportOutput(r io.Reader) (CheckOutput, error) {
	if r == nil {
		var err error
		r, err = os.Open(base.InWorkDir("output"))
		if err != nil {
			return CheckOutput{}, err
		}
	}
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

func (c *Check) GetImage(ctx context.Context) (*buildv1beta1.Image, error) {
	if c.image != nil {
		return c.image, nil
	}
	image, err := base.GetImage(ctx, c.c, c.opt.ImageName, c.opt.ImageNamespace)
	if err != nil {
		return nil, err
	}
	c.image = image
	return image, nil
}

func (c *Check) GetInput(ctx context.Context) (*CheckInput, error) {
	image, err := c.GetImage(ctx)
	if err != nil {
		return nil, err
	}
	conds := imageutil.GetConditionByStatus(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked, buildv1beta1.ImageConditionStatusFalse)
	input := GetCheckInput(c.opt.ImageTarget, conds)
	return &input, nil
}

func (c *Check) UpdateImage(ctx context.Context, image *buildv1beta1.Image, output CheckOutput) error {
	logrus.Info("==== output ====")
	pp.Println(output)
	for _, rev := range output.Revisions {
		image.Status.Conditions = imageutil.UpdateCheckedCondition(
			image.Status.Conditions,
			buildv1beta1.ImageConditionStatusTrue,
			rev.Revision,
			rev.ResolvedRevision,
		)
		image.Status.Conditions = imageutil.UpdateUploadedCondition(
			image.Status.Conditions,
			rev.Exist,
			rev.Revision,
			rev.ResolvedRevision,
		)
	}
	return c.c.Status().Update(ctx, image, &client.UpdateOptions{})
}

func GetCheckInput(registry string, conds []buildv1beta1.ImageCondition) CheckInput {
	prs := []Revision{}
	for _, c := range conds {
		prs = append(prs, Revision{Registry: registry, ResolvedRevision: c.ResolvedRevision, Revision: c.Revision})
	}
	return CheckInput{
		Revisions: prs,
	}
}

func GetCheckOutput() CheckOutput {
	return CheckOutput{Revisions: []Revision{}}
}
