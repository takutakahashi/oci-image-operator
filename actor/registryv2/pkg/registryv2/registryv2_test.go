package registryv2

import (
	"net/http"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestRegistry_TagExists(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)
	type args struct {
		tag string
	}
	type fields struct {
		c   *http.Client
		opt Opt
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				opt: Opt{
					URL:   "https://ghcr.io",
					Image: "takutakahashi/oci-image-operator/manager",
					Auth: &Auth{
						Username: "takutakahashi",
						Password: os.Getenv("GITHUB_TOKEN"),
					},
				},
				c: &http.Client{},
			},
			args: args{
				tag: "v0.1.9",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ng",
			fields: fields{
				opt: Opt{
					URL:   "https://ghcr.io",
					Image: "takutakahashi/oci-image-operator/manager",
					Auth: &Auth{
						Username: "takutakahashi",
						Password: os.Getenv("GITHUB_TOKEN"),
					},
				},
				c: &http.Client{},
			},
			args: args{
				tag: "vvvvv-error",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Registry{
				c:   tt.fields.c,
				opt: tt.fields.opt,
			}
			got, err := r.TagExists(tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.TagExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Registry.TagExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
