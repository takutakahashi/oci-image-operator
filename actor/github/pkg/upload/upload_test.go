package upload

import (
	"context"
	"reflect"
	"testing"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
)

func TestUpload_Output(t *testing.T) {
	type fields struct {
		opt *github.GithubOpt
	}
	type args struct {
		ctx   context.Context
		input upload.Input
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    upload.Output
		wantErr bool
	}{
		//{
		//	name: "ok",
		//	fields: fields{
		//		opt: &github.GithubOpt{
		//			BaseURL:             "https://api.github.com/",
		//			Org:                 "takutakahashi",
		//			Repo:                "build-test",
		//			Branches:            "main",
		//			Tags:                "",
		//			WorkflowFileName:    "build.yaml",
		//			PersonalAccessToken: os.Getenv("GITHUB_TOKEN"),
		//			HTTPClient:          nil,
		//		},
		//	},
		//	args: args{
		//		ctx: context.Background(),
		//		input: upload.Input{
		//			Builds: []upload.ImageBuild{
		//				{Target: "hoge", Tag: "59507e6468921d235e0078728c550e525c075f7c"},
		//				{Target: "hoge", Tag: "48edf3c8bae2395e1b2b6331ba1bd072889061a5"},
		//			},
		//		},
		//	},
		//	want: upload.Output{
		//		Builds: []upload.ImageBuild{
		//			{Target: "hoge", Tag: "59507e6468921d235e0078728c550e525c075f7c", Succeeded: v1beta1.ImageConditionStatusTrue},
		//			{Target: "hoge", Tag: "48edf3c8bae2395e1b2b6331ba1bd072889061a5", Succeeded: v1beta1.ImageConditionStatusTrue},
		//		},
		//	},
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := github.Init(tt.fields.opt)
			if err != nil {
				t.Errorf("Upload.Output() error = %v", err)
				return
			}
			u := Upload{
				gh: g,
			}
			got, err := u.Output(tt.args.ctx, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Upload.Output() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Upload.Output() = %v, want %v", got, tt.want)
			}
		})
	}
}
