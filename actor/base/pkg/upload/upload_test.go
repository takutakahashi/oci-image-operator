package upload

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/internal/testutil"
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

func TestUpload_UpdateImage(t *testing.T) {
	type fields struct {
		ch  chan bool
		in  io.Writer
		out io.Reader
		opt Opt
	}
	type args struct {
		ctx    context.Context
		image  *buildv1beta1.Image
		output *Output
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantErr        bool
		wantConditions []buildv1beta1.ImageCondition
	}{
		{
			name: "ok",
			args: args{
				ctx: context.Background(),
				image: &buildv1beta1.Image{
					Status: buildv1beta1.ImageStatus{
						Conditions: []buildv1beta1.ImageCondition{
							{
								Type:             buildv1beta1.ImageConditionTypeChecked,
								Status:           buildv1beta1.ImageConditionStatusTrue,
								ResolvedRevision: "uploadimage",
								Revision:         "aaa",
								TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
							},
							{
								Type:             buildv1beta1.ImageConditionTypeUploaded,
								Status:           buildv1beta1.ImageConditionStatusFalse,
								ResolvedRevision: "uploadimage",
								TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
							},
						},
					},
				},
				output: &Output{
					Builds: []ImageBuild{
						{
							Target:    "targetimage",
							Tag:       "uploadimage",
							Succeeded: buildv1beta1.ImageConditionStatusTrue,
						},
					},
				},
			},
			wantConditions: []buildv1beta1.ImageCondition{
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					ResolvedRevision: "uploadimage",
					Revision:         "aaa",
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					ResolvedRevision: "uploadimage",
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := testutil.NewImage()
			c, s := testutil.Setup(image)
			defer s()
			image, err := base.GetImage(tt.args.ctx, c, image.Name, image.Namespace)
			if err != nil {
				t.Errorf("Upload.UpdateImage() error = %v", err)
				return
			}
			image.Status = tt.args.image.Status
			u := &Upload{
				c:   c,
				ch:  tt.fields.ch,
				in:  tt.fields.in,
				out: tt.fields.out,
				opt: tt.fields.opt,
			}
			if err := u.UpdateImage(tt.args.ctx, image, tt.args.output); (err != nil) != tt.wantErr {
				t.Errorf("Upload.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			image, err = base.GetImage(tt.args.ctx, c, image.Name, image.Namespace)
			if err != nil {
				t.Errorf("Upload.UpdateImage() error = %v", err)
				return
			}

			if diff := cmp.Diff(image.Status.Conditions, tt.wantConditions, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime")); diff != "" {
				t.Errorf("diff detected. %v", diff)
				return
			}

		})
	}
}
