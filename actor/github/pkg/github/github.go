package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Netflix/go-env"
	"github.com/google/go-github/v43/github"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
)

type GithubOpt struct {
	BaseURL             string `env:"GITHUB_API_URL,default=https://api.github.com/"`
	Org                 string `env:"GITHUB_ORG,required=true"`
	Repo                string `env:"GITHUB_REPO,required=true"`
	Branches            string `env:"TARGET_BRANCHES"`
	Tags                string `env:"TARGET_TAGS"`
	PersonalAccessToken string `env:"GITHUB_TOKEN"`
	HTTPClient          *http.Client
}

type Github struct {
	c        *github.Client
	opt      *GithubOpt
	branches []string
	tags     []string
	revs     map[string]string
}

func Init(opt *GithubOpt) (*Github, error) {
	if opt.Org == "" {
		newOpt, err := GenOpt(opt.HTTPClient)
		if err != nil {
			return nil, err
		}
		opt = newOpt
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
	return &Github{c: c, opt: opt, branches: b, tags: t, revs: map[string]string{}}, nil
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
	tags, _, err := g.c.Repositories.ListTags(
		ctx, g.opt.Org, g.opt.Repo, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return map[string]string{}, nil
	}
	g.setTagHash(detect.MapKeyLatestTagHash, tags[0].GetCommit().GetSHA())
	g.setTagHash(detect.MapKeyLatestTagName, tags[0].GetName())
	for _, b := range g.tags {
		for _, tag := range tags {
			if tag.GetName() == b {
				g.setTagHash(b, tag.GetCommit().GetSHA())
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
