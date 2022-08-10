package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	batchv1apply "k8s.io/client-go/applyconfigurations/batch/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	actorWorkDir = "/tmp/actor-base"
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
	opts := []cmp.Option{cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime")}
	return cmp.Diff(before.Status.Conditions, after.Status.Conditions, opts...)
}

func EnsureDetect(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	deploy, err := detectDeployment(image, template)
	if err != nil {
		return nil, err
	}
	if err := applyDeployment(ctx, c, deploy); err != nil {
		return nil, err
	}
	image.Status.Conditions = ensureDetectConditions(image)
	return image, nil
}

func ensureDetectConditions(image *buildv1beta1.Image) []buildv1beta1.ImageCondition {
	for _, t := range image.Spec.Repository.TagPolicies {
		image.Status.Conditions = UpdateCondition(
			image.Status.Conditions,
			buildv1beta1.ImageConditionTypeDetected,
			&buildv1beta1.ImageConditionStatusFalse,
			t.Policy,
			t.Revision,
			"",
		)
	}
	return image.Status.Conditions
}

func EnsureCheck(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	/**
		1. check result of detect from status.
		2. if changes of detect was not found, return the same image.
		3. if below are satisfied, ensure Job.
			i. detectedCondition is transitioned and transition execute after checkCondition
	**/
	logrus.Info(image.Status.Conditions)
	conds := GetCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeDetected)
	if conds == nil {
		return image, nil
	}
	// FIXME: multiple targets
	if len(image.Spec.Targets) != 1 {
		return nil, fmt.Errorf("multiple targets is not supported now")
	}
	for _, detectedCondition := range conds {
		checkedCondition := GetConditionBy(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked, detectedCondition)
		if checkedCondition.LastTransitionTime != nil && detectedCondition.LastTransitionTime.Before(checkedCondition.LastTransitionTime) {
			logrus.Info("image already checked")
			continue
		}
		logrus.Info("checking image")
		job, err := checkJob(image, template, detectedCondition)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build job")
		}
		if err := applyJob(ctx, c, job); err != nil {
			return nil, errors.Wrap(err, "failed to apply job")
		}
	}
	return image, nil
}

func EnsureUpload(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	conds := GetCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked)
	if conds == nil {
		return image, nil
	}
	for _, checkedCondition := range conds {
		uploadedCondition := GetConditionBy(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked, checkedCondition)
		if uploadedCondition.LastTransitionTime != nil && checkedCondition.LastTransitionTime.Before(uploadedCondition.LastTransitionTime) && uploadedCondition.Status == buildv1beta1.ImageConditionStatusTrue {
			logrus.Info("image already uploaded")
			continue
		}
		logrus.Info("uploading image")
		job, err := uploadJob(image, template, uploadedCondition)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build job")
		}
		if err := applyJob(ctx, c, job); err != nil {
			return nil, errors.Wrap(err, "failed to apply job")
		}
	}
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
	tags, branches := []string{}, []string{}
	for _, policy := range image.Spec.Repository.TagPolicies {
		switch policy.Policy {
		case buildv1beta1.ImageTagPolicyTypeBranchHash:
			branches = append(branches, policy.Revision)
		case buildv1beta1.ImageTagPolicyTypeTagHash:
			tags = append(tags, policy.Revision)
		}
	}
	targetEnv := []*corev1apply.EnvVarApplyConfiguration{
		corev1apply.EnvVar().WithName("TARGET_BRANCHES").WithValue(strings.Join(branches, ",")),
		corev1apply.EnvVar().WithName("TARGET_TAGS").WithValue(strings.Join(tags, ",")),
	}
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithServiceAccountName("oci-image-operator-controller-manager").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(image.Name, image.Namespace, "detect").WithEnv(targetEnv...),
			actorContainer(&template.Spec.Detect, "detect").WithEnv(targetEnv...),
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

