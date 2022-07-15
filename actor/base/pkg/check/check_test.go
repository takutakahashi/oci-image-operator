package check

import (
	"bytes"
	"reflect"
	"strings"
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
						Exist:            false,
					},
				},
			},
			wantW: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","exist":false}]}`,
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
				r: `{"revisions":[{"registry":"reg","resolved_revision":"testrevhash","exist":false}]}`,
			},
			wantErr: false,
			want: CheckOutput{
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
