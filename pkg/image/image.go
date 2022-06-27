package image

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Ensure(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	if after, err := EnsureDetect(ctx, c, image, template, secrets); err != nil || Diff(image, after) != "" {
		return after, err
	}
	if after, err := EnsureCheck(image, template, secrets); err != nil || Diff(image, after) != "" {
		return after, err
	}
	return EnsureCheck(image, template, secrets)
}

func Diff(before, after *buildv1beta1.Image) string {
	opts := []cmp.Option{}
	return cmp.Diff(before, after, opts...)
}

func EnsureDetect(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	deploy, err := detectDeployment(image, template)
	if err != nil {
		return nil, err
	}
	if err := apply(ctx, c, deploy); err != nil {
		return nil, err
	}
	return image, nil
}

func EnsureCheck(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	return image, nil
}

func EnsureUpload(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	return image, nil
}

func setLabel(name string, b map[string]string) map[string]string {
	if b == nil {
		b = map[string]string{}
	}
	b["build.takutakahashi.dev/image"] = name
	return b
}

func detectDeployment(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate) (*appsv1apply.DeploymentApplyConfiguration, error) {
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithServiceAccountName("actor-detect").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(),
			actorContainer(&template.Spec.Detect),
		))
	deploy := appsv1apply.Deployment(fmt.Sprintf("%s-detect", image.Name), image.Namespace).
		WithLabels(image.Labels).
		WithOwnerReferences().
		WithAnnotations(image.Annotations).
		WithSpec(appsv1apply.DeploymentSpec().
			WithReplicas(1).
			WithSelector(
				metav1apply.LabelSelector().WithMatchLabels(
					setLabel(image.Name, image.Labels))).
			WithTemplate(podTemplate))
	deploy.Spec.Template.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))
	return deploy, nil
}

func baseContainer() *corev1apply.ContainerApplyConfiguration {
	return corev1apply.Container().
		WithName("update-image").
		WithImage("ghcr.io/takutakahashi/oci-image-operator/actor-base:beta").
		WithArgs("detect").
		WithVolumeMounts(corev1apply.VolumeMount().WithMountPath("/tmp/actor-base").WithName("tmpdir"))

}

func actorContainer(spec *buildv1beta1.ImageFlowTemplateSpecTemplate) *corev1apply.ContainerApplyConfiguration {
	ret := (*corev1apply.ContainerApplyConfiguration)(spec.Actor)
	ret.Name = pointer.String("main")
	ret.Command = []string{"/entrypoint", "detect"}
	ret.VolumeMounts = append(ret.VolumeMounts, *corev1apply.VolumeMount().WithMountPath("/tmp/actor-output").WithName("tmpdir"))
	return ret
}

func apply(ctx context.Context, c client.Client, deploy *appsv1apply.DeploymentApplyConfiguration) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}
	var current appsv1.Deployment
	err = c.Get(ctx, client.ObjectKey{Namespace: *deploy.Namespace, Name: *deploy.Name}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "image-controller")
	if err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(deploy, currApplyConfig) {
		return nil
	}

	return c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "image-controller",
		Force:        pointer.Bool(true),
	})
}
