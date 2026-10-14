package image

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
)

func TestMarkUploadConditionAsCanceled(t *testing.T) {
	type args struct {
		conditions       []buildv1beta1.ImageCondition
		tagPolicy        buildv1beta1.ImageTagPolicyType
		revision         string
		resolvedRevision string
	}
	tests := []struct {
		name string
		args args
		want []buildv1beta1.ImageCondition
	}{
		{
			name: "check",
			args: args{
				conditions: []buildv1beta1.ImageCondition{
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision:         "master",
						ResolvedRevision: "tocancel",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusTrue,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
						Revision:         "master",
						ResolvedRevision: "nottocancel-1",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision:         "master2",
						ResolvedRevision: "nottocancel-2",
					},
				},
				tagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
				revision:         "master",
				resolvedRevision: "qwerty",
			},
			want: []buildv1beta1.ImageCondition{
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusCanceled,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Revision:         "master",
					ResolvedRevision: "tocancel",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
					Revision:         "master",
					ResolvedRevision: "nottocancel-1",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Revision:         "master2",
					ResolvedRevision: "nottocancel-2",
				},
			},
		},
		{
			name: "upload",
			args: args{
				conditions: []buildv1beta1.ImageCondition{
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision:         "master",
						ResolvedRevision: "tocancel",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
						Revision:         "master",
						ResolvedRevision: "tocancel",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
						Revision:         "master",
						ResolvedRevision: "nottocancel-1",
					},
				},
				tagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
				revision:         "master",
				resolvedRevision: "qwerty",
			},
			want: []buildv1beta1.ImageCondition{
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusCanceled,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Revision:         "master",
					ResolvedRevision: "tocancel",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusCanceled,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
					Revision:         "master",
					ResolvedRevision: "tocancel",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
					Revision:         "master",
					ResolvedRevision: "nottocancel-1",
				},
			},
		},
		{
			name: "concern_resolved_revision",
			args: args{
				conditions: []buildv1beta1.ImageCondition{
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision:         "master",
						ResolvedRevision: "tocancel",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
						Revision:         "master",
						ResolvedRevision: "tocancel",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
						Revision:         "master",
						ResolvedRevision: "nottocancel",
					},
				},
				tagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
				revision:         "master",
				resolvedRevision: "nottocancel",
			},
			want: []buildv1beta1.ImageCondition{
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusCanceled,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Revision:         "master",
					ResolvedRevision: "tocancel",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusCanceled,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
					Revision:         "master",
					ResolvedRevision: "tocancel",
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
					Revision:         "master",
					ResolvedRevision: "nottocancel",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MarkUploadConditionAsCanceled(tt.args.conditions, tt.args.tagPolicy, tt.args.revision, tt.args.resolvedRevision); !reflect.DeepEqual(got, tt.want) {
				t.Error("MarkUploadConditionAsCanceled()")
				fmt.Println(cmp.Diff(got, tt.want))
			}
		})
	}
}

func newTestImage() *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "default",
			Labels:      map[string]string{"example.com/foo": "bar"},
			Annotations: map[string]string{"owned-by": "test"},
		},
		Spec: buildv1beta1.ImageSpec{
			Repository: buildv1beta1.ImageRepository{
				TagPolicies: []buildv1beta1.ImageTagPolicy{
					{Policy: buildv1beta1.ImageTagPolicyTypeBranchHash, Revision: "master"},
				},
			},
			Targets: []buildv1beta1.ImageTarget{{Name: "ghcr.io/test/test"}},
		},
	}
}

func newTestTemplate(t *testing.T, podTemplateJSON string) *buildv1beta1.ImageFlowTemplate {
	t.Helper()
	tmp := buildv1beta1.ImageFlowTemplateSpecTemplate{
		Actor: (*buildv1beta1.ContainerApplyConfiguration)(corev1apply.Container().WithImage("actor:test")),
	}
	if podTemplateJSON != "" {
		tmp.PodTemplate = &buildv1beta1.PodTemplateSpecApplyConfiguration{}
		if err := json.Unmarshal([]byte(podTemplateJSON), tmp.PodTemplate); err != nil {
			t.Fatal(err)
		}
	}
	return &buildv1beta1.ImageFlowTemplate{
		Spec: buildv1beta1.ImageFlowTemplateSpec{Detect: tmp, Check: tmp, Upload: tmp},
	}
}

var testCondition = buildv1beta1.ImageCondition{
	Type:             buildv1beta1.ImageConditionTypeUploaded,
	Status:           buildv1beta1.ImageConditionStatusFalse,
	TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
	Revision:         "master",
	ResolvedRevision: "abcdef",
}

// buildPodTemplates returns the pod templates of detect Deployment, check Job and upload Job.
func buildPodTemplates(t *testing.T, template *buildv1beta1.ImageFlowTemplate) map[string]*corev1apply.PodTemplateSpecApplyConfiguration {
	t.Helper()
	deploy, err := detectDeployment(newTestImage(), template)
	if err != nil {
		t.Fatal(err)
	}
	check, err := checkJob(newTestImage(), template, testCondition)
	if err != nil {
		t.Fatal(err)
	}
	upload, err := uploadJob(newTestImage(), template, testCondition)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]*corev1apply.PodTemplateSpecApplyConfiguration{
		"detect": deploy.Spec.Template,
		"check":  check.Spec.Template,
		"upload": upload.Spec.Template,
	}
}

