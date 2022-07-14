package check

import (
	"reflect"
	"testing"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCheckFile(t *testing.T) {
	now := v1.Now()
	type args struct {
		registry string
		conds    []buildv1beta1.ImageCondition
	}
	tests := []struct {
		name string
		args args
		want CheckInput
	}{
		{
			name: "ok",
			args: args{
				registry: "reg",
				conds: []buildv1beta1.ImageCondition{
					{
						LastTransitionTime: &now,
						Type:               buildv1beta1.ImageConditionTypeDetected,
						Status:             buildv1beta1.ImageConditionStatusTrue,
						TagPolicy:          buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision:           "master",
						ResolvedRevision:   "testrevhash",
					},
				},
			},
			want: CheckInput{
				Revisions: []Revision{
					{
						Registry:         "reg",
						ResolvedRevision: "testrevhash",
						Exist:            false,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCheckInput(tt.args.registry, tt.args.conds); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCheckFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
