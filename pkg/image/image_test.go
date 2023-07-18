package image

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MarkUploadConditionAsCanceled(tt.args.conditions, tt.args.tagPolicy, tt.args.revision); !reflect.DeepEqual(got, tt.want) {
				t.Error("MarkUploadConditionAsCanceled()")
				fmt.Println(cmp.Diff(got, tt.want))
			}
		})
	}
}
