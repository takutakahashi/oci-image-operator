package controllers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Image controller", func() {
	var _ = BeforeEach(func() {
		ctx := context.TODO()
		err := k8sClient.DeleteAllOf(ctx, &buildv1beta1.ImageFlowTemplate{}, client.InNamespace("default"))
		Expect(err).To(Succeed())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace("default"))
		Expect(err).To(Succeed())
		err = k8sClient.Create(ctx, newImageFlowTemplate("test"), &client.CreateOptions{})
		Expect(err).To(Succeed())
		err = k8sClient.Create(ctx, newSecret("test"), &client.CreateOptions{})
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
				if deploy.Spec.Template.Spec.ServiceAccountName != "oci-image-operator-controller-manager" {
					return errors.New("wrong service account name")
				}
				c := mainContainer(deploy.Spec.Template.Spec.Containers)
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
				if e := getEnv(c.Env, "TARGET_BRANCHES"); e.Value != "main" {
					return fmt.Errorf("target branches = %v", e.Value)
				}
				if e := getEnv(c.Env, "TARGET_TAGS"); e.Value != "latest" {
					return fmt.Errorf("target branches = %v", e.Value)
				}
				if e := getEnv(c.Env, "DEF_IMAGE_VALUE"); e.Value != "image" {
					return fmt.Errorf("target branches = %v", e.Value)
				}
				if e := getEnv(c.Env, "DEF_IMAGE_CM"); e.ValueFrom.ConfigMapKeyRef.Key != "cm" {
					return fmt.Errorf("target branches = %v", e.ValueFrom.ConfigMapKeyRef.Key)
				}
				c = baseContainer(deploy.Spec.Template.Spec.Containers)
				if c.Args[len(c.Args)-1] != "detect" {
					return fmt.Errorf("args not contain detect")
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
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-check-check-dd2a454", Namespace: "oci-image-operator-system"}, &job); err != nil {
					return err
				}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-check-check-0984785", Namespace: "oci-image-operator-system"}, &job); err != nil {
					return err
				}
				c := mainContainer(job.Spec.Template.Spec.Containers)
				if e := getEnv(c.Env, "RESOLVED_REVISION"); e.Value != "test12345" {
					return fmt.Errorf("env is not match. env: %v", e)
				}

				c = baseContainer(job.Spec.Template.Spec.Containers)
				if e := getEnv(c.Env, "RESOLVED_REVISION"); e.Value != "test12345" {
					return fmt.Errorf("env is not match. env: %v", e)
				}
				if e := getEnv(c.Env, "DEF_IMAGE_VALUE"); e.Value != "image" {
					return fmt.Errorf("target branches = %v", e.Value)
				}
				if e := getEnv(c.Env, "DEF_IMAGE_CM"); e.ValueFrom.ConfigMapKeyRef.Key != "cm" {
					return fmt.Errorf("target branches = %v", e.ValueFrom.ConfigMapKeyRef.Key)
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
		})
		It("upload should success", func() {
			ctx := context.TODO()
			image := newImage("test-upload")
			inClusterImage := &buildv1beta1.Image{}
			objKey := types.NamespacedName{Name: image.Name, Namespace: image.Namespace}
			err := k8sClient.Create(ctx, image, &client.CreateOptions{})
			Expect(err).To(Succeed())
			err = toChecked(image, "master", "test12345")
			Expect(err).To(Succeed())
			Eventually(func() error {
				if err := k8sClient.Get(ctx, objKey, inClusterImage); err != nil {
					return err
				}
				if len(inClusterImage.Status.Conditions) == 0 {
					logrus.Info(image.Status.Conditions)
					return fmt.Errorf("conditions are not found")
				}
				job := batchv1.Job{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-upload-upload-0984785", Namespace: "oci-image-operator-system"}, &job); err != nil {
					return err
				}
				return nil
			}).WithTimeout(2000 * time.Millisecond).Should(Succeed())
		})
	})
	//! [test]
})

func newSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		StringData: map[string]string{
			"username": "username",
			"password": "password",
		},
	}
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

func toDetected(image *buildv1beta1.Image, revision, resolvedRevision string) error {
	t := metav1.Now()
	image.Status = buildv1beta1.ImageStatus{
		Conditions: []buildv1beta1.ImageCondition{
			{
				LastTransitionTime: &t,
				Status:             buildv1beta1.ImageConditionStatusFalse,
				Type:               buildv1beta1.ImageConditionTypeChecked,
				Revision:           revision,
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
			},
			{
				LastTransitionTime: &t,
				Type:               buildv1beta1.ImageConditionTypeChecked,
				Status:             buildv1beta1.ImageConditionStatusFalse,
				Revision:           "master2",
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
			},
		},
	}
	return k8sClient.Status().Update(context.TODO(), image, &client.UpdateOptions{})
}
func toChecked(image *buildv1beta1.Image, revision, resolvedRevision string) error {
	t := metav1.Now()
	image.Status = buildv1beta1.ImageStatus{
		Conditions: []buildv1beta1.ImageCondition{
			{
				LastTransitionTime: &t,
				Status:             buildv1beta1.ImageConditionStatusTrue,
				Type:               buildv1beta1.ImageConditionTypeChecked,
				Revision:           revision,
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
			},
			{
				LastTransitionTime: &t,
				Status:             buildv1beta1.ImageConditionStatusTrue,
				Type:               buildv1beta1.ImageConditionTypeChecked,
				Revision:           "master2",
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
			},
			{
				LastTransitionTime: &t,
				Status:             buildv1beta1.ImageConditionStatusFalse,
				Type:               buildv1beta1.ImageConditionTypeUploaded,
				Revision:           revision,
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
			},
			{
				LastTransitionTime: &t,
				Status:             buildv1beta1.ImageConditionStatusFalse,
				Type:               buildv1beta1.ImageConditionTypeUploaded,
				Revision:           "master2",
				ResolvedRevision:   resolvedRevision,
				TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
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

func mainContainer(containers []corev1.Container) corev1.Container {
	return getContainer(containers, "main")
}

func baseContainer(containers []corev1.Container) corev1.Container {
	return getContainer(containers, "actor-base")
}

func getContainer(containers []corev1.Container, name string) corev1.Container {
	for _, c := range containers {
		if c.Name == name {
			return c
		}
	}
	panic("container not found")
}

func getEnv(env []corev1.EnvVar, key string) corev1.EnvVar {
	for _, e := range env {
		if e.Name == key {
			return e
		}
	}
	panic("env not found")
}
