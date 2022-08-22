package external

import "testing"

func TestParseImageName(t *testing.T) {
	type args struct {
		image string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				image: "ghcr.io/takutakahashi/oci-image-operator/manager:v0.1",
			},
			want:  "ghcr.io",
			want1: "takutakahashi/oci-image-operator/manager",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseImageName(tt.args.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseImageName() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseImageName() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