func TestPodTemplateSpecWithoutPodTemplate(t *testing.T) {
	template := newTestTemplate(t, "")
	image := newTestImage()
	// pod template built by the previous implementation
	want := corev1apply.PodTemplateSpec().WithSpec(corev1apply.PodSpec().
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName("oci-image-operator-controller-manager").
		WithVolumes(corev1apply.Volume().WithName("tmpdir").WithEmptyDir(corev1apply.EmptyDirVolumeSource())).
		WithContainers(
			actorContainer(image.Name, image.Namespace, &template.Spec.Upload, "upload").
				WithEnv(corev1apply.EnvVar().WithName("RESOLVED_REVISION").WithValue("abcdef")),
		))
	want.ObjectMetaApplyConfiguration = metav1apply.ObjectMeta().WithLabels(setLabel(image.Name, image.Labels))

	got := buildPodTemplates(t, template)["upload"]
	if !equality.Semantic.DeepEqual(got, want) {
		t.Errorf("pod template changed without podTemplate: %s", cmp.Diff(want, got))
	}
	if got := buildPodTemplates(t, template)["detect"]; got.Spec.RestartPolicy != nil {
		t.Errorf("detect restartPolicy = %v, want nil", *got.Spec.RestartPolicy)
	}
}

func TestPodTemplateSpecMerge(t *testing.T) {
	podTemplate := `{
  "metadata": {
    "name": "ignored",
    "labels": {"build.takutakahashi.dev/image": "overwritten", "custom": "label"},
    "annotations": {"karpenter.sh/do-not-disrupt": "true"}
  },
  "spec": {
    "serviceAccountName": "other",
    "restartPolicy": "Never",
    "affinity": {"nodeAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": {"nodeSelectorTerms": [
      {"matchExpressions": [{"key": "karpenter.sh/capacity-type", "operator": "NotIn", "values": ["spot"]}]}
    ]}}},
    "tolerations": [{"key": "arch", "operator": "Equal", "value": "arm64", "effect": "NoSchedule"}],
    "nodeSelector": {"kubernetes.io/arch": "amd64"},
    "volumes": [{"name": "tmpdir", "hostPath": {"path": "/tmp"}}, {"name": "extra", "emptyDir": {}}],
    "containers": [{"name": "main", "image": "overwritten"}, {"name": "sidecar", "image": "sidecar:test"}]
  }
}`
	wantRestartPolicy := map[string]*corev1.RestartPolicy{
		"detect": nil,
		"check":  restartPolicyPtr(corev1.RestartPolicyOnFailure),
		"upload": restartPolicyPtr(corev1.RestartPolicyOnFailure),
	}
	for role, got := range buildPodTemplates(t, newTestTemplate(t, podTemplate)) {
		t.Run(role, func(t *testing.T) {
			if got.Name != nil {
				t.Errorf("metadata.name = %s, want nil", *got.Name)
			}
			wantLabels := map[string]string{"build.takutakahashi.dev/image": "test", "example.com/foo": "bar", "custom": "label"}
			if !reflect.DeepEqual(got.Labels, wantLabels) {
				t.Errorf("labels: %s", cmp.Diff(wantLabels, got.Labels))
			}
			wantAnnotations := map[string]string{"karpenter.sh/do-not-disrupt": "true"}
			if !reflect.DeepEqual(got.Annotations, wantAnnotations) {
				t.Errorf("annotations: %s", cmp.Diff(wantAnnotations, got.Annotations))
			}

			spec := got.Spec
			if *spec.ServiceAccountName != "oci-image-operator-controller-manager" {
				t.Errorf("serviceAccountName = %s", *spec.ServiceAccountName)
			}
			if !reflect.DeepEqual(spec.RestartPolicy, wantRestartPolicy[role]) {
				t.Errorf("restartPolicy: %s", cmp.Diff(wantRestartPolicy[role], spec.RestartPolicy))
			}
			if spec.Affinity == nil || len(spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) != 1 {
				t.Errorf("affinity is not passed through: %+v", spec.Affinity)
			}
			if len(spec.Tolerations) != 1 || *spec.Tolerations[0].Key != "arch" {
				t.Errorf("tolerations are not passed through: %+v", spec.Tolerations)
			}
			if spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
				t.Errorf("nodeSelector is not passed through: %+v", spec.NodeSelector)
			}

			if len(spec.Volumes) != 2 || *spec.Volumes[0].Name != "tmpdir" || spec.Volumes[0].EmptyDir == nil || spec.Volumes[0].HostPath != nil || *spec.Volumes[1].Name != "extra" {
				t.Errorf("volumes are not merged: %+v", spec.Volumes)
			}
			if len(spec.Containers) != 2 || *spec.Containers[0].Name != "main" || *spec.Containers[0].Image != "actor:test" || *spec.Containers[1].Name != "sidecar" {
				t.Errorf("containers are not merged: %+v", spec.Containers)
			}
		})
	}
}

func TestPodTemplateSpecDoesNotMutateTemplate(t *testing.T) {
	template := newTestTemplate(t, `{"spec": {"containers": [{"name": "sidecar"}], "volumes": [{"name": "extra"}]}}`)
	before := template.Spec.Upload.PodTemplate.DeepCopy()
	buildPodTemplates(t, template)
	if !reflect.DeepEqual(before, template.Spec.Upload.PodTemplate) {
		t.Errorf("template is mutated: %s", cmp.Diff(before, template.Spec.Upload.PodTemplate))
	}
}

func restartPolicyPtr(p corev1.RestartPolicy) *corev1.RestartPolicy {
	return &p
}
