package image

import (
	"errors"

	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func Ensure(image *buildv1beta1.Image, template *buildv1beta1.ImageFlowTemplate, secrets map[string]*corev1.Secret) (*buildv1beta1.Image, error) {
	return nil, errors.New("error")
}
