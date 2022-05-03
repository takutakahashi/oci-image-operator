package detect

import (
	"path/filepath"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestDetect_UpdateImage(t *testing.T) {
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
			if err := d.UpdateImage(); (err != nil) != tt.wantErr {
				t.Errorf("Detect.UpdateImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
