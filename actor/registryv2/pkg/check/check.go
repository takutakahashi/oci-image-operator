package check

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/check"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/external"
	"github.com/takutakahashi/oci-image-operator/actor/registryv2/pkg/registryv2"
	"github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

type Check struct {
	r *registryv2.Registry
}

func Init(r *registryv2.Registry) (Check, error) {
	return Check{r: r}, nil
}

func (c Check) Run() error {
	ctx := context.Background()
	for {
		time.Sleep(5 * time.Second)
		if err := c.Execute(ctx); err != nil {
			logrus.Error("error occured while executing check")
			logrus.Error(err)
		} else {
			logrus.Info("execute succeeded. exit process.")
			break
		}
	}
	return nil
}

func (c Check) Execute(ctx context.Context) error {
	input, err := external.LoadCheckInput(nil)
	if err != nil {
		return err
	}
	out, err := c.Output(&input)
	if err != nil {
		return err
	}
	return external.ExportCheckOutput(out, nil)
}

func (c Check) Output(in *check.CheckInput) (check.CheckOutput, error) {
	revs := []check.Revision{}
	for _, rev := range in.Revisions {
		exist, err := c.r.TagExists(rev.ResolvedRevision)
		if err != nil {
			logrus.Error(err)
			exist = false
		}
		rev.Exist = parseExist(exist)
		revs = append(revs, rev)
	}
	return check.CheckOutput{Revisions: revs}, nil
}

func parseExist(b bool) v1beta1.ImageConditionStatus {
	if b {
		return v1beta1.ImageConditionStatusTrue
	} else {
		return v1beta1.ImageConditionStatusFalse
	}
}
