package detect

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/internal/testutil"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDetect_UpdateImage(t *testing.T) {
	c, s := testutil.Setup(testutil.NewImage())
	defer s()
	type fields struct {
		c          client.Client
		detectFile DetectFile
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		want    []buildv1beta1.ImageCondition
	}{
		{
			name: "ok",
			fields: fields{
				c: c,
				detectFile: DetectFile{
					Branches: map[string]string{
						"master": "aaa",
					},
					Tags: map[string]string{
						"latest/hash": "000011112222",
					},
				},
			},
			want: []buildv1beta1.ImageCondition{
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					Revision:         "master",
					ResolvedRevision: "aaa",
				},
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
					Type:             buildv1beta1.ImageConditionTypeChecked,
					Status:           buildv1beta1.ImageConditionStatusFalse,
					Revision:         "latest",
					ResolvedRevision: "000011112222",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detect{
				c: tt.fields.c,
				opt: DetectOpt{
					ImageName:      "test",
					ImageNamespace: "default",
				},
			}
			got, err := d.UpdateImage(context.TODO(), &tt.fields.detectFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Detect.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
			}
			time.Sleep(100 * time.Millisecond)
			savedObj := buildv1beta1.Image{}
			if err := c.Get(context.TODO(), ktypes.NamespacedName{Namespace: got.Namespace, Name: got.Name}, &savedObj); err != nil {
				t.Errorf("Detect.UpdateImage() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, savedObj.Status.Conditions, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime")); diff != "" {
				t.Error("Detect.UpdateImage() diff detected")
				t.Error(diff)
			}
		})
	}
}
