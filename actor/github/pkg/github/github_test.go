package github

import (
	"context"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-github/v43/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/types"
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
					Name: pointer.String("v0.1"),
					Commit: &github.Commit{
						SHA: pointer.String("00001111"),
					},
				},
				{
					Name: pointer.String("v0.2"),
					Commit: &github.Commit{
						SHA: pointer.String("00002222"),
					},
				},
			},
		),
	)
}

func TestGithub_BranchHash(t *testing.T) {
	os.Setenv("GITHUB_ORG", "test")
	os.Setenv("GITHUB_REPO", "test")
	os.Setenv("TARGET_BRANCHES", "master")
	type fields struct {
		opt *GithubOpt
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				opt: &GithubOpt{
					BaseURL:    "https://api.github.com/",
					Org:        "test",
					Repo:       "test",
					Branches:   "master",
					Tags:       "",
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				"master": "master123master",
			},
		},
		{
			name: "ok_env",
			fields: fields{
				opt: &GithubOpt{
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				"master": "master123master",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Init(tt.fields.opt)
			if err != nil {
				t.Errorf("Github.BranchHash() error = %v", err)
				return
			}
			got, err := g.BranchHash(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Github.BranchHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Github.BranchHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGithub_TagHash(t *testing.T) {
	os.Setenv("GITHUB_ORG", "test")
	os.Setenv("GITHUB_REPO", "test")
	os.Setenv("TARGET_TAGS", "v0.1")
	type fields struct {
		opt *GithubOpt
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				opt: &GithubOpt{
					BaseURL:    "https://api.github.com/",
					Org:        "test",
					Repo:       "test",
					Branches:   "",
					Tags:       "v0.1",
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				types.MapKeyLatestTagHash: "00001111",
				types.MapKeyLatestTagName: "v0.1",
				"v0.1":                    "00001111",
			},
		},
		{
			name: "ok_latest",
			fields: fields{
				opt: &GithubOpt{
					BaseURL:    "https://api.github.com/",
					Org:        "test",
					Repo:       "test",
					Branches:   "",
					Tags:       "",
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				types.MapKeyLatestTagHash: "00001111",
				types.MapKeyLatestTagName: "v0.1",
			},
		},
		{
			name: "ok_env",
			fields: fields{
				opt: &GithubOpt{
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				types.MapKeyLatestTagHash: "00001111",
				types.MapKeyLatestTagName: "v0.1",
				"v0.1":                    "00001111",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Init(tt.fields.opt)
			if err != nil {
				t.Errorf("Github.TagHash() error = %v", err)
				return
			}
			got, err := g.TagHash(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Github.TagHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Github.TagHash() = %v, want %v", got, tt.want)
			}
		})
	}
}
