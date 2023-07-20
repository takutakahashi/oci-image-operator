package upload

import (
	"context"
	"io"
	"os"

	"github.com/k0kubun/pp"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	imageutil "github.com/takutakahashi/oci-image-operator/pkg/image"
	"gopkg.in/fsnotify.v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Input struct {
	Builds []ImageBuild `json:"builds"`
}

type Output struct {
	Builds []ImageBuild `json:"builds"`
}

type ImageBuild struct {
	Target    string                            `json:"target"`
	Tag       string                            `json:"tag"`
	Succeeded buildv1beta1.ImageConditionStatus `json:"succeeded,omitempty"`
}

type Opt struct {
	ImageName      string
	ImageNamespace string
	ImageTarget    string
	WatchPath      string
}

type Upload struct {
	c     client.Client
	ch    chan bool
	in    io.Writer
	out   io.Reader
	opt   Opt
	image *buildv1beta1.Image
}

func Init(cfg *rest.Config, opt Opt) (*Upload, error) {
	if cfg == nil {
		cfg = ctrl.GetConfigOrDie()
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Upload{
		c:   c,
		ch:  make(chan bool),
		opt: opt,
	}, nil
}

func (u *Upload) GetImage(ctx context.Context) (*buildv1beta1.Image, error) {
	if u.image != nil {
		return u.image, nil
	}
	image, err := base.GetImage(ctx, u.c, u.opt.ImageName, u.opt.ImageNamespace)
	if err != nil {
		return nil, err
	}
	u.image = image
	return image, nil
}

func (c *Upload) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := c.Execute(ctx); err != nil {
		logrus.Error("error from execute")
		logrus.Error(err)
	}
	done := make(chan bool)
	c.ch = done
	go func() {
		for {
			select {
			case e, ok := <-watcher.Events:
				if !ok {
					logrus.Info("failed to get event")
					return
				}
				logrus.Info("====== detected file changes =======")
				logrus.Info(e)
				if e.Op == fsnotify.Write {
					if err := c.Execute(ctx); err != nil {
						logrus.Error("error from execute")
						logrus.Error(err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logrus.Error("error from watcher")
				logrus.Error(err)
			}
		}
	}()

	watcher.Add(c.opt.WatchPath)
	if err != nil {
		logrus.Error(err)
	}
	<-done
	return nil
}

func (u *Upload) GetInput(ctx context.Context) (*Input, error) {
	image, err := u.GetImage(ctx)
	if err != nil {
		return nil, err
	}
	out := getInput(u.opt.ImageTarget, image.Status.Conditions)
	return &out, nil
}

func (u *Upload) Execute(ctx context.Context) error {

	image, err := u.GetImage(ctx)
	if err != nil {
		return err
	}
	input, err := u.GetInput(ctx)
	if err != nil {
		return err
	}
	if base.ActorOutputExists() {
		output := &Output{}
		if err := u.Import(output); err != nil {
			return err
		}
		if err := u.UpdateImage(ctx, image, output); err != nil {
			return err
		}
		u.Stop()
		return nil

	}
	if !base.ActorInputExists() {
		if err := u.Export(input); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func getInput(target string, conditions []buildv1beta1.ImageCondition) Input {
	builds := []ImageBuild{}
	for _, cond := range conditions {
		if cond.Type == buildv1beta1.ImageConditionTypeUploaded && cond.Status != buildv1beta1.ImageConditionStatusTrue && cond.Status != buildv1beta1.ImageConditionStatusCanceled {
			builds = append(builds, ImageBuild{Tag: cond.ResolvedRevision, Target: target})
		}
	}
	logrus.Info("==== input ====")
	pp.Println(builds)
	return Input{Builds: builds}
}

func (u *Upload) Export(input *Input) error {
	if u.in == nil {
		w, err := os.Create(base.InWorkDir("input"))
		if err != nil {
			return err
		}
		u.in = w
	}
	return base.ParseJSON(input, u.in)
}

func (u *Upload) Import(output *Output) error {
	if u.out == nil {
		r, err := os.Open(base.InWorkDir("output"))
		if err != nil {
			return err
		}
		u.out = r
	}
	return base.MarshalJSON(output, u.out)

}

func (u *Upload) UpdateImage(ctx context.Context, image *buildv1beta1.Image, output *Output) error {
	logrus.Info("==== output ====")
	pp.Println(output)
	for _, build := range output.Builds {
		exist := false
		for _, c := range imageutil.GetCondition(image.Status.Conditions, buildv1beta1.ImageConditionTypeUploaded) {
			if c.ResolvedRevision == build.Tag {
				image.Status.Conditions = imageutil.UpdateUploadedCondition(
					image.Status.Conditions,
					build.Succeeded,
					c.Revision,
					build.Tag,
				)
				exist = true
			}
		}
		if !exist {
			image.Status.Conditions = imageutil.UpdateUploadedCondition(
				image.Status.Conditions,
				build.Succeeded,
				"",
				build.Tag,
			)
		}
	}
	return u.c.Status().Update(ctx, image, &client.UpdateOptions{})
}

func (u *Upload) Stop() {
	if u.ch != nil {
		u.ch <- true
	}
}
