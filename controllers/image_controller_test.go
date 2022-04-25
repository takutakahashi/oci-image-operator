package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Image controller", func() {
	var _ = BeforeEach(func() {
		ctx := context.TODO()
		err := k8sClient.DeleteAllOf(ctx, &buildv1beta1.ImageFlowTemplate{}, client.InNamespace("default"))
		Expect(err).To(Succeed())
		err = k8sClient.Create(ctx, newImageFlowTemplate("test"), &client.CreateOptions{})
		Expect(err).To(Succeed())
	})
	//! [test]
	Describe("create", func() {
		It("should success", func() {
			ctx := context.TODO()
			image := newImage()
			inClusterImage := &buildv1beta1.Image{}
			objKey := types.NamespacedName{Name: image.Name, Namespace: image.Namespace}
			err := k8sClient.Create(ctx, image, &client.CreateOptions{})
			Expect(err).To(Succeed())
			Eventually(func() error {
				if err := k8sClient.Get(ctx, objKey, inClusterImage); err != nil {
					return err
				}
				deploy := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-detect", objKey.Name), Namespace: objKey.Namespace},
					deploy); err != nil {
					return err
				}
				if d := cmp.Diff(deploy.Spec.Template.Spec.Containers[0].Command, []string{"/entrypoint", "detect"}); d != "" {
					return fmt.Errorf("diff detected. %s", d)
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-check", objKey.Name), Namespace: objKey.Namespace},
					&batchv1.Job{}); err != nil {
					return err
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-upload", objKey.Name), Namespace: objKey.Namespace},
					&batchv1.Job{}); err != nil {
					return err
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
		})
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

func newImageFlowTemplate(name string) *buildv1beta1.ImageFlowTemplate {
	podTemplate := corev1apply.
		PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithContainers(corev1apply.Container().
			WithName("main").WithImage("busybox")))
	var template *buildv1beta1.PodTemplateSpecApplyConfiguration = (*buildv1beta1.PodTemplateSpecApplyConfiguration)(podTemplate)
	tmp := buildv1beta1.ImageFlowTemplateSpecTemplate{
		PodTemplate: template,
	}
	return &buildv1beta1.ImageFlowTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageFlowTemplateSpec{
			Detect: tmp,
			Check:  tmp,
			Upload: tmp,
		},
	}
}
