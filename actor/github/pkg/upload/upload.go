package upload

import (
	"context"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/github"
	"github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

type Upload struct {
	gh *github.Github
}

func Init() (*Upload, error) {
	opt, err := github.GenOpt(nil)
	if err != nil {
		return nil, err
	}
	gh, err := github.Init(opt)
	if err != nil {
		return nil, err
	}
	return &Upload{gh: gh}, nil
}

func (u Upload) Output(ctx context.Context, input *upload.Input) (upload.Output, error) {
	out := upload.Output{
		Builds: make([]upload.ImageBuild, len(input.Builds)),
	}
	// 1. trigger action
	// 2. wait for action result
	// 3. output if actions is succeeded
	resultCh := make(chan upload.ImageBuild)
	wg := &sync.WaitGroup{}
	for _, build := range input.Builds {
		wg.Add(1)
		go func(b upload.ImageBuild) {
			err := retry.Do(func() error {
				return u.gh.Dispatch(ctx, b.Tag, true)
			}, retry.Delay(1*time.Minute), retry.Attempts(3))
			if err != nil {
				b.Succeeded = v1beta1.ImageConditionStatusFailed
			} else {
				b.Succeeded = v1beta1.ImageConditionStatusTrue
			}
			resultCh <- b
			wg.Done()
		}(build)
		time.Sleep(5 * time.Second)
	}
	for i := range out.Builds {
		out.Builds[i] = <-resultCh
	}
	wg.Wait()
	return out, nil
}