func checkJob(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, detectedCondition buildv1beta1.ImageCondition) (*batchv1apply.JobApplyConfiguration, error) {
	revEnv := corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue(detectedCondition.ResolvedRevision)
	registryEnv := []*corev1apply.EnvVarApplyConfiguration{
		corev1apply.EnvVar().WithName("REGISTRY_IMAGE_NAME").WithValue(image.Spec.Targets[0].Name),
	}
	if image.Spec.Targets[0].Auth.SecretName != "" {
		registryEnv = append(registryEnv,
			corev1apply.EnvVar().WithName("REGISTRY_AUTH_USERNAME").WithValueFrom(corev1apply.EnvVarSource().WithSecretKeyRef(corev1apply.SecretKeySelector().WithName(image.Spec.Targets[0].Auth.SecretName).WithKey("username"))),
			corev1apply.EnvVar().WithName("REGISTRY_AUTH_PASSWORD").WithValueFrom(corev1apply.EnvVarSource().WithSecretKeyRef(corev1apply.SecretKeySelector().WithName(image.Spec.Targets[0].Auth.SecretName).WithKey("password"))),
		)
	}
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName("oci-image-operator-controller-manager").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(image.Name, image.Namespace, "check").WithEnv(revEnv).WithEnv(registryEnv...),
			actorContainer(&template.Spec.Check, "check").WithEnv(revEnv).WithEnv(registryEnv...),
		))
	// add sha256 from revision and tag policy
	r := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", detectedCondition.Revision, detectedCondition.TagPolicy)))
	h := hex.EncodeToString(r[:])
	name := fmt.Sprintf("%s-check-%s", image.Name, h[:7])
	job := batchv1apply.Job(name, "oci-image-operator-system").
		WithLabels(image.Labels).
		// TODO: add owner reference
		WithOwnerReferences().
		WithAnnotations(image.Annotations).
		WithSpec(batchv1apply.JobSpec().
			WithTemplate(podTemplate))
	job.Spec.Template.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))
	return job, nil
}

func uploadJob(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, uploadedCondition buildv1beta1.ImageCondition) (*batchv1apply.JobApplyConfiguration, error) {
	revEnv := corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue(uploadedCondition.ResolvedRevision)
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName("oci-image-operator-controller-manager").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			baseContainer(image.Name, image.Namespace, "upload").WithEnv(revEnv),
			actorContainer(&template.Spec.Upload, "upload").WithEnv(revEnv),
		))
	// add sha256 from revision and tag policy
	r := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", uploadedCondition.Revision, uploadedCondition.TagPolicy)))
	h := hex.EncodeToString(r[:])
	name := fmt.Sprintf("%s-upload-%s", image.Name, h[:7])
	job := batchv1apply.Job(name, "oci-image-operator-system").
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
		WithImagePullPolicy(corev1.PullAlways).
		WithEnv(
			corev1apply.EnvVar().WithName("IMAGE_NAME").WithValue(name),
			corev1apply.EnvVar().WithName("IMAGE_NAMESPACE").WithValue(namespace),
		).
		WithVolumeMounts(corev1apply.VolumeMount().WithMountPath(actorWorkDir).WithName("tmpdir"))

}

func actorContainer(spec *buildv1beta1.ImageFlowTemplateSpecTemplate, role string) *corev1apply.ContainerApplyConfiguration {
	return (*corev1apply.ContainerApplyConfiguration)(spec.Actor.DeepCopy()).
		WithName("main").
		WithArgs(role).
		WithVolumeMounts(corev1apply.VolumeMount().WithMountPath(actorWorkDir).WithName("tmpdir"))
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
	if err := c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "image-controller",
		Force:        pointer.Bool(true),
	}); err != nil {
		existsJob := &batchv1.Job{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: *job.Namespace, Name: *job.Name}, existsJob); err != nil {
			return err
		}
		if err := c.Delete(ctx, existsJob, &client.DeleteOptions{}); err != nil {
			return err
		}
		return c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
			FieldManager: "image-controller",
			Force:        pointer.Bool(true),
		})
	}
	return nil
}

func GetCondition(conditions []buildv1beta1.ImageCondition, conditionType buildv1beta1.ImageConditionType) []buildv1beta1.ImageCondition {
	ret := []buildv1beta1.ImageCondition{}
	for _, c := range conditions {
		if c.Type == conditionType {
			ret = append(ret, c)
		}
	}
	return ret
}

