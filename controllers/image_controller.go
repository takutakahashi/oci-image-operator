/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sirupsen/logrus"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	imageutil "github.com/takutakahashi/oci-image-operator/pkg/image"
)

// ImageReconciler reconciles a Image object
type ImageReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=images,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=imageflowtemplates,verbs=get;list;watch
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=images/status,verbs=list;get;create;update;patch;watch
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=images/finalizers,verbs=update;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;get;create;update;patch;delete;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=list;get;create;update;patch;delete;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=list;get;watch

const IMAGE_FINALIZERS string = "build.takutakahashi.dev/image"

func (r *ImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	image, imt, secrets, err := r.gatherResources(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to gather required resources")
		return ctrl.Result{Requeue: true}, nil
	}
	logrus.Info(image.GetFinalizers())
	if image.GetFinalizers() == nil {
		updated := image.DeepCopy()
		updated.SetFinalizers([]string{IMAGE_FINALIZERS})
		return ctrl.Result{}, r.Update(ctx, updated, &client.UpdateOptions{})
	}
	after, err := imageutil.Ensure(ctx, r.Client, image.DeepCopy(), imt, secrets)
	if err != nil {
		logger.Error(err, "failed to ensure image")
		return ctrl.Result{Requeue: true}, nil
	}
	diff := imageutil.Diff(image, after)
	if diff != "" {
		logrus.Infof("diff: %s", diff)
		if err := r.Status().Update(ctx, after, &client.UpdateOptions{}); err != nil {
			return ctrl.Result{}, err
		}
	}
	logrus.Info("no diff detected")
	logrus.Info("reconcilation finished")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1beta1.Image{}).
		Owns(&appsv1.Deployment{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ImageReconciler) gatherResources(ctx context.Context, req ctrl.Request) (*buildv1beta1.Image, *buildv1beta1.ImageFlowTemplate, map[string]*corev1.Secret, error) {

	image := &buildv1beta1.Image{}
	if err := r.Get(ctx, req.NamespacedName, image); err != nil {
		return nil, nil, nil, err
	}
	imt := &buildv1beta1.ImageFlowTemplate{}
	imtName := image.Spec.TemplateName
	if imtName == "" {
		imtName = image.Annotations[buildv1beta1.AnnotationImageFlowTemplateDefaultAll]
		r.Recorder.Eventf(image, corev1.EventTypeNormal, "UseDefaultTemplate", "use default template: %s", imtName)
	}
	if err := r.Get(ctx, types.NamespacedName{Name: imtName, Namespace: image.Namespace}, imt); err != nil {
		r.Recorder.Event(image, corev1.EventTypeWarning, "TemplateNotFound", err.Error())
		return nil, nil, nil, err
	}
	secrets := map[string]*corev1.Secret{}
	if image.Spec.Repository.Auth.SecretName != "" {
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: image.Spec.Repository.Auth.SecretName, Namespace: image.Namespace}, s); err == nil {
			secrets[fmt.Sprintf("repository/%s", image.Spec.Repository.Auth.SecretName)] = s
		}
	}
	for _, target := range image.Spec.Targets {
		if target.Auth.SecretName == "" {
			continue
		}
		s := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: target.Auth.SecretName, Namespace: image.Namespace}, s); err != nil {
			return nil, nil, nil, err
		}
		secrets[fmt.Sprintf("targets/%s", target.Auth.SecretName)] = s
	}
	return image, imt, secrets, nil
}
