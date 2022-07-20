package upload

import (
	"context"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Input struct {
	Builds []ImageBuild `json:"builds"`
}

type Output struct {
	Builds []ImageBuild `json:"builds"`
}

type ImageBuild struct {
	Target    string                            `json:"target"`
	Tag       string                            `json:"tag"`
	Succeeded buildv1beta1.ImageConditionStatus `json:"succeeded,omitempty"`
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
	input := getInput(u.opt.ImageTarget, image.Status.Conditions)
	panic(input)
}

func getInput(target string, conditions []buildv1beta1.ImageCondition) Input {
	builds := []ImageBuild{}
	for _, cond := range conditions {
		if cond.Type == buildv1beta1.ImageConditionTypeUploaded && cond.Status != buildv1beta1.ImageConditionStatusTrue {
			builds = append(builds, ImageBuild{Tag: cond.ResolvedRevision, Target: target})
		}
	}
	return Input{Builds: builds}
}
