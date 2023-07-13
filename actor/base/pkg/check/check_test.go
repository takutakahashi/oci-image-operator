package check

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/internal/testutil"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
						Type:               buildv1beta1.ImageConditionTypeChecked,
						Status:             buildv1beta1.ImageConditionStatusFalse,
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
						Revision:         "master",
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

func TestCheckInput_Export(t *testing.T) {
	type fields struct {
		Revisions []Revision
	}
	tests := []struct {
		name    string
		fields  fields
		wantW   string
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				Revisions: []Revision{
					{
						Registry:         "reg",
						ResolvedRevision: "testrevhash",
						Revision:         "master",
						Exist:            buildv1beta1.ImageConditionStatusFalse,
					},
				},
			},
			wantW: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","revision":"master","exist":"False"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CheckInput{
				Revisions: tt.fields.Revisions,
			}
			w := &bytes.Buffer{}
			if err := c.Export(w); (err != nil) != tt.wantErr {
				t.Errorf("CheckInput.Export() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("CheckInput.Export() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}

func TestImportOutput(t *testing.T) {
	type args struct {
		r string
	}
	tests := []struct {
		name    string
		args    args
		want    CheckOutput
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				r: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","revision":"master","exist":"False"}]}`,
			},
			wantErr: false,
			want: CheckOutput{
				Revisions: []Revision{
					{
						Registry:         "reg",
						ResolvedRevision: "testrevhash",
						Revision:         "master",
						Exist:            buildv1beta1.ImageConditionStatusFalse,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.args.r)
			got, err := ImportOutput(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImportOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ImportOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_UpdateImage(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		opt CheckOpt
	}
	type args struct {
		ctx    context.Context
		image  *buildv1beta1.Image
		output CheckOutput
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		wantErr       bool
		wantCondition []buildv1beta1.ImageCondition
	}{
		{
			name: "ok",
			fields: fields{
				opt: CheckOpt{
					ImageName:      "test",
					ImageNamespace: "default",
					ImageTarget:    "target",
				},
			},
			args: args{
				ctx: ctx,
				output: CheckOutput{
					Revisions: []Revision{
						{
							Registry:         "reg",
							ResolvedRevision: "resolved",
							Revision:         "master",
							Exist:            buildv1beta1.ImageConditionStatusFalse,
						},
					},
				},
			},
			wantCondition: []buildv1beta1.ImageCondition{
				{
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					ResolvedRevision: "resolved",
					Revision:         "master",
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
				},
				{
					Type:             buildv1beta1.ImageConditionTypeUploaded,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeUnused,
					ResolvedRevision: "resolved",
					Revision:         "master",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.image == nil {
				tt.args.image = testutil.NewImage()
			}
			cli, s := testutil.Setup(tt.args.image)
			c := &Check{
				c:   cli,
				opt: tt.fields.opt,
			}
			defer s()
			if err := c.c.Get(ctx, types.NamespacedName{Name: tt.args.image.Name, Namespace: tt.args.image.Namespace}, tt.args.image); err != nil {
				t.Errorf("failed to get image")
			}
			if err := c.UpdateImage(tt.args.ctx, tt.args.image, tt.args.output); (err != nil) != tt.wantErr {
				t.Errorf("Check.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
			}
			savedObj, err := base.GetImage(ctx, c.c, c.opt.ImageName, c.opt.ImageNamespace)
			if err != nil {
				t.Errorf("Check.UpdateImage() error = %v", err)
			}
			if d := cmp.Diff(savedObj.Status.Conditions, tt.wantCondition, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime")); d != "" {
				t.Errorf("Diff detected. %v", d)
			}
		})
	}
}
