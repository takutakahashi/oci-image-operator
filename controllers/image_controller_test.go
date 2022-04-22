package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Image controller", func() {
	//! [test]
	It("should success", func() {
		ctx := context.TODO()
		err := k8sClient.Create(ctx, newImage(), &client.CreateOptions{})
		Expect(err).To(Succeed())
	})
	//! [test]
})

func newImage() *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageSpec{
			TemplateName: "",
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
					Auth: buildv1beta1.ImageAuth{
						Type:       buildv1beta1.ImageAuthTypeBasic,
						SecretName: "test",
					},
				},
			},
		},
	}
}
