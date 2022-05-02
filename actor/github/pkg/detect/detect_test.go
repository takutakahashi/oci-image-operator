package detect

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v43/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
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

func TestDetect_Execute(t *testing.T) {
	gh, err := mygithub.Init(&mygithub.GithubOpt{
		BaseURL:    "https://api.github.com/",
		Org:        "test",
		Repo:       "test",
		Branches:   "master",
		HTTPClient: mockhttp(),
	})
	if err != nil {
		panic(err)
	}
	buf := bytes.Buffer{}

	type fields struct {
		gh *mygithub.Github
		w  io.Writer
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
		wantBuf string
	}{
		{
			name: "ok",
			fields: fields{
				gh: gh,
				w:  &buf,
			},
			wantErr: false,
			wantBuf: `{"branches":{"master":"master123master"},"tags":{"latest":"00001111"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detect{
				gh: tt.fields.gh,
				w:  tt.fields.w,
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
