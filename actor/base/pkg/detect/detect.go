package detect

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	imageutil "github.com/takutakahashi/oci-image-operator/pkg/image"
	"gopkg.in/fsnotify.v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Detect struct {
	c   client.Client
	ch  chan bool
	f   io.Reader
	opt DetectOpt
}

type DetectOpt struct {
	ImageName      string
	ImageNamespace string
	WatchPath      string
}

type DetectFile struct {
	Branches map[string]string `json:"branches"`
	Tags     map[string]string `json:"tags"`
}

const (
	MapKeyLatestTagHash = "latest/hash"
	MapKeyLatestTagName = "latest/name"
)

func Init(cfg *rest.Config, opt DetectOpt) (*Detect, error) {
	if cfg == nil {
		cfg = ctrl.GetConfigOrDie()
	}
	c, err := base.GenClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Detect{
		c:   c,
		opt: opt,
	}, nil
}

func (d *Detect) RunHTTP(ctx context.Context) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := d.UpdateImage(ctx); err != nil {
			logrus.Error(err)
		}
	})
	logrus.Info("listen http 8080...")
	return http.ListenAndServe("0.0.0.0:8080", nil)
}

func (d *Detect) Run(ctx context.Context) error {
	logrus.Infof("watching path: %s", d.opt.WatchPath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	done := make(chan bool)
	d.ch = done
	go func() {
		for {
			select {
			case e, ok := <-watcher.Events:
				if !ok {
					return
				}
				logrus.Info(e)
				// FIXME: WRITE op run and  kubernetes api is executed twice when file updated.
				if _, err := d.UpdateImage(ctx); err != nil {
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

	watcher.Add(d.opt.WatchPath)
	if err != nil {
		logrus.Fatal(err)
	}
	<-done
	return nil
}

func (d *Detect) Stop() {
	logrus.Info("Stopping worker")
	d.ch <- true
}

func (d *Detect) UpdateImage(ctx context.Context) (*buildv1beta1.Image, error) {
	var f io.Reader
	if d.f == nil {
		file, err := os.Open(d.opt.WatchPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		f = file
	} else {
		f = d.f
	}
	detectFile, err := parseJSON(f)
	if err != nil {
		return nil, err
	}
	logrus.Infof("detected struct: %v", detectFile)
	image := buildv1beta1.Image{}
	nn := ktypes.NamespacedName{
		Namespace: d.opt.ImageNamespace,
		Name:      d.opt.ImageName,
	}
	if err := d.c.Get(ctx, nn, &image); err != nil {
		return nil, err
	}

	newImage := image.DeepCopy()
	newConditions := ensureConditions(newImage.Status.Conditions, detectFile)
	diff := cmp.Diff(image.Status.Conditions, newConditions, cmpopts.IgnoreFields(buildv1beta1.ImageCondition{}, "LastTransitionTime"))
	logrus.Infof("diff: %s", diff)
	if diff != "" {
		newImage.Status.Conditions = newConditions
		if err := d.c.Status().Update(ctx, newImage); err != nil {
			return nil, err
		}
		logrus.Info("image updated")
	}
	logrus.Info("image ensured")
	return newImage, nil
}

func parseJSON(r io.Reader) (*DetectFile, error) {
	scanner := bufio.NewScanner(r)
	buf := []byte{}
	for scanner.Scan() {
		buf = append(buf, scanner.Bytes()...)
	}
	logrus.Info(string(buf))
	file := &DetectFile{}
	if err := json.Unmarshal(buf, file); err != nil {
		return nil, err
	}
	return file, nil
}

func ensureConditions(conditions []buildv1beta1.ImageCondition, detectFile *DetectFile) []buildv1beta1.ImageCondition {
	logrus.Info("=== before ===")
	logrus.Info(conditions)
	for branch, resolvedRevision := range detectFile.Branches {
		conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeDetected, &buildv1beta1.ImageConditionStatusTrue,
			buildv1beta1.ImageTagPolicyTypeBranchHash, branch, resolvedRevision)
		conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeChecked, &buildv1beta1.ImageConditionStatusFalse,
			buildv1beta1.ImageTagPolicyTypeBranchHash, branch, resolvedRevision)
	}
	for key, resolvedRevision := range detectFile.Tags {
		if key == MapKeyLatestTagName || key == MapKeyLatestTagHash {
			conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeDetected, &buildv1beta1.ImageConditionStatusTrue,
				buildv1beta1.ImageTagPolicyTypeTagHash, "latest", resolvedRevision)
			conditions = imageutil.UpdateCondition(conditions, buildv1beta1.ImageConditionTypeChecked, &buildv1beta1.ImageConditionStatusFalse,
				buildv1beta1.ImageTagPolicyTypeTagHash, "latest", resolvedRevision)
		}
	}
	for i, c := range conditions {
		switch c.TagPolicy {
		case buildv1beta1.ImageTagPolicyTypeTagHash:
			conditions[i].ResolvedRevision = detectFile.Tags[MapKeyLatestTagHash]
		case buildv1beta1.ImageTagPolicyTypeTagName:
			conditions[i].ResolvedRevision = detectFile.Tags[MapKeyLatestTagName]
		case buildv1beta1.ImageTagPolicyTypeBranchHash:
			conditions[i].ResolvedRevision = detectFile.Branches[c.Revision]
		case buildv1beta1.ImageTagPolicyTypeBranchName:
			conditions[i].ResolvedRevision = c.Revision
		default:
			conditions[i].ResolvedRevision = c.Revision
		}
	}
	logrus.Info("=== after ===")
	logrus.Info(conditions)
	return conditions
}
