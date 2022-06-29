package controllers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/strings/slices"
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
		It("detect should success", func() {
			ctx := context.TODO()
			image := newImage("test-detect")
			inClusterImage := &buildv1beta1.Image{}
			objKey := types.NamespacedName{Name: image.Name, Namespace: image.Namespace}
			err := k8sClient.Create(ctx, image, &client.CreateOptions{})
			Expect(err).To(Succeed())
			Eventually(func() error {
				if err := k8sClient.Get(ctx, objKey, inClusterImage); err != nil {
					return err
				}
				deploy := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-detect", objKey.Name), Namespace: "oci-image-operator-system"},
					deploy); err != nil {
					return err
				}
				if deploy.Spec.Template.Spec.ServiceAccountName != "oci-image-operator-actor-detect" {
					return errors.New("wrong service account name")
				}
				for _, c := range deploy.Spec.Template.Spec.Containers {
					if c.Name == "main" {
						if d := cmp.Diff(c.Command, []string{"/entrypoint", "detect"}); d != "" {
							return fmt.Errorf("diff detected. %s", d)
						}

						contained := []string{}
						required := []string{"AUTH_SECRET_NAME", "REPOSITORY", "TEST_ENV"}
						for _, e := range c.Env {
							if slices.Contains(required, e.Name) {
								contained = append(contained, e.Name)
							}
						}
						sort.Slice(contained, func(i, j int) bool { return contained[i] < contained[j] })
						if !cmp.Equal(contained, required) {
							return fmt.Errorf("required env is invalid. %s", contained)
						}
						break
					}
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
		})

		It("check should success", func() {
			ctx := context.TODO()
			image := newImage("test-check")
			inClusterImage := &buildv1beta1.Image{}
			objKey := types.NamespacedName{Name: image.Name, Namespace: image.Namespace}
			err := k8sClient.Create(ctx, image, &client.CreateOptions{})
			Expect(err).To(Succeed())
			err = toDetected(image, "master", "test12345")
			Expect(err).To(Succeed())
			Eventually(func() error {
				if err := k8sClient.Get(ctx, objKey, inClusterImage); err != nil {
					return err
				}
				if len(inClusterImage.Status.Conditions) == 0 {
					logrus.Info(image.Status.Conditions)
					return fmt.Errorf("conditions are not found")
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
			Eventually(func() error {
				job := batchv1.Job{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-check", image.Name), Namespace: "oci-image-operator-system"}, &job); err != nil {
					return err
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
		})
	})
	//! [test]
})

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

func toDetected(image *buildv1beta1.Image, revision, resolvedRevision string) error {
	for i, tp := range image.Spec.Repository.TagPolicies {
		if tp.Revision == revision {
			image.Spec.Repository.TagPolicies[i].ResolvedRevision = resolvedRevision
		}
	}
	if err := k8sClient.Update(context.TODO(), image, &client.UpdateOptions{}); err != nil {
		return err
	}
	t := metav1.Now()
	image.Status = buildv1beta1.ImageStatus{
		Conditions: []buildv1beta1.ImageCondition{
			{
				LastTransitionTime: &t,
				LastProbeTime:      &t,
				Type:               buildv1beta1.ImageConditionTypeDetected,
				Revision:           revision,
			},
		},
	}
	return k8sClient.Status().Update(context.TODO(), image, &client.UpdateOptions{})
}

func newImageFlowTemplate(name string) *buildv1beta1.ImageFlowTemplate {
	actor := (*buildv1beta1.ContainerApplyConfiguration)(corev1apply.Container().
		WithEnv(
			corev1apply.EnvVar().WithName("TEST_ENV").WithValue("test"),
			corev1apply.EnvVar().WithName("REPOSITORY").WithValue("test"),
			corev1apply.EnvVar().WithName("AUTH_SECRET_NAME").WithValue("test"),
		).
		WithImage("ghcr.io/takutakahashi/oci-image-operator/actor-noop:beta"))
	volumes := []buildv1beta1.VolumeApplyConfiguration{}
	tmp := buildv1beta1.ImageFlowTemplateSpecTemplate{
		Actor:   actor,
		Volumes: volumes,
		RequiredEnv: []string{
			"TEST_ENV",
		},
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
