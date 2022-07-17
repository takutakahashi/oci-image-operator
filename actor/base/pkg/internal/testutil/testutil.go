package testutil

import (
	"context"
	"os"
	"path/filepath"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func Setup(image *buildv1beta1.Image) (client.Client, func() error) {
	os.Setenv("IMAGE_NAME", "test")
	os.Setenv("IMAGE_NAMESPACE", "default")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../../../..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		panic(err)
	}
	ctx := context.TODO()
	if image != nil {
		err = c.Create(ctx, NewImage())
		if err != nil {
			panic(err)
		}
	}
	return c, testEnv.Stop
}

func NewImage() *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageSpec{
			TemplateName: "test",
			Repository: buildv1beta1.ImageRepository{
				URL: "https://github.com/taktuakahashi/testbed.git",
				TagPolicies: []buildv1beta1.ImageTagPolicy{
					{
						Policy:   buildv1beta1.ImageTagPolicyTypeTagHash,
						Revision: "master",
					},
				},
			},
			Targets: []buildv1beta1.ImageTarget{
				{
					Name: "ghcr.io/takutakahashi/test",
					//Auth: buildv1beta1.ImageAuth{
					//	Type:       buildv1beta1.ImageAuthTypeBasic,
					//	SecretName: "test",
					//},
				},
			},
		},
	}
}
