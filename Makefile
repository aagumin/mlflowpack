.PHONY: all build test lint clean package stack-build stack-run stack builder test-integration create-test-model test-build
.PHONY: kpack-setup kpack-deploy kpack-test-build kpack-test-inference kpack-cleanup kpack-all

BUILDPACK_ID := io.amazme.buildpacks.mlflow-model
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
CONTAINER_TOOL ?= docker
KIND_CLUSTER ?= kind
TIMEOUT ?= 600

# Lima-wrapped pack command
PACK := lima pack

all: build

build:
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/detect ./cmd/detect
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/build ./cmd/build

test:
	cd buildpack && go test -v -race ./...

lint:
	cd buildpack && golangci-lint run ./...

package: build
	$(PACK) buildpack package ${BUILDPACK_ID} \
		--config buildpack/package.toml \
		--tag ${BUILDPACK_ID}:${VERSION}

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
		--pull-policy never

# ============================================================
# Integration tests (local with pack)
# ============================================================

test-integration:
	./tests/integration/test_build.sh

create-test-model:
	cd tests/integration/test-model && python3 create_model.py

test-build: create-test-model
	$(PACK) build test-mlflow-model \
		--builder amazme/mlserver-builder:${VERSION} \
		--path tests/integration/test-model \
		--pull-policy never

# ============================================================
# kpack tests (kind cluster)
# ============================================================

kpack-setup:
	./tests/integration/test_kpack.sh setup

kpack-deploy:
	./tests/integration/test_kpack.sh deploy

kpack-test-build:
	./tests/integration/test_kpack.sh test

kpack-test-inference:
	./tests/integration/test_kpack.sh inference

kpack-cleanup:
	./tests/integration/test_kpack.sh cleanup

kpack-all: kpack-setup kpack-deploy kpack-test-build

# ============================================================
# Development targets
# ============================================================

dev-train:
	python dev/train.py

dev-mlflow:
	mlflow server --host 0.0.0.0 --port 5000

clean:
	rm -rf buildpack/bin/
	rm -rf out/
