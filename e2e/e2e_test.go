package e2e

import (
	"context"
	"testing"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_E2E(t *testing.T) {
	ctx := context.Background()
	c := prepare(ctx)
	_ = c
}

func prepare(ctx context.Context) client.Client {
	c, err := base.GenClient(ctrl.GetConfigOrDie())
	if err != nil {
		panic(err)
	}
	nlist := &corev1.NamespaceList{}
	if err := c.List(ctx, nlist); err != nil {
		panic(err)
	}
	for _, n := range nlist.Items {
		if n.Name == "oci-image-operator-system" {
			if _, ok := n.Labels["build.takutakahashi.dev/for-e2e-testing"]; ok {
				return c
			} else {
				panic("this env is not for testing")
			}

		}
	}
	if err := c.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oci-image-operator-system",
			Labels: map[string]string{
				"build.takutakahashi.dev/for-e2e-testing": "yes",
			},
		},
	}); err != nil {
		panic(err)
	}

	return c
}

func newImage(name string) *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageSpec{
			TemplateName: "test",
			Repository: buildv1beta1.ImageRepository{
				URL: "https://github.com/taktuakahashi/testbed.git",
				TagPolicies: []buildv1beta1.ImageTagPolicy{
					{
						Policy:   buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision: "main",
					},
					{
						Policy:   buildv1beta1.ImageTagPolicyTypeTagHash,
						Revision: "latest",
					},
				},
			},
			Targets: []buildv1beta1.ImageTarget{
				{
					Name: "ghcr.io/takutakahashi/test",
					Auth: buildv1beta1.ImageAuth{
						Type:       buildv1beta1.ImageAuthTypeBasic,
						SecretName: "test",
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "DEF_IMAGE_VALUE",
					Value: "image",
				},
				{
					Name: "DEF_IMAGE_CM",
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "cm",
							},
							Key:      "cm",
							Optional: pointer.Bool(false),
						},
					},
				},
			},
		},
	}
}
