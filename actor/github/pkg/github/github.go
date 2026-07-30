package github

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Netflix/go-env"
	"github.com/google/go-github/v43/github"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"golang.org/x/oauth2"
	"k8s.io/utils/strings/slices"
)

type GithubOpt struct {
	BaseURL             string `env:"GITHUB_API_URL,default=https://api.github.com/"`
	Org                 string `env:"GITHUB_ORG,required=true"`
	Repo                string `env:"GITHUB_REPO,required=true"`
	Branches            string `env:"TARGET_BRANCHES"`
	Tags                string `env:"TARGET_TAGS"`
	PersonalAccessToken string `env:"GITHUB_TOKEN"`
	WorkflowFileName    string `env:"GITHUB_WORKFLOW_FILENAME,default=build.yaml"`
	// DispatchIDInput is the workflow_dispatch input name used to pass a unique ID
	// so that the dispatched run can be identified from the run list.
	DispatchIDInput string `env:"GITHUB_DISPATCH_ID_INPUT,default=dispatch_id"`
	// DispatchIDDisabled disables sending the dispatch ID input entirely and
	// falls back to the legacy "latest run" detection.
	DispatchIDDisabled bool `env:"GITHUB_DISPATCH_ID_DISABLED,default=false"`
	// RunNameRegex optionally extracts the dispatch ID from the run name.
	// It must contain exactly one capture group which captures the dispatch ID.
	// When empty, the run name is matched by containing "[<dispatch ID>]".
	RunNameRegex string `env:"GITHUB_RUN_NAME_REGEX"`
	HTTPClient   *http.Client
}

type Github struct {
	c         *github.Client
	opt       *GithubOpt
	branches  []string
	tags      []string
	revs      map[string]string
	runNameRe *regexp.Regexp
}

func Init(opt *GithubOpt) (*Github, error) {
	if opt.Org == "" {
		newOpt, err := GenOpt(opt.HTTPClient)
		if err != nil {
			return nil, err
		}
		opt = newOpt
	}
	if opt.HTTPClient == nil {
		httpcli := &http.Client{}
		if opt.PersonalAccessToken != "" {
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: opt.PersonalAccessToken},
			)
			httpcli = oauth2.NewClient(context.Background(), ts)
		}
		opt.HTTPClient = httpcli
	}
	c := github.NewClient(opt.HTTPClient)
	baseURL, err := url.Parse(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	c.BaseURL = baseURL
	b, t := []string{}, []string{}
	if opt.Branches != "" {
		b = strings.Split(opt.Branches, ",")
	}
	if opt.Tags != "" {
		t = strings.Split(opt.Tags, ",")
	}
	if opt.DispatchIDInput == "" {
		opt.DispatchIDInput = "dispatch_id"
	}
	var runNameRe *regexp.Regexp
	if opt.RunNameRegex != "" {
		re, err := regexp.Compile(opt.RunNameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid GITHUB_RUN_NAME_REGEX %q: %w", opt.RunNameRegex, err)
		}
		if re.NumSubexp() < 1 {
			return nil, fmt.Errorf("GITHUB_RUN_NAME_REGEX %q must contain a capture group for the dispatch ID", opt.RunNameRegex)
		}
		runNameRe = re
	}
	return &Github{c: c, opt: opt, branches: b, tags: t, revs: map[string]string{}, runNameRe: runNameRe}, nil
}

func GenOpt(httpClient *http.Client) (*GithubOpt, error) {
	var opt GithubOpt
	_, err := env.UnmarshalFromEnviron(&opt)
	if err != nil {
		return nil, err
	}
	opt.HTTPClient = httpClient
	return &opt, err
}

func (g Github) BranchHash(ctx context.Context) (map[string]string, error) {
	if len(g.branches) == 0 {
		return map[string]string{}, nil
	}
	for _, b := range g.branches {
		branch, _, err := g.c.Repositories.GetBranch(
			ctx, g.opt.Org, g.opt.Repo, b, true)
		if err != nil {
			return nil, err
		}
		g.setBranchHash(b, branch.GetCommit().GetSHA())
	}
	return g.getBranchHashes(), nil
}
func (g Github) TagHash(ctx context.Context) (map[string]string, error) {
	if len(g.tags) == 0 {
		return map[string]string{}, nil
	}
	tags, _, err := g.c.Repositories.ListTags(
		ctx, g.opt.Org, g.opt.Repo, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return map[string]string{}, nil
	}
	for _, t := range g.tags {
		if t == detect.MapKeyLatestTagHash {
			g.setTagHash(detect.MapKeyLatestTagHash, tags[0].GetCommit().GetSHA())
			break
		}
		if t == detect.MapKeyLatestTagName {
			g.setTagHash(detect.MapKeyLatestTagName, tags[0].GetName())
			break
		}
		for _, tag := range tags {
			if tag.GetName() == t {
				g.setTagHash(t, tag.GetCommit().GetSHA())
			}
		}
	}
	return g.getTagHashes(), nil
}

