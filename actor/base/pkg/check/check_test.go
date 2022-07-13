package check

import (
	"reflect"
	"testing"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

func TestGetCheckFile(t *testing.T) {
	type args struct {
		conds []buildv1beta1.ImageCondition
	}
	tests := []struct {
		name string
		args args
		want CheckFile
	}{
		{
			name: "ok",
			args: args{
				conds: []buildv1beta1.ImageCondition{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetCheckFile(tt.args.conds); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCheckFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
