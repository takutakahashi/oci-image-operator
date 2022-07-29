module github.com/takutakahashi/oci-image-operator/actor/registryv2

go 1.18

require github.com/spf13/cobra v1.4.0

replace (
	github.com/takutakahashi/oci-image-operator => ../..
	github.com/takutakahashi/oci-image-operator/actor/base => ../base
)

require (
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