func (g *Github) setBranchHash(branch, hash string) {
	g.setHash("branch", branch, hash)
}
func (g *Github) setTagHash(tag, hash string) {
	g.setHash("tag", tag, hash)
}

func (g *Github) setHash(t, v, hash string) {
	g.revs[fmt.Sprintf("%s/%s", t, v)] = hash

}

func (g *Github) getBranchHashes() map[string]string {
	return g.getHashes("branch")
}

func (g *Github) getTagHashes() map[string]string {
	return g.getHashes("tag")
}

func (g *Github) getHashes(t string) map[string]string {
	ret := map[string]string{}
	for k, v := range g.revs {
		if strings.Contains(k, t) {
			ret[strings.TrimLeft(k, fmt.Sprintf("%s/", t))] = v
		}
	}
	return ret

}

func (g *Github) Dispatch(ctx context.Context, ref string, wait bool) error {
	run, err := g.ExecuteRun(ctx, ref)
	if err != nil {
		return err
	}
	if wait {
		return g.waitForComplete(ctx, run)
	}
	return nil
}

func (g *Github) ExecuteRun(ctx context.Context, ref string) (*github.WorkflowRun, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	branch, err := g.defaultBranch(ctx)
	if err != nil {
		return nil, err
	}
	dispatchID, err := g.dispatch(ctx, branch.GetName(), ref)
	if err != nil {
		return nil, err
	}
	if dispatchID != "" {
		findCtx, findCancel := context.WithTimeout(ctx, 30*time.Second)
		defer findCancel()
		run, err := g.findRunByDispatchID(findCtx, dispatchID)
		if err == nil {
			return run, nil
		}
		logrus.Warnf("failed to identify run by dispatch ID, falling back to legacy run detection: %v", err)
	}
	return g.findRunLegacy(ctx)
}

func (g *Github) defaultBranch(ctx context.Context) (*github.Branch, error) {
	branch, _, _ := g.c.Repositories.GetBranch(
		ctx,
		g.opt.Org,
		g.opt.Repo,
		"master",
		false,
	)
	if branch == nil {
		var err error = nil
		branch, _, err = g.c.Repositories.GetBranch(
			ctx,
			g.opt.Org,
			g.opt.Repo,
			"main",
			false,
		)
		if err != nil {
			return nil, err
		}
	}
	if branch == nil {
		return nil, fmt.Errorf("default branch must be main or master")
	}
	return branch, nil
}

// dispatch triggers the workflow. It returns the dispatch ID passed to the
// workflow, or an empty string when the run must be detected in the legacy way.
func (g *Github) dispatch(ctx context.Context, branchName, ref string) (string, error) {
	dispatchID := ""
	inputs := map[string]interface{}{
		"revision": ref,
	}
	if !g.opt.DispatchIDDisabled {
		dispatchID = newDispatchID()
		inputs[g.opt.DispatchIDInput] = dispatchID
	}
	res, err := g.c.Actions.CreateWorkflowDispatchEventByFileName(
		ctx,
		g.opt.Org,
		g.opt.Repo,
		g.opt.WorkflowFileName,
		github.CreateWorkflowDispatchEventRequest{
			Ref:    branchName,
			Inputs: inputs,
		},
	)
	if err != nil && dispatchID != "" && isUnexpectedInputsError(err) {
		logrus.Warnf("workflow %s does not accept input %q, dispatching without it: %v", g.opt.WorkflowFileName, g.opt.DispatchIDInput, err)
		dispatchID = ""
		delete(inputs, g.opt.DispatchIDInput)
		res, err = g.c.Actions.CreateWorkflowDispatchEventByFileName(
			ctx,
			g.opt.Org,
			g.opt.Repo,
			g.opt.WorkflowFileName,
			github.CreateWorkflowDispatchEventRequest{
				Ref:    branchName,
				Inputs: inputs,
			},
		)
	}
	if err != nil {
		return "", err
	}
	if res.StatusCode != 204 {
		return "", fmt.Errorf("dispatch failed: %s", res.Status)
	}
	return dispatchID, nil
}

