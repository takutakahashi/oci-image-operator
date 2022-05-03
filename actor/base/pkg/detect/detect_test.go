package detect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestDetect_UpdateImage(t *testing.T) {
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
	c, err := genClient(cfg)
	if err != nil {
		panic(err)
	}
	ctx := context.TODO()
	err = c.Create(ctx, newImage())
	if err != nil {
		panic(err)
	}
	type fields struct {
		c         client.Client
		watchPath string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:   "ok",
			fields: fields{c: c, watchPath: "/tmp/github-actor/detect"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detect{
				c:         tt.fields.c,
				watchPath: tt.fields.watchPath,
			}
			if _, err := d.UpdateImage(); (err != nil) != tt.wantErr {
				t.Errorf("Detect.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

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
