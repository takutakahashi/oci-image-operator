package detect

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/internal/testutil"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDetect_UpdateImage(t *testing.T) {
	c, s := testutil.Setup(testutil.NewImage())
	defer s()
	type fields struct {
		c         client.Client
		watchPath string
		json      string
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
				c:         c,
				watchPath: "/tmp/github-actor/detect",
				json:      `{"branches":{"master":"aaa"},"tags":{"latest/hash":"000011112222"}}`,
			},
			want: []buildv1beta1.ImageCondition{
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Type:             buildv1beta1.ImageConditionTypeDetected,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					Revision:         "master",
					ResolvedRevision: "aaa",
				},
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
					Type:             buildv1beta1.ImageConditionTypeDetected,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					Revision:         "latest",
					ResolvedRevision: "000011112222",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f io.Reader = nil
			if tt.fields.json != "" {
				f = strings.NewReader(tt.fields.json)
			}
			d := &Detect{
				c: tt.fields.c,
				f: f,
				opt: DetectOpt{
					WatchPath:      tt.fields.watchPath,
					ImageName:      "test",
					ImageNamespace: "default",
				},
			}
			got, err := d.UpdateImage(context.TODO())
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

func TestDetect_Run(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)
	c, s := testutil.Setup(testutil.NewImage())
	defer s()
	type fields struct {
		json string
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
				json: `{"branches":{"master":"aaa"},"tags":{"latest/hash":"000011112222"}}`,
			},
			want: []buildv1beta1.ImageCondition{
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeBranchHash,
					Type:             buildv1beta1.ImageConditionTypeDetected,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					Revision:         "master",
					ResolvedRevision: "aaa",
				},
				{
					TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
					Type:             buildv1beta1.ImageConditionTypeDetected,
					Status:           buildv1beta1.ImageConditionStatusTrue,
					Revision:         "latest",
					ResolvedRevision: "000011112222",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
			f, err := os.CreateTemp(".", "test")
			if err != nil {
				panic(err)
			}
			defer os.Remove(f.Name())
			d := &Detect{
				c: c,
				opt: DetectOpt{
					WatchPath:      f.Name(),
					ImageName:      "test",
					ImageNamespace: "default",
				},
			}
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer d.Stop()
				_, err := f.Write(bytes.NewBufferString(tt.fields.json).Bytes())
				if err != nil {
					t.Error(err)
				}
				savedObj := buildv1beta1.Image{}
				time.Sleep(2000 * time.Millisecond)
				if err := c.Get(context.TODO(), ktypes.NamespacedName{Name: "test", Namespace: "default"}, &savedObj); err != nil {
					panic(err)
				}
				if diff := cmp.Diff(tt.want, savedObj.Status.Conditions, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime")); diff != "" {
					t.Error("diff detected")
					t.Error(diff)
				}
			}()
			if err := d.Run(ctx); (err != nil) != tt.wantErr {
				t.Errorf("Detect.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			wg.Wait()
			cancel()
		})
	}
}
