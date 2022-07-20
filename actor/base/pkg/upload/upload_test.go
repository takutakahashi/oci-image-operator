package upload

import (
	"reflect"
	"testing"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

func Test_getInput(t *testing.T) {
	type args struct {
		target     string
		conditions []buildv1beta1.ImageCondition
	}
	tests := []struct {
		name string
		args args
		want Input
	}{
		{
			name: "ok",
			args: args{
				target: "target",
				conditions: []buildv1beta1.ImageCondition{
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						ResolvedRevision: "resolved",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusTrue,
						ResolvedRevision: "resolved_true",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeUploaded,
						Status:           buildv1beta1.ImageConditionStatusUnknown,
						ResolvedRevision: "resolved_unknown",
					},
					{
						Type:             buildv1beta1.ImageConditionTypeChecked,
						Status:           buildv1beta1.ImageConditionStatusFalse,
						ResolvedRevision: "resolved_checked",
					},
				},
			},
			want: Input{
				Builds: []ImageBuild{
					{
						Target: "target",
						Tag:    "resolved",
					},
					{
						Target: "target",
						Tag:    "resolved_unknown",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getInput(tt.args.target, tt.args.conditions); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getInput() = %v, want %v", got, tt.want)
			}
		})
	}
}
