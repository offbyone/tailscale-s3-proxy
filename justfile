build: build-arm64 build-amd64

build-arm64:
    GOOS=linux GOARCH=arm64 go build -o bin/tailscale-s3-proxy-arm64 tailscale-s3-proxy

build-amd64:
    GOOS=linux GOARCH=amd64 go build -o bin/tailscale-s3-proxy-amd64 tailscale-s3-proxy

deps:
    go get -u ./... && go mod tidy && go mod vendor
