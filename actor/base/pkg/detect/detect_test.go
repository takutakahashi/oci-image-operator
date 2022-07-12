package detect

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func newImage() *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageSpec{
			TemplateName: "test",
			Repository: buildv1beta1.ImageRepository{
				URL: "https://github.com/taktuakahashi/testbed.git",
				TagPolicies: []buildv1beta1.ImageTagPolicy{
					{
						Policy:   buildv1beta1.ImageTagPolicyTypeTagHash,
						Revision: "master",
					},
				},
			},
			Targets: []buildv1beta1.ImageTarget{
				{
					Name: "ghcr.io/takutakahashi/test",
					//Auth: buildv1beta1.ImageAuth{
					//	Type:       buildv1beta1.ImageAuthTypeBasic,
					//	SecretName: "test",
					//},
				},
			},
		},
	}
}

func setup() (client.Client, func() error) {
	os.Setenv("IMAGE_NAME", "test")
	os.Setenv("IMAGE_NAMESPACE", "default")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../../../..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		panic(err)
	}
	ctx := context.TODO()
	err = c.Create(ctx, newImage())
	if err != nil {
		panic(err)
	}
	return c, testEnv.Stop
}

func TestDetect_UpdateImage(t *testing.T) {
	c, s := setup()
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
		want    buildv1beta1.ImageCondition
	}{
		{
			name: "ok",
			fields: fields{
				c:         c,
				watchPath: "/tmp/github-actor/detect",
				json:      `{"branches":{"master":"aaa"},"tags":{"latest/hash":"000011112222"}}`,
			},
			want: buildv1beta1.ImageCondition{
				TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
				Revision:         "master",
				ResolvedRevision: "000011112222",
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
			if diff := cmp.Diff(tt.want, savedObj.Spec.Repository.TagPolicies); diff != "" {
				t.Error("Detect.UpdateImage() diff detected")
				t.Error(diff)
			}
		})
	}
}

func TestDetect_Run(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)
	c, s := setup()
	defer s()
	type fields struct {
		json string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		want    buildv1beta1.ImageCondition
	}{
		{
			name: "ok",
			fields: fields{
				json: `{"branches":{"master":"aaa"},"tags":{"latest/hash":"000011112222"}}`,
			},
			want: buildv1beta1.ImageCondition{
				TagPolicy:        buildv1beta1.ImageTagPolicyTypeTagHash,
				Revision:         "master",
				ResolvedRevision: "000011112222",
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
				t.Log(savedObj.Spec.Repository.TagPolicies)
				if diff := cmp.Diff(tt.want, savedObj.Spec.Repository.TagPolicies); diff != "" {
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
