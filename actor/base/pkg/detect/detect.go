package detect

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/types"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"gopkg.in/fsnotify.v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Detect struct {
	c         client.Client
	watchPath string
	f         io.Reader
}

func Init(cfg *rest.Config, watchPath string) (*Detect, error) {
	if cfg == nil {
		cfg = ctrl.GetConfigOrDie()
	}
	c, err := genClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Detect{
		c:         c,
		watchPath: watchPath,
	}, nil
}

func (d *Detect) Run() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	done := make(chan bool)
	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				}
				if _, err := d.UpdateImage(); err != nil {
					logrus.Error(err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logrus.Error(err)
			}
		}
	}()

	watcher.Add(d.watchPath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	return nil
}

func (d *Detect) UpdateImage() (*buildv1beta1.Image, error) {
	ctx := context.TODO()
	if d.f == nil {
		f, err := os.Open(d.watchPath)
		if err != nil {
			return nil, err
		}
		d.f = f
	}
	detectFile, err := parseJSON(d.f)
	if err != nil {
		return nil, err
	}

	image := buildv1beta1.Image{}
	nn := ktypes.NamespacedName{
		Namespace: os.Getenv("IMAGE_NAMESPACE"),
		Name:      os.Getenv("IMAGE_NAME"),
	}
	if err := d.c.Get(ctx, nn, &image); err != nil {
		return nil, err
	}
	newImage := image.DeepCopy()
	newPolicy := []buildv1beta1.ImageTagPolicy{}
	for _, policy := range image.Spec.Repository.TagPolicies {
		switch policy.Policy {
		case buildv1beta1.ImageTagPolicyTypeTagHash:
			policy.ResolvedRevision = detectFile.Tags[types.MapKeyLatestTagHash]
		case buildv1beta1.ImageTagPolicyTypeTagName:
			policy.ResolvedRevision = detectFile.Tags[types.MapKeyLatestTagName]
		case buildv1beta1.ImageTagPolicyTypeBranchHash:
			policy.ResolvedRevision = detectFile.Branches[policy.Revision]
		case buildv1beta1.ImageTagPolicyTypeBranchName:
			policy.ResolvedRevision = policy.Revision
		default:
			policy.ResolvedRevision = policy.Revision
		}
		newPolicy = append(newPolicy, policy)
	}
	newImage.Spec.Repository.TagPolicies = newPolicy
	return newImage, nil
}

func genClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(buildv1beta1.AddToScheme(scheme))
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

func parseJSON(r io.Reader) (*types.DetectFile, error) {
	buf := []byte{}
	if _, err := r.Read(buf); err != nil {
		return nil, err
	}
	file := &types.DetectFile{}
	if err := json.Unmarshal(buf, file); err != nil {
		return nil, err
	}
	return file, nil
}
