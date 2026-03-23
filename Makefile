.PHONY: all build test lint clean package stack-build stack-run stack builder builders manifest push-builders e2e e2e-build e2e-runtime e2e-models

BUILDPACK_ID := io.github.aagumin.mlflow-model
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
TIMEOUT ?= 600

# Registry configuration (override for ghcr.io)
REGISTRY ?= docker.io
IMAGE_PREFIX ?= aagumin
BUILD_IMAGE_TAG ?= 43
RUN_IMAGE_TAG ?= 43
BUILDER_IMAGE ?= $(REGISTRY)/$(IMAGE_PREFIX)/mlserver-builder:$(VERSION)

# Multi-platform configuration
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER_ARCHS := $(subst linux/,,$(PLATFORMS))

# Detect OS
UNAME_S := $(shell uname -s)

# Lima on macOS, native on Linux
ifeq ($(UNAME_S),Darwin)
	PACK := lima pack
	CONTAINER_TOOL ?= docker
	DOCKER ?= lima docker
else
	PACK := pack
	CONTAINER_TOOL ?= docker
	DOCKER ?= docker
endif

all: build

build:
	@echo "Building buildpack binaries for amd64 and arm64..."
	mkdir -p buildpack/bin/linux-amd64
	cd buildpack && GOSUMDB=sum.golang.org CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/detect ./cmd/detect
	cd buildpack && GOSUMDB=sum.golang.org CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/build ./cmd/build
	mkdir -p buildpack/bin/linux-arm64
	cd buildpack && GOSUMDB=sum.golang.org CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/detect ./cmd/detect
	cd buildpack && GOSUMDB=sum.golang.org CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/build ./cmd/build
	chmod +x buildpack/bin/linux-amd64/detect buildpack/bin/linux-amd64/build
	chmod +x buildpack/bin/linux-arm64/detect buildpack/bin/linux-arm64/build

test:
	cd buildpack && GOSUMDB=sum.golang.org GOCACHE=/tmp/aipack-go-cache go test -v ./...
	cd buildpack && GOSUMDB=sum.golang.org GOCACHE=/tmp/aipack-go-cache go test -v -race ./internal/...

lint:
	cd buildpack && GOSUMDB=sum.golang.org golangci-lint run ./...

package: build
	$(PACK) buildpack package ${BUILDPACK_ID} \
		--config buildpack/package.toml \
		--tag ${BUILDPACK_ID}:${VERSION} \
		--verbose 

# ============================================================
# Stack images (base images for build and run)
# ============================================================

stack-build:
	$(DOCKER) buildx build --push \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-build:$(BUILD_IMAGE_TAG) \
		stack/build

stack-run:
	$(DOCKER) buildx build --push \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-run:$(RUN_IMAGE_TAG) \
		stack/run

stack: stack-build stack-run

# ============================================================
# Builder (modern approach without deprecated stack)
# ============================================================

# Generate builder config from template
builder.generated.toml: builder.toml.template
	sed -e 's|{{REGISTRY}}|$(REGISTRY)|g' \
	    -e 's|{{IMAGE_PREFIX}}|$(IMAGE_PREFIX)|g' \
	    -e 's|{{BUILD_IMAGE_TAG}}|$(BUILD_IMAGE_TAG)|g' \
	    -e 's|{{RUN_IMAGE_TAG}}|$(RUN_IMAGE_TAG)|g' \
	    $< > $@

# Create builder for specific architecture
builder-%: builder.generated.toml
	$(PACK) builder create $(BUILDER_IMAGE)-$* \
		--config $< \
		--pull-policy if-not-present \
		--verbose

# Create builders for all architectures
builders: $(foreach arch,$(BUILDER_ARCHS),builder-$(arch))

# Create manifest list from architecture-specific builders
manifest: builders
	$(DOCKER) manifest create $(BUILDER_IMAGE) \
		$(foreach arch,$(BUILDER_ARCHS),$(BUILDER_IMAGE)-$(arch))

# Push all architecture-specific builders
push-builders: manifest
	@for arch in $(BUILDER_ARCHS); do \
		$(DOCKER) push $(BUILDER_IMAGE)-$$arch; \
	done
	$(DOCKER) manifest push $(BUILDER_IMAGE)

# Full builder cycle: stack + package + builders + manifest
builder: stack package manifest

# ============================================================
# Development targets
# ============================================================

dev-train:
	python dev/train.py

e2e-models:
	./.venv/bin/python e2e/scripts/generate_models.py

e2e-build:
	./e2e/scripts/verify-build.sh pyfunc
	./e2e/scripts/verify-build.sh sklearn

e2e-runtime:
	./e2e/scripts/verify-runtime.sh pyfunc
	./e2e/scripts/verify-runtime.sh sklearn

e2e:
	./e2e/scripts/run-all.sh

clean:
	rm -rf buildpack/bin/linux-amd64
	rm -rf buildpack/bin/linux-arm64
	rm -rf out/
	rm -f builder.generated.toml