func GetConditionBy(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, baseCondition buildv1beta1.ImageCondition) buildv1beta1.ImageCondition {
	for _, c := range conditions {
		if c.Type == condType && c.Revision == baseCondition.Revision && c.TagPolicy == baseCondition.TagPolicy {
			return c
		}
	}
	return buildv1beta1.ImageCondition{
		LastTransitionTime: nil,
		Type:               condType,
		Status:             buildv1beta1.ImageConditionStatusUnknown,
		Revision:           baseCondition.Revision,
		ResolvedRevision:   "",
		TagPolicy:          baseCondition.TagPolicy,
	}
}
func SetCondition(conditions []buildv1beta1.ImageCondition, condition buildv1beta1.ImageCondition) []buildv1beta1.ImageCondition {
	for i, c := range conditions {
		if c.Type == condition.Type && c.Revision == condition.Revision && c.TagPolicy == condition.TagPolicy {
			conditions[i] = condition
			return conditions
		}
	}
	conditions = append(conditions, condition)
	return conditions
}

func UpdateCheckedCondition(conditions []buildv1beta1.ImageCondition, status buildv1beta1.ImageConditionStatus, resolvedRevision string) []buildv1beta1.ImageCondition {
	exists := false
	now := v1.Now()
	conds := GetCondition(conditions, buildv1beta1.ImageConditionTypeChecked)
	for _, cond := range conds {
		if cond.ResolvedRevision == resolvedRevision {
			exists = true
			if cond.Status != status {
				cond.Status = status
				cond.LastTransitionTime = &now
				return SetCondition(conditions, cond)
			}
		}
	}
	if !exists {
		conditions = append(conditions, buildv1beta1.ImageCondition{
			Type:               buildv1beta1.ImageConditionTypeChecked,
			Status:             status,
			TagPolicy:          buildv1beta1.ImageTagPolicyTypeUnused,
			Revision:           "",
			ResolvedRevision:   resolvedRevision,
			LastTransitionTime: &now,
		})
	}
	return conditions

}
func UpdateUploadedCondition(conditions []buildv1beta1.ImageCondition, status buildv1beta1.ImageConditionStatus, resolvedRevision string) []buildv1beta1.ImageCondition {
	exists := false
	now := v1.Now()
	conds := GetCondition(conditions, buildv1beta1.ImageConditionTypeUploaded)
	for _, cond := range conds {
		if cond.ResolvedRevision == resolvedRevision {
			exists = true
			if cond.Status != status {
				cond.Status = status
				cond.LastTransitionTime = &now
				return SetCondition(conditions, cond)
			}
		}
	}
	if !exists {
		conditions = append(conditions, buildv1beta1.ImageCondition{
			Type:               buildv1beta1.ImageConditionTypeUploaded,
			Status:             status,
			TagPolicy:          buildv1beta1.ImageTagPolicyTypeUnused,
			Revision:           "",
			ResolvedRevision:   resolvedRevision,
			LastTransitionTime: &now,
		})
	}
	return conditions
}

func UpdateCondition(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, status *buildv1beta1.ImageConditionStatus, tagPolicy buildv1beta1.ImageTagPolicyType, revision, resolvedRevision string) []buildv1beta1.ImageCondition {
	now := v1.Now()
	cond := GetConditionBy(conditions, condType, buildv1beta1.ImageCondition{TagPolicy: tagPolicy, Revision: revision})
	if cond.LastTransitionTime == nil {
		if status != nil {
			cond.Status = *status
		} else {
			cond.Status = buildv1beta1.ImageConditionStatusUnknown
		}
		cond.TagPolicy = tagPolicy
		cond.Revision = revision
		cond.ResolvedRevision = resolvedRevision
		cond.LastTransitionTime = &now
		return SetCondition(conditions, cond)
	}
	if status == nil {
		if cond.ResolvedRevision != resolvedRevision {
			cond.Status = buildv1beta1.ImageConditionStatusTrue
			cond.LastTransitionTime = &now
		} else {
			cond.Status = buildv1beta1.ImageConditionStatusFalse
			cond.LastTransitionTime = &now
		}
	} else {
		if cond.ResolvedRevision != resolvedRevision {
			cond.ResolvedRevision = resolvedRevision
			cond.LastTransitionTime = &now
		}
		if cond.Status != *status {
			cond.Status = *status
			cond.LastTransitionTime = &now
		}
	}
	return SetCondition(conditions, cond)

}

func InWorkDir(path string) string {
	return fmt.Sprintf("%s/%s", actorWorkDir, path)
}
