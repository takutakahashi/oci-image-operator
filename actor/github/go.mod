module github.com/takutakahashi/oci-image-operator/actor/github

go 1.18

require (
	github.com/Netflix/go-env v0.0.0-20210215222557-e437a7e7f9fb
	github.com/google/go-cmp v0.5.7
	github.com/google/go-github/v43 v43.0.0
	github.com/migueleliasweb/go-github-mock v0.0.8
	github.com/spf13/cobra v1.4.0
	github.com/takutakahashi/oci-image-operator/actor/base v0.0.0-00010101000000-000000000000
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
)

replace (
	github.com/takutakahashi/oci-image-operator/actor/base => ../base
	github.com/takutakahashi/oci-image-operator => ../..
)

require (
	github.com/google/go-github/v41 v41.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
)
