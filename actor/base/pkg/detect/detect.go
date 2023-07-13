package detect

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/k0kubun/pp"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	imageutil "github.com/takutakahashi/oci-image-operator/pkg/image"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Detect struct {
	c   client.Client
	ch  chan bool
	opt DetectOpt
}

type DetectOpt struct {
	ImageName      string
	ImageNamespace string
}

type DetectFile struct {
	Branches map[string]string `json:"branches"`
	Tags     map[string]string `json:"tags"`
}

const (
	MapKeyLatestTagHash = "latest/hash"
	MapKeyLatestTagName = "latest/name"
)

func Init(cfg *rest.Config, opt DetectOpt) (*Detect, error) {
	if cfg == nil {
		cfg = ctrl.GetConfigOrDie()
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Detect{
		c:   c,
		opt: opt,
	}, nil
}

func (d *Detect) Stop() {
	logrus.Info("Stopping worker")
	d.ch <- true
}

func (d *Detect) UpdateImage(ctx context.Context, detectFile *DetectFile) (*buildv1beta1.Image, error) {
	logrus.Infof("detected struct: %v", detectFile)
	image := buildv1beta1.Image{}
	nn := ktypes.NamespacedName{
		Namespace: d.opt.ImageNamespace,
		Name:      d.opt.ImageName,
	}
	if err := d.c.Get(ctx, nn, &image); err != nil {
		return nil, err
	}

	newImage := image.DeepCopy()
	newConditions := ensureConditions(newImage.Status.Conditions, detectFile)
	diff := cmp.Diff(image.Status.Conditions, newConditions, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime"))
	logrus.Infof("diff: %s", diff)
	if diff != "" {
		newImage.Status.Conditions = newConditions
		if err := d.c.Status().Update(ctx, newImage); err != nil {
			return nil, err
		}
		logrus.Info("image updated")
	}
	logrus.Info("image ensured")
	return newImage, nil
}

func ensureConditions(conditions []buildv1beta1.ImageCondition, detectFile *DetectFile) []buildv1beta1.ImageCondition {
	pp.Println(conditions)
	for branch, resolvedRevision := range detectFile.Branches {
		if resolvedRevision == "" {
			continue

		}
		conds := imageutil.GetConditionByStatus(conditions, buildv1beta1.ImageConditionTypeChecked, buildv1beta1.ImageConditionStatusTrue)
		checked := buildv1beta1.ImageConditionStatusFalse
		for _, cond := range conds {
			if cond.ResolvedRevision == resolvedRevision {
				checked = cond.Status
			}
		}
		conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeChecked, &checked,
			buildv1beta1.ImageTagPolicyTypeBranchHash, branch, resolvedRevision)
	}
	for key, resolvedRevision := range detectFile.Tags {
		if resolvedRevision == "" {
			continue
		}
		if key == MapKeyLatestTagName || key == MapKeyLatestTagHash {
			conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeChecked, &buildv1beta1.ImageConditionStatusFalse,
				buildv1beta1.ImageTagPolicyTypeTagHash, "latest", resolvedRevision)
		}
	}
	pp.Println("-----------  before and after ------------")
	pp.Println(conditions)
	return conditions
}
