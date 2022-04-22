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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/google/go-cmp/cmp"
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
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=imageflowtemplates,verbs=get;list
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=images/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=build.takutakahashi.dev,resources=images/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Image object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	image, imt, secrets, err := r.gatherResources(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	after, err := imageutil.Ensure(image, imt, secrets)
	if err != nil {
		return ctrl.Result{}, err
	}
	if diff := cmp.Diff(image, after); diff != "" {
		if err := r.Update(ctx, after, &client.UpdateOptions{}); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1beta1.Image{}).
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
	}
	if err := r.Get(ctx, types.NamespacedName{Name: imtName, Namespace: image.Namespace}, imt); err != nil {
		return nil, nil, nil, err
	}
	secrets := map[string]*corev1.Secret{}
	s := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: image.Spec.Repository.Auth.SecretName, Namespace: image.Namespace}, s); err != nil {
		return nil, nil, nil, err
	}
	secrets[fmt.Sprintf("repository/%s", image.Spec.Repository.Auth.SecretName)] = s
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
