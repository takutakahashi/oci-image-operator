# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the go source
COPY . ./
WORKDIR /workspace/actor/registryv2
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go


# Build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ubuntu
WORKDIR /
COPY --from=builder /workspace/actor/registryv2/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
