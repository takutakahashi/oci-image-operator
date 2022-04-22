FROM gitpod/workspace-full
RUN go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest \
 && curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH) \
 && chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
