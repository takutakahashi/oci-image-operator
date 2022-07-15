package check

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
						Exist:            buildv1beta1.ImageConditionStatusFalse,
					},
				},
			},
			wantW: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","exist":"False"}]}`,
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
				r: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","exist":"False"}]}`,
			},
			wantErr: false,
			want: CheckOutput{
				Revisions: []Revision{
					{
						Registry:         "reg",
						ResolvedRevision: "testrevhash",
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
	type fields struct {
		c   client.Client
		ch  chan bool
		opt CheckOpt
		in  io.Writer
		out io.Reader
	}
	type args struct {
		ctx    context.Context
		image  *buildv1beta1.Image
		output CheckOutput
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Check{
				c:   tt.fields.c,
				ch:  tt.fields.ch,
				opt: tt.fields.opt,
				in:  tt.fields.in,
				out: tt.fields.out,
			}
			if err := c.UpdateImage(tt.args.ctx, tt.args.image, tt.args.output); (err != nil) != tt.wantErr {
				t.Errorf("Check.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