func isUnexpectedInputsError(err error) bool {
	var errRes *github.ErrorResponse
	if !errors.As(err, &errRes) {
		return false
	}
	return errRes.Response != nil &&
		errRes.Response.StatusCode == http.StatusUnprocessableEntity &&
		strings.Contains(errRes.Message, "Unexpected inputs")
}

func newDispatchID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// findRunByDispatchID locates the run dispatched by us. The workflow is
// expected to echo the dispatch ID input into its run-name, which is exposed
// as the run name in the API even while the run stays queued.
func (g *Github) findRunByDispatchID(ctx context.Context, dispatchID string) (*github.WorkflowRun, error) {
	for {
		runs, _, err := g.c.Actions.ListWorkflowRunsByFileName(
			ctx,
			g.opt.Org,
			g.opt.Repo,
			g.opt.WorkflowFileName,
			&github.ListWorkflowRunsOptions{
				Event: "workflow_dispatch",
				ListOptions: github.ListOptions{
					PerPage: 30,
				},
			},
		)
		if err != nil {
			return nil, err
		}
		for _, run := range runs.WorkflowRuns {
			if g.runNameMatches(run.GetName(), dispatchID) {
				return run, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("no run matched dispatch ID %s: %w", dispatchID, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func (g *Github) runNameMatches(name, dispatchID string) bool {
	if g.runNameRe != nil {
		m := g.runNameRe.FindStringSubmatch(name)
		return len(m) >= 2 && m[1] == dispatchID
	}
	return strings.Contains(name, fmt.Sprintf("[%s]", dispatchID))
}

// findRunLegacy assumes the latest run of the workflow is ours. This is
// unreliable when multiple dispatches for the same workflow run concurrently
// and is kept only as a fallback for workflows without run-name support.
func (g *Github) findRunLegacy(ctx context.Context) (*github.WorkflowRun, error) {
	waiting := []string{"queued", "in_progress", "waiting"}
	expectedTime := time.Now().Add(-1 * time.Minute)
	for {
		nowRuns, _, err := g.c.Actions.ListWorkflowRunsByFileName(
			ctx,
			g.opt.Org,
			g.opt.Repo,
			g.opt.WorkflowFileName,
			&github.ListWorkflowRunsOptions{
				ListOptions: github.ListOptions{
					PerPage: 1,
				},
			},
		)
		if err != nil {
			return nil, err
		}
		if len(nowRuns.WorkflowRuns) > 0 {
			run := nowRuns.WorkflowRuns[0]
			if slices.Contains(waiting, run.GetStatus()) && run.GetCreatedAt().After(expectedTime) {
				return run, nil
			}
			logrus.Info("latest run is not our run")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (g *Github) cancelRun(ctx context.Context, ourRun *github.WorkflowRun) error {
	res, err := g.c.Actions.CancelWorkflowRunByID(
		ctx,
		g.opt.Org,
		g.opt.Repo,
		ourRun.GetID(),
	)
	if err != nil || res.StatusCode != 202 {
		return fmt.Errorf("failed to cancel workflow, id: %d", ourRun.GetID())
	}
	logrus.Info("cancelled")
	return nil
}

func (g *Github) waitForComplete(ctx context.Context, ourRun *github.WorkflowRun) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	done := make(chan error, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigs
		logrus.Infof("%v recieved", s)
		done <- g.cancelRun(ctx, ourRun)
	}()
	go func() {
		for {
			time.Sleep(3 * time.Second)
			run, _, err := g.c.Actions.GetWorkflowRunByID(
				ctx,
				g.opt.Org,
				g.opt.Repo,
				ourRun.GetID(),
			)
			if err != nil {
				done <- err
				return
			}
			logrus.Info(run.GetConclusion())
			switch run.GetConclusion() {
			case "success":
				done <- nil
				return
			case "failure":
				done <- nil
				return
			default:
				continue
			}
		}

	}()
	err := <-done
	return err
}
