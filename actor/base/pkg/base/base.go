package base

import (
	"context"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(buildv1beta1.AddToScheme(scheme))
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

func GetImage(ctx context.Context, c client.Client, name, namespace string) (*buildv1beta1.Image, error) {
	i := &buildv1beta1.Image{}
	nn := types.NamespacedName{Name: name, Namespace: namespace}
	if err := c.Get(ctx, nn, i); err != nil {
		return nil, err
	}
	return i, nil
}

func GetImageFlowTemplate(ctx context.Context, c client.Client, name, namespace string) (*buildv1beta1.ImageFlowTemplate, error) {
	i := &buildv1beta1.ImageFlowTemplate{}
	nn := types.NamespacedName{Name: name, Namespace: namespace}
	if err := c.Get(ctx, nn, i); err != nil {
		return nil, err
	}
	return i, nil
}
