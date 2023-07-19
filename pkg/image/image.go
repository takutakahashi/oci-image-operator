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
	ttl          = 86400
)

func Ensure(ctx context.Context, c client.Client, image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	for _, cond := range image.Status.Conditions {
		if err := cancelJob(ctx, c, image, cond); err != nil {
			return nil, err
		}
	}
	// TODO: refine status only uploaded
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
	if image.DeletionTimestamp != nil {
		if err := c.Delete(ctx, &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name: fmt.Sprintf("%s-detect", image.Name), Namespace: "oci-image-operator-system",
			},
		}); client.IgnoreNotFound(err) != nil {
			return image, err
		} else {
			image.SetFinalizers([]string{})
			return image, c.Update(ctx, image, &client.UpdateOptions{})
		}
	}
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
	conds := GetConditionByStatus(image.Status.Conditions, buildv1beta1.ImageConditionTypeChecked, buildv1beta1.ImageConditionStatusFalse)
	if len(conds) == 0 {
		return image, nil
	}
	// FIXME: multiple targets
	if len(image.Spec.Targets) != 1 {
		return nil, fmt.Errorf("multiple targets is not supported now")
	}
	for _, checkedCondition := range conds {
		logrus.Info("checking image")
		job, err := checkJob(image, template, checkedCondition)
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
	conds := GetConditionByStatus(image.Status.Conditions, buildv1beta1.ImageConditionTypeUploaded, buildv1beta1.ImageConditionStatusFalse)
	if conds == nil {
		return image, nil
	}
	for _, uploadedCondition := range conds {
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

//func ReapStaleConditions(conds []buildv1beta1.ImageCondition) []buildv1beta1.ImageCondition {
//	GetConditionBy(conds, buildv1beta1.ImageConditionTypeChecked, buildv1beta1.ImageCondition{})
//}

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
			actorContainer(image.Name, image.Namespace, &template.Spec.Detect, "detect").WithEnv(targetEnv...).WithEnv(toEnvVarConfiguration(image.Spec.Env)...),
		))
	deploy := appsv1apply.Deployment(fmt.Sprintf("%s-detect", image.Name), "oci-image-operator-system").
		WithLabels(image.Labels).
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

func checkJob(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, checkedCondition buildv1beta1.ImageCondition) (*batchv1apply.JobApplyConfiguration, error) {
	revEnv := corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue(checkedCondition.ResolvedRevision)
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
			actorContainer(image.Name, image.Namespace, &template.Spec.Check, "check").WithEnv(revEnv).WithEnv(registryEnv...).WithEnv(toEnvVarConfiguration(image.Spec.Env)...),
		))
	// add sha256 from revision and tag policy
	name := genName(image.Name, checkedCondition)
	job := batchv1apply.Job(name, "oci-image-operator-system").
		WithLabels(image.Labels).
		// TODO: add owner reference
		WithOwnerReferences().
		WithAnnotations(image.Annotations).
		WithSpec(batchv1apply.JobSpec().
			WithTemplate(podTemplate).
			WithTTLSecondsAfterFinished(ttl))
	job.Spec.Template.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))
	return job, nil
}

func genName(imageName string, cond buildv1beta1.ImageCondition) string {
	op := ""
	switch cond.Type {
	case buildv1beta1.ImageConditionTypeUploaded:
		op = "upload"
	case buildv1beta1.ImageConditionTypeChecked:
		op = "check"

	default:
		op = "unknown"
	}
	r := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%s", cond.TagPolicy, cond.Revision, cond.ResolvedRevision)))
	h := hex.EncodeToString(r[:])
	return fmt.Sprintf("%s-%s-%s", imageName, op, h[:7])
}

func cancelJob(ctx context.Context, c client.Client, image *buildv1beta1.Image, cond buildv1beta1.ImageCondition) error {
	if cond.Status != buildv1beta1.ImageConditionStatusCanceled {
		return nil
	}
	p := v1.DeletePropagationBackground
	return client.IgnoreNotFound(c.Delete(ctx, &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      genName(image.Name, cond),
			Namespace: "oci-image-operator-system",
		},
	}, &client.DeleteOptions{
		PropagationPolicy: &p,
	}))
}

