.PHONY: all build test lint clean package stack-build stack-run stack builder

BUILDPACK_ID := io.amazme.buildpacks.mlflow-model
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
TIMEOUT ?= 600

# Detect OS
UNAME_S := $(shell uname -s)

# Lima on macOS, native on Linux
ifeq ($(UNAME_S),Darwin)
	PACK := lima pack
	CONTAINER_TOOL ?= docker
else
	PACK := pack
	CONTAINER_TOOL ?= docker
endif

all: build

build:
	mkdir -p buildpack/bin/linux-amd64 buildpack/bin/linux-arm64
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/detect ./cmd/detect
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/build ./cmd/build
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/detect ./cmd/detect
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/build ./cmd/build
	chmod +x buildpack/bin/detect buildpack/bin/build

test:
	cd buildpack && GOCACHE=/tmp/aipack-go-cache go test -v ./...
	cd buildpack && GOCACHE=/tmp/aipack-go-cache go test -v -race ./internal/...

lint:
	cd buildpack && golangci-lint run ./...

package: build
	$(PACK) buildpack package ${BUILDPACK_ID} \
		--config buildpack/package.toml \
		--tag ${BUILDPACK_ID}:${VERSION} \
		--force-color \
		--verbose 

# ============================================================
# Stack images (base images for build and run)
# ============================================================

stack-build:
	$(CONTAINER_TOOL) build -t amazme/fedora-mlserver-build:43 stack/build

stack-run:
	$(CONTAINER_TOOL) build -t amazme/fedora-mlserver-run:43 stack/run

stack: stack-build stack-run

# ============================================================
# Builder (modern approach without deprecated stack)
# ============================================================

builder: stack package
	$(PACK) builder create amazme/mlserver-builder:${VERSION} \
		--config builder.toml \
		--pull-policy never \
		--verbose

# ============================================================
# Development targets
# ============================================================

dev-train:
	python dev/train.py


clean:
	rm -rf buildpack/bin/linux-amd64
	rm -rf buildpack/bin/linux-arm64
	rm -rf out/
