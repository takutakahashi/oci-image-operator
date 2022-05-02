package image

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	"k8s.io/utils/strings/slices"
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
	var podTemplate *corev1apply.PodTemplateSpecApplyConfiguration = (*corev1apply.PodTemplateSpecApplyConfiguration)(template.Spec.Detect.PodTemplate)
	containers := []corev1apply.ContainerApplyConfiguration{}
	for _, container := range podTemplate.Spec.Containers {
		if *container.Name == "main" {
			containedEnv := []string{}
			for _, e := range container.Env {
				if e.Name != nil && slices.Contains(template.Spec.Detect.RequiredEnv, *e.Name) {
					containedEnv = append(containedEnv, *e.Name)
				}
			}
			if d := cmp.Diff(template.Spec.Detect.RequiredEnv, containedEnv, cmpopts.SortSlices(func(i, j string) bool { return i < j })); d != "" {
				return nil, fmt.Errorf("requiredEnv and contained env are not match. diff = %s", d)
			}
			newContainer :=
				*corev1apply.Container().
					WithName(*container.Name).WithImage(*container.Image).WithCommand("/entrypoint", "detect").
					WithEnv(
						corev1apply.EnvVar().WithName("REPOSITORY").WithValue(image.Spec.Repository.URL),
						corev1apply.EnvVar().WithName("AUTH_SECRET_NAME").WithValue(image.Spec.Repository.Auth.SecretName),
					)
			newContainer.VolumeMounts = container.VolumeMounts
			newContainer.ReadinessProbe = container.ReadinessProbe
			newContainer.LivenessProbe = container.LivenessProbe
			newContainer.SecurityContext = container.SecurityContext
			newContainer.Env = append(newContainer.Env, container.Env...)
			containers = append(containers, newContainer)
		} else {
			containers = append(containers, container)
		}
	}
	podTemplate.Spec.Containers = containers
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
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return nil, err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}
	var current appsv1.Deployment
	err = c.Get(ctx, client.ObjectKey{Namespace: image.Namespace, Name: fmt.Sprintf("%s-detect", image.Name)}, &current)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	currApplyConfig, err := appsv1apply.ExtractDeployment(&current, "image-controller")
	if err != nil {
		return nil, err
	}

	if equality.Semantic.DeepEqual(deploy, currApplyConfig) {
		return image, nil
	}

	err = c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "image-controller",
		Force:        pointer.Bool(true),
	})
	return image, err
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
