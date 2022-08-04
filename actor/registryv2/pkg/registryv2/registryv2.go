package registryv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/external"
)

type token struct {
	Val string `json:"token"`
}

type Opt struct {
	Image string
	Auth  *Auth
}
type Auth struct {
	Username string
	Password string
}
type Registry struct {
	c         *http.Client
	opt       Opt
	authToken string
}

func Init(c *http.Client, opt Opt) (*Registry, error) {
	if c == nil {
		c = &http.Client{}
	}
	return &Registry{
		opt: opt,
		c:   c,
	}, nil
}

func (r Registry) TagExists(tag string) (bool, error) {
	hostname, familiarName, err := external.ParseImageName(r.opt.Image)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse image")
	}
	res, err := r.get(fmt.Sprintf("https://%s/v2/%s/manifests/%s", hostname, familiarName, tag))
	printbody(res)

	return err == nil && res.StatusCode == http.StatusOK, err
}

func (r Registry) get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if isGhcr(url) {
		token, err := r.genTokenForGhcr()
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	} else {
		if r.opt.Auth != nil {
			req.SetBasicAuth(r.opt.Auth.Username, r.opt.Auth.Password)
		}
	}
	res, err := r.c.Do(req)
	return res, err
}

func isGhcr(url string) bool {
	return strings.Contains(url, "https://ghcr.io") || strings.Contains(url, "https://containers.")
}

func (r *Registry) genTokenForGhcr() (string, error) {
	if r.opt.Auth == nil {
		return "", fmt.Errorf("auth is empty")
	}
	if r.authToken != "" {
		return r.authToken, nil
	}
	hostname, _, err := external.ParseImageName(r.opt.Image)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image name")
	}
	callURL := fmt.Sprintf("%s/token", fmt.Sprintf("https://%s", hostname))
	req, err := http.NewRequest("GET", callURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(r.opt.Auth.Username, r.opt.Auth.Password)
	res, err := r.c.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed, err = %v, res = %v", err, res)
	}
	buf := bytes.Buffer{}
	buf.ReadFrom(res.Body)
	t := token{}
	if err := json.Unmarshal(buf.Bytes(), &t); err != nil {
		return "", err
	}
	r.authToken = t.Val
	return t.Val, nil
}

func printbody(res *http.Response) {
	buf := bytes.Buffer{}
	buf.ReadFrom(res.Body)
	logrus.Info(buf.String())
}
