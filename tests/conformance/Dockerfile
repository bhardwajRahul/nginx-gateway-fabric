# syntax=docker/dockerfile:1.17
FROM golang:1.24

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance

COPY ../. /go/src/github.com/nginx/nginx-gateway-fabric/
COPY --link go.mod /go/src/github.com/nginx/nginx-gateway-fabric/tests/
COPY --link go.sum /go/src/github.com/nginx/nginx-gateway-fabric/tests/
RUN go mod download

COPY --link conformance /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance
