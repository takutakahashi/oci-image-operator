package detect

import (
	"log"

	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"gopkg.in/fsnotify.v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Detect struct {
	c         client.Client
	watchPath string
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
				if err := d.UpdateImage(); err != nil {
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

func (d *Detect) UpdateImage() error {
	return nil
}

func genClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(buildv1beta1.AddToScheme(scheme))
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}
