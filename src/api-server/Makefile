.PHONY: default check test build image

# Default build target
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
DOCKER_BUILD_PLATFORMS ?= linux/amd64,linux/arm64

default: check test build

dist:
	mkdir dist

build: dist
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -trimpath -ldflags '-s' -o "./dist/${GOOS}/${GOARCH}/api-server" .

test:
	go test -v -cover ./...

lint:
	golangci-lint run

build-linux-arm64: export GOOS := linux
build-linux-arm64: export GOARCH := arm64
build-linux-arm64:
	make build

build-linux-amd64: export GOOS := linux
build-linux-amd64: export GOARCH := amd64
build-linux-amd64:
	make build

## Build Multi archs Docker image
multi-arch-image-%: build-linux-amd64 build-linux-arm64
	docker buildx build $(DOCKER_BUILDX_ARGS) -t ghcr.io/traefik/api-server:$* --platform=$(DOCKER_BUILD_PLATFORMS) .
