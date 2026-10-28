FROM golang:1.27.3 AS builder
WORKDIR /app/
COPY . ./
WORKDIR /app/actor/noop
RUN go mod download
RUN go build -o /entrypoint main.go

FROM ubuntu:24.04
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /entrypoint /entrypoint
CMD ["/entrypoint"]
