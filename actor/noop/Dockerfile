FROM golang:1.26.5 AS builder
WORKDIR /app/
COPY . ./
WORKDIR /app/actor/noop
RUN go mod download
RUN go build -o /entrypoint main.go

FROM ubuntu

COPY --from=builder /entrypoint /entrypoint
CMD ["/entrypoint"]