func uploadJob(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, uploadedCondition buildv1beta1.ImageCondition) (*batchv1apply.JobApplyConfiguration, error) {
	revEnv := corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue(uploadedCondition.ResolvedRevision)
	podTemplate := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName("oci-image-operator-controller-manager").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			actorContainer(image.Name, image.Namespace, &template.Spec.Upload, "upload").WithEnv(revEnv).WithEnv(toEnvVarConfiguration(image.Spec.Env)...),
		))
	// add sha256 from revision and tag policy
	name := genName(image.Name, uploadedCondition)
	job := batchv1apply.Job(name, "oci-image-operator-system").
		WithLabels(image.Labels).
		WithAnnotations(image.Annotations).
		WithSpec(batchv1apply.JobSpec().
			WithTemplate(podTemplate).
			WithTTLSecondsAfterFinished(ttl))
	job.Spec.Template.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))
	return job, nil
}

func actorContainer(name, namespace string, spec *buildv1beta1.ImageFlowTemplateSpecTemplate, role string) *corev1apply.ContainerApplyConfiguration {
	return (*corev1apply.ContainerApplyConfiguration)(spec.Actor.DeepCopy()).
		WithName("main").
		WithArgs(role).
		WithEnv(
			corev1apply.EnvVar().WithName("IMAGE_NAME").WithValue(name),
			corev1apply.EnvVar().WithName("IMAGE_NAMESPACE").WithValue(namespace),
		).
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
func jobRunning(ctx context.Context, c client.Client, job *batchv1apply.JobApplyConfiguration) (bool, error) {
	existJob := &batchv1.Job{}
	if err := c.Get(ctx, types.NamespacedName{Name: *job.Name, Namespace: *job.Namespace}, existJob); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	} else if apierrors.IsNotFound(err) {
		return false, nil
	}
	return existJob.Status.Active > 0, nil
}
func applyJob(ctx context.Context, c client.Client, job *batchv1apply.JobApplyConfiguration) error {
	// wait for previous job
	if running, err := jobRunning(ctx, c, job); err != nil {
		return errors.Wrap(err, "failed to get Job status")
	} else if running {
		return fmt.Errorf("previous job is still running")
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(job)
	if err != nil {
		return errors.Wrap(err, "failed to ToUnstructured")
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}
	var current batchv1.Job
	err = c.Get(ctx, client.ObjectKey{Namespace: *job.Namespace, Name: *job.Name}, &current)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to Get")
	}

	currApplyConfig, err := batchv1apply.ExtractJob(&current, "image-controller")
	if err != nil {
		return errors.Wrap(err, "failed to extractJob")
	}
	if equality.Semantic.DeepEqual(job, currApplyConfig) {
		return nil
	}
	if current.GetName() != "" {
		if err := c.Delete(ctx, &current, &client.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to Delete")
		}
	}
	return c.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: "image-controller",
		Force:        pointer.Bool(true),
	})
}
func toEnvVarConfiguration(env []corev1.EnvVar) []*corev1apply.EnvVarApplyConfiguration {
	ret := []*corev1apply.EnvVarApplyConfiguration{}
	for _, e := range env {
		c := corev1apply.EnvVar().WithName(e.Name).WithValue(e.Value)
		if e.ValueFrom != nil {
			if e.ValueFrom.ConfigMapKeyRef != nil {
				c.WithValueFrom(corev1apply.EnvVarSource().
					WithConfigMapKeyRef(corev1apply.ConfigMapKeySelector().
						WithKey(e.ValueFrom.ConfigMapKeyRef.Key).
						WithName(e.ValueFrom.ConfigMapKeyRef.Name).
						WithOptional(*e.ValueFrom.ConfigMapKeyRef.Optional)))
			}
			if e.ValueFrom.SecretKeyRef != nil {
				c.WithValueFrom(corev1apply.EnvVarSource().
					WithSecretKeyRef(corev1apply.SecretKeySelector().
						WithKey(e.ValueFrom.SecretKeyRef.Key).
						WithName(e.ValueFrom.SecretKeyRef.Name).
						WithOptional(*e.ValueFrom.SecretKeyRef.Optional)))
			}
			if e.ValueFrom.ResourceFieldRef != nil {
				c.WithValueFrom(corev1apply.EnvVarSource().
					WithResourceFieldRef(corev1apply.ResourceFieldSelector().
						WithContainerName(e.ValueFrom.ResourceFieldRef.ContainerName).
						WithResource(e.ValueFrom.ResourceFieldRef.Resource).
						WithDivisor(e.ValueFrom.ResourceFieldRef.Divisor)))
			}
			if e.ValueFrom.FieldRef != nil {
				c.WithValueFrom(corev1apply.EnvVarSource().
					WithFieldRef(corev1apply.ObjectFieldSelector().
						WithFieldPath(e.ValueFrom.FieldRef.FieldPath).
						WithAPIVersion(e.ValueFrom.FieldRef.APIVersion)))
			}
		}
		ret = append(ret, c)
	}
	return ret
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

func GetConditionByStatus(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, status buildv1beta1.ImageConditionStatus) []buildv1beta1.ImageCondition {
	ret := []buildv1beta1.ImageCondition{}
	for _, c := range GetCondition(conditions, condType) {
		if c.Status == status {
			ret = append(ret, c)
		}
	}
	return ret
}

/*
Mark as Canceled with below strategy.
 1. checked Condition will be canceled when specified tagPolicy and revision are matched
 2. upload condition will be canceled when checked condition with specified tagPolicy and revision is exists and resolved revision of uploaded will be matched
*/
func MarkUploadConditionAsCanceled(conditions []buildv1beta1.ImageCondition, tagPolicy buildv1beta1.ImageTagPolicyType, revision string) []buildv1beta1.ImageCondition {
	// TODO: not cancel latest upload
	for i, c := range conditions {
		if c.Type == buildv1beta1.ImageConditionTypeUploaded && c.Revision == revision {
			checked := GetConditionByRevision(conditions, buildv1beta1.ImageConditionTypeChecked, revision)
			if checked.TagPolicy == tagPolicy && checked.ResolvedRevision == c.ResolvedRevision && checked.Status != buildv1beta1.ImageConditionStatusUnknown {
				conditions[i].Status = buildv1beta1.ImageConditionStatusCanceled
			}
		}
		if c.Type == buildv1beta1.ImageConditionTypeChecked && c.TagPolicy == tagPolicy && c.Revision == revision {
			conditions[i].Status = buildv1beta1.ImageConditionStatusCanceled
		}
	}
	return conditions

}
func GetConditionByPolicy(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, tagPolicy buildv1beta1.ImageTagPolicyType, revision string) []buildv1beta1.ImageCondition {
	ret := []buildv1beta1.ImageCondition{}
	for _, c := range GetCondition(conditions, condType) {
		if c.TagPolicy == tagPolicy && c.Revision == revision {
			ret = append(ret, c)
		}
	}
	return ret

}

func GetConditionByResolvedRevision(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, resolvedRevision string) buildv1beta1.ImageCondition {
	for _, c := range conditions {
		if c.Type == condType && c.ResolvedRevision == resolvedRevision {
			return c
		}
	}
	return buildv1beta1.ImageCondition{
		LastTransitionTime: nil,
		Type:               condType,
		Status:             buildv1beta1.ImageConditionStatusUnknown,
		ResolvedRevision:   resolvedRevision,
	}
}
func GetConditionByRevision(conditions []buildv1beta1.ImageCondition, condType buildv1beta1.ImageConditionType, revision string) buildv1beta1.ImageCondition {
	for _, c := range conditions {
		if c.Type == condType && c.Revision == revision {
			return c
		}
	}
	return buildv1beta1.ImageCondition{
		LastTransitionTime: nil,
		Type:               condType,
		Status:             buildv1beta1.ImageConditionStatusUnknown,
		Revision:           revision,
	}
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

func UpdateCheckedCondition(conditions []buildv1beta1.ImageCondition, status buildv1beta1.ImageConditionStatus, revision, resolvedRevision string) []buildv1beta1.ImageCondition {
	exists := false
	now := v1.Now()
	conds := GetCondition(conditions, buildv1beta1.ImageConditionTypeChecked)
	for _, cond := range conds {
		if cond.ResolvedRevision == resolvedRevision {
			exists = true
			cond.Revision = revision
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
			Revision:           revision,
			ResolvedRevision:   resolvedRevision,
			LastTransitionTime: &now,
		})
	}
	return conditions

}
func UpdateUploadedCondition(conditions []buildv1beta1.ImageCondition, status buildv1beta1.ImageConditionStatus, revision, resolvedRevision string) []buildv1beta1.ImageCondition {
	now := v1.Now()
	exist := false
	for i, c := range conditions {
		if c.Revision == revision &&
			c.Type == buildv1beta1.ImageConditionTypeUploaded &&
			c.ResolvedRevision == resolvedRevision {
			conditions[i].Status = status
			conditions[i].LastTransitionTime = &now
			exist = true
		}
	}
	if !exist {
		conditions = append(conditions, buildv1beta1.ImageCondition{
			Type:               buildv1beta1.ImageConditionTypeUploaded,
			Status:             status,
			TagPolicy:          buildv1beta1.ImageTagPolicyTypeUnused,
			Revision:           revision,
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
