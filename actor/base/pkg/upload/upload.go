package upload

import (
	"context"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Input struct {
	Name string
	Tag  string
}

type Output struct {
	Results []Result
}

type Result struct {
	Target    string `json:"target"`
	Tag       string `json:"tag"`
	Succeeded buildv1beta1.ImageConditionStatus
}

type Opt struct {
	ImageName      string
	ImageNamespace string
	ImageTarget    string
}

type Upload struct {
	c   client.Client
	ch  chan bool
	opt Opt
}

func (u *Upload) Execute(ctx context.Context) error {
	image, err := base.GetImage(ctx, u.c, u.opt.ImageName, u.opt.ImageNamespace)
	if err != nil {
		return err
	}
	inputs := getImagesNeedToBeBuilt(image.Status.Conditions)
	panic(inputs)
}

func getImagesNeedToBeBuilt(conditions []buildv1beta1.ImageCondition) []Input {
	ret := []Input{}
	return ret
}
