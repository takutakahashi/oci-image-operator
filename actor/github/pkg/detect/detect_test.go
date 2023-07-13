package detect

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v43/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	mygithub "github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
	"k8s.io/utils/pointer"
)

func mockhttp() *http.Client {
	return mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposBranchesByOwnerByRepoByBranch,
			github.Branch{
				Name: pointer.String("master"),
				Commit: &github.RepositoryCommit{
					SHA: pointer.String("master123master"),
				},
			},
		),
		mock.WithRequestMatch(
			mock.GetReposTagsByOwnerByRepo,
			[]github.RepositoryTag{
				{
					Name: pointer.String("v0.2"),
					Commit: &github.Commit{
						SHA: pointer.String("00002222"),
					},
				},
				{
					Name: pointer.String("v0.1"),
					Commit: &github.Commit{
						SHA: pointer.String("00001111"),
					},
				},
			},
		),
	)
}

func TestDetect_Execute(t *testing.T) {
	gh, err := mygithub.Init(&mygithub.GithubOpt{
		BaseURL:    "https://api.github.com/",
		Org:        "test",
		Repo:       "test",
		Branches:   "master",
		Tags:       "latest/hash",
		HTTPClient: mockhttp(),
	})
	if err != nil {
		panic(err)
	}
	buf := bytes.Buffer{}

	type fields struct {
		gh *mygithub.Github
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		wantBuf string
	}{
		{
			name: "ok_branch",
			fields: fields{
				gh: gh,
			},
			wantErr: false,
			wantBuf: `{"branches":{"master":"master123master"},"tags":{"latest/hash":"00002222"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detect{
				gh: tt.fields.gh,
			}
			if err := d.Execute(); (err != nil) != tt.wantErr {
				t.Errorf("Detect.Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(buf.String(), tt.wantBuf); diff != "" {
				t.Errorf("Detect.Execute() diff = %v", diff)
			}
		})
	}
}

func TestDetect_Output(t *testing.T) {
	gh, err := mygithub.Init(&mygithub.GithubOpt{
		BaseURL:    "https://api.github.com/",
		Org:        "test",
		Repo:       "test",
		Branches:   "master",
		Tags:       "latest/hash",
		HTTPClient: mockhttp(),
	})
	if err != nil {
		panic(err)
	}
	type fields struct {
		gh *mygithub.Github
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *detect.DetectFile
		wantErr bool
	}{
		{
			name: "ok_branch",
			fields: fields{
				gh: gh,
			},
			wantErr: false,
			want: &detect.DetectFile{
				Branches: map[string]string{
					"master": "master123master",
				},
				Tags: map[string]string{
					"latest/hash": "00002222",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detect{
				gh: tt.fields.gh,
			}
			got, err := d.Output(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Detect.Output() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Detect.Output() = %v, want %v", got, tt.want)
			}
		})
	}
}
