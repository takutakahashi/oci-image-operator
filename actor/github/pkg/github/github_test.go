package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-github/v43/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
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
	os.Setenv("GITHUB_API_URL", "https://api.github.com/")
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
	os.Setenv("GITHUB_API_URL", "https://api.github.com/")
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
				"v0.1": "00001111",
			},
		},
		{
			name: "ok_empty",
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
			want: map[string]string{},
		},
		{
			name: "ok_latest_tag_hash",
			fields: fields{
				opt: &GithubOpt{
					BaseURL:    "https://api.github.com/",
					Org:        "test",
					Repo:       "test",
					Branches:   "",
					Tags:       "latest/hash",
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				detect.MapKeyLatestTagHash: "00001111",
			},
		},
		{
			name: "ok_latest_tag_name",
			fields: fields{
				opt: &GithubOpt{
					BaseURL:    "https://api.github.com/",
					Org:        "test",
					Repo:       "test",
					Branches:   "",
					Tags:       "latest/name",
					HTTPClient: mockhttp(),
				},
			},
			args: args{
				ctx: context.TODO(),
			},
			want: map[string]string{
				detect.MapKeyLatestTagName: "v0.1",
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
				"v0.1": "00001111",
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

func TestGithub_runNameMatches(t *testing.T) {
	tests := []struct {
		name       string
		regex      string
		runName    string
		dispatchID string
		want       bool
	}{
		{
			name:       "default_match",
			runName:    "docker_push_builder abc123 [f00ba4]",
			dispatchID: "f00ba4",
			want:       true,
		},
		{
			name:       "default_unmatch",
			runName:    "docker_push_builder abc123 [deadbeef]",
			dispatchID: "f00ba4",
			want:       false,
		},
		{
			name:       "default_unmatch_without_brackets",
			runName:    "docker_push_builder abc123 f00ba4",
			dispatchID: "f00ba4",
			want:       false,
		},
		{
			name:       "regex_match",
			regex:      `dispatch_id=([0-9a-f]+)`,
			runName:    "build abc123 dispatch_id=f00ba4",
			dispatchID: "f00ba4",
			want:       true,
		},
		{
			name:       "regex_unmatch",
			regex:      `dispatch_id=([0-9a-f]+)`,
			runName:    "build abc123 dispatch_id=deadbeef",
			dispatchID: "f00ba4",
			want:       false,
		},
		{
			name:       "regex_no_match_in_name",
			regex:      `dispatch_id=([0-9a-f]+)`,
			runName:    "build abc123",
			dispatchID: "f00ba4",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Init(&GithubOpt{
				BaseURL:      "https://api.github.com/",
				Org:          "test",
				Repo:         "test",
				RunNameRegex: tt.regex,
				HTTPClient:   mockhttp(),
			})
			if err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			if got := g.runNameMatches(tt.runName, tt.dispatchID); got != tt.want {
				t.Errorf("runNameMatches(%q, %q) = %v, want %v", tt.runName, tt.dispatchID, got, tt.want)
			}
		})
	}
}

func TestGithub_Init_InvalidRunNameRegex(t *testing.T) {
	tests := []struct {
		name  string
		regex string
	}{
		{name: "broken_regex", regex: `dispatch_id=([`},
		{name: "no_capture_group", regex: `dispatch_id=[0-9a-f]+`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Init(&GithubOpt{
				BaseURL:      "https://api.github.com/",
				Org:          "test",
				Repo:         "test",
				RunNameRegex: tt.regex,
				HTTPClient:   mockhttp(),
			})
			if err == nil {
				t.Errorf("Init() with regex %q should return error", tt.regex)
			}
		})
	}
}

func mockExecuteRun(dispatchStatus func(callCount int, inputs map[string]interface{}) int, dispatchedID *string) *http.Client {
	dispatchCalls := 0
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
		mock.WithRequestMatchHandler(
			mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				dispatchCalls++
				body, _ := io.ReadAll(r.Body)
				var req struct {
					Ref    string                 `json:"ref"`
					Inputs map[string]interface{} `json:"inputs"`
				}
				_ = json.Unmarshal(body, &req)
				if id, ok := req.Inputs["dispatch_id"].(string); ok {
					*dispatchedID = id
				}
				status := dispatchStatus(dispatchCalls, req.Inputs)
				if status == http.StatusUnprocessableEntity {
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"message":"Unexpected inputs provided: [\"dispatch_id\"]"}`))
					return
				}
				w.WriteHeader(status)
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowId,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				runs := github.WorkflowRuns{
					TotalCount: pointer.Int(2),
					WorkflowRuns: []*github.WorkflowRun{
						{
							ID:        pointer.Int64(999),
							Name:      pointer.String("docker_push_builder other [deadbeef]"),
							Status:    pointer.String("queued"),
							CreatedAt: &github.Timestamp{Time: time.Now()},
						},
						{
							ID:        pointer.Int64(100),
							Name:      pointer.String("docker_push_builder rev123 [" + *dispatchedID + "]"),
							Status:    pointer.String("queued"),
							CreatedAt: &github.Timestamp{Time: time.Now()},
						},
					},
				}
				b, _ := json.Marshal(runs)
				_, _ = w.Write(b)
			}),
		),
	)
}

func TestGithub_ExecuteRun_IdentifiesRunByDispatchID(t *testing.T) {
	dispatchedID := ""
	cli := mockExecuteRun(func(_ int, _ map[string]interface{}) int {
		return http.StatusNoContent
	}, &dispatchedID)
	g, err := Init(&GithubOpt{
		BaseURL:          "https://api.github.com/",
		Org:              "test",
		Repo:             "test",
		WorkflowFileName: "build.yaml",
		HTTPClient:       cli,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	run, err := g.ExecuteRun(context.Background(), "rev123")
	if err != nil {
		t.Fatalf("ExecuteRun() error = %v", err)
	}
	if dispatchedID == "" {
		t.Fatal("dispatch_id input was not sent")
	}
	if run.GetID() != 100 {
		t.Errorf("ExecuteRun() returned run ID %d, want 100", run.GetID())
	}
}

func TestGithub_ExecuteRun_FallbackOnUnexpectedInputs(t *testing.T) {
	dispatchedID := ""
	cli := mockExecuteRun(func(callCount int, inputs map[string]interface{}) int {
		if _, ok := inputs["dispatch_id"]; ok {
			return http.StatusUnprocessableEntity
		}
		return http.StatusNoContent
	}, &dispatchedID)
	g, err := Init(&GithubOpt{
		BaseURL:          "https://api.github.com/",
		Org:              "test",
		Repo:             "test",
		WorkflowFileName: "build.yaml",
		HTTPClient:       cli,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	run, err := g.ExecuteRun(context.Background(), "rev123")
	if err != nil {
		t.Fatalf("ExecuteRun() error = %v", err)
	}
	// legacy fallback returns the latest run
	if run.GetID() != 999 {
		t.Errorf("ExecuteRun() returned run ID %d, want 999 (latest run)", run.GetID())
	}
}

func TestGithub_ExecuteRun_DispatchIDDisabled(t *testing.T) {
	dispatchedID := ""
	cli := mockExecuteRun(func(_ int, inputs map[string]interface{}) int {
		if _, ok := inputs["dispatch_id"]; ok {
			t.Error("dispatch_id must not be sent when disabled")
		}
		return http.StatusNoContent
	}, &dispatchedID)
	g, err := Init(&GithubOpt{
		BaseURL:            "https://api.github.com/",
		Org:                "test",
		Repo:               "test",
		WorkflowFileName:   "build.yaml",
		DispatchIDDisabled: true,
		HTTPClient:         cli,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	run, err := g.ExecuteRun(context.Background(), "rev123")
	if err != nil {
		t.Fatalf("ExecuteRun() error = %v", err)
	}
	if run.GetID() != 999 {
		t.Errorf("ExecuteRun() returned run ID %d, want 999 (latest run)", run.GetID())
	}
}

func TestGithub_Dispatch(t *testing.T) {
	type fields struct {
		opt *GithubOpt
	}
	type args struct {
		ctx  context.Context
		ref  string
		wait bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Mock
		//{
		//	name: "ok",
		//	fields: fields{
		//		opt: &GithubOpt{
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
		//		ctx:  context.Background(),
		//		ref:  "main",
		//		wait: true,
		//	},
		//	wantErr: false,
		//},
		//{
		//	name: "error",
		//	fields: fields{
		//		opt: &GithubOpt{
		//			BaseURL:             "https://api.github.com/",
		//			Org:                 "takutakahashi",
		//			Repo:                "build-test",
		//			Branches:            "main",
		//			Tags:                "",
		//			WorkflowFileName:    "error.yaml",
		//			PersonalAccessToken: os.Getenv("GITHUB_TOKEN"),
		//			HTTPClient:          nil,
		//		},
		//	},
		//	args: args{
		//		ctx:  context.Background(),
		//		ref:  "main",
		//		wait: true,
		//	},
		//	wantErr: true,
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := Init(tt.fields.opt)
			if err != nil {
				t.Errorf("Github.Dispatch() error = %v", err)
				return
			}
			if err := g.Dispatch(tt.args.ctx, tt.args.ref, tt.args.wait); (err != nil) != tt.wantErr {
				t.Errorf("Github.Dispatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
