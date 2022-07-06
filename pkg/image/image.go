package image

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	batchv1apply "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Ensure(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	if after, err := EnsureDetect(ctx, c, image, template, secrets); err != nil || Diff(image, after) != "" {
		return after, err
	}
	if after, err := EnsureCheck(ctx, c, image, template, secrets); err != nil || Diff(image, after) != "" {
		return after, err
	}
	return EnsureUpload(ctx, c, image, template, secrets)
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
	if err := applyDeployment(ctx, c, deploy); err != nil {
		return nil, err
	}
	return image, nil
}

func EnsureCheck(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	/**
		1. check result of detect from status.
		2. if changes of detect was not found, return the same image.
		3. if below are satisfied, ensure Job.
			i. detectedCondition is transitioned and transition execute after checkCondition
	**/
	logrus.Info(image.Status.Conditions)
	detectedCondition := getCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeDetected)
	if detectedCondition.LastTransitionTime == nil {
		logrus.Info("image not detected")
		return image, nil
	}
	checkedCondition := getCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked)
	if checkedCondition.LastTransitionTime != nil && detectedCondition.LastTransitionTime.Before(checkedCondition.LastTransitionTime) {
		logrus.Info("image already checked")
		return image, nil
	}
	logrus.Info("checking image")
	job, err := checkJob(image, template)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build job")
	}
	if err := applyJob(ctx, c, job); err != nil {
		return nil, errors.Wrap(err, "failed to apply job")
	}
	return image, nil
}

func EnsureUpload(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
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
		WithServiceAccountName("oci-image-operator-actor-detect").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(image.Name, image.Namespace, "detect"),
			actorContainer(&template.Spec.Detect, "detect"),
		))
	deploy := appsv1apply.Deployment(fmt.Sprintf("%s-detect", image.Name), "oci-image-operator-system").
		WithLabels(image.Labels).
		// TODO: add owner reference
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

func checkJob(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate) (*batchv1apply.JobApplyConfiguration, error) {
	detectedCondition := getCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeDetected)
	rev, err := getResolvedRevision(image, detectedCondition)
	if err != nil {
		return nil, err
	}
	revEnv := corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue(rev)
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName("oci-image-operator-actor-check").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(image.Name, image.Namespace, "check").WithEnv(revEnv),
			actorContainer(&template.Spec.Check, "check").WithEnv(revEnv),
		))
	job := batchv1apply.Job(fmt.Sprintf("%s-check", image.Name), "oci-image-operator-system").
		WithLabels(image.Labels).
		// TODO: add owner reference
		WithOwnerReferences().
		WithAnnotations(image.Annotations).
		WithSpec(batchv1apply.JobSpec().
			WithTemplate(podTemplate))
	job.Spec.Template.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))
	return job, nil
}

func baseContainer(name, namespace, role string) *corev1apply.ContainerApplyConfiguration {
	return corev1apply.Container().
		WithName("actor-base").
		WithImage("ghcr.io/takutakahashi/oci-image-operator/actor-base:beta").
		WithArgs(role).
		WithEnv(
			corev1apply.EnvVar().WithName("IMAGE_NAME").WithValue(name),
			corev1apply.EnvVar().WithName("IMAGE_NAMESPACE").WithValue(namespace),
		).
		WithVolumeMounts(corev1apply.VolumeMount().WithMountPath("/tmp/actor-base").WithName("tmpdir"))

}

func actorContainer(spec *buildv1beta1.ImageFlowTemplateSpecTemplate, role string) *corev1apply.ContainerApplyConfiguration {
	ret := (*corev1apply.ContainerApplyConfiguration)(spec.Actor)
	ret.Name = pointer.String("main")
	ret.Command = []string{"/entrypoint", role}
	ret.VolumeMounts = append(ret.VolumeMounts, *corev1apply.VolumeMount().WithMountPath("/tmp/actor-output").WithName("tmpdir"))
	return ret
}

func applyDeployment(ctx context.Context, c client.Client, deploy *appsv1apply.DeploymentApplyConfiguration) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}
	var current appsv1.Deployment
	err = c.Get(ctx, client.ObjectKey{Namespace: *deploy.Namespace, Name: *deploy.Name}, &current)
	if err != nil && !apierrors.IsNotFound(err) {
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
func applyJob(ctx context.Context, c client.Client, job *batchv1apply.JobApplyConfiguration) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(job)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}
	var current batchv1.Job
	err = c.Get(ctx, client.ObjectKey{Namespace: *job.Namespace, Name: *job.Name}, &current)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	currApplyConfig, err := batchv1apply.ExtractJob(&current, "image-controller")
	if err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(job, currApplyConfig) {
		return nil
	}

	return c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "image-controller",
		Force:        pointer.Bool(true),
	})
}

func getCondition(conditions []buildv1beta1.ImageCondition, conditionType buildv1beta1.ImageConditionType) buildv1beta1.ImageCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return c
		}
	}
	return buildv1beta1.ImageCondition{
		LastTransitionTime: nil,
		Status:             buildv1beta1.ImageConditionStatusUnknown,
		Type:               conditionType,
	}
}

func getResolvedRevision(image *buildv1beta1.Image, detectedCondition buildv1beta1.ImageCondition) (string, error) {
	for _, p := range image.Spec.Repository.TagPolicies {
		if p.Revision == detectedCondition.Revision && p.Policy == detectedCondition.TagPolicy {
			return p.ResolvedRevision, nil
		}
	}
	return "", fmt.Errorf("failed to get resolved revision. %v, %v", detectedCondition.Revision, detectedCondition.TagPolicy)
}
