package check

import (
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

type CheckFile struct {
	PolicyRevisions []PolicyRevision
}

type PolicyRevision struct {
	Policy           buildv1beta1.ImageTagPolicyType
	Revision         string
	ResolvedRevision string
}
