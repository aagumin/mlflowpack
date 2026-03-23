.PHONY: all build test lint clean package stack-build stack-run stack builder builders manifest push-builders e2e e2e-build e2e-runtime e2e-models setup-env registry-start registry-stop

BUILDPACK_ID := io.github.aagumin.mlflow-model
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
TIMEOUT ?= 600

# ============================================================
# Environment detection
# ============================================================

UNAME_S := $(shell uname -s)

# Detect Lima on macOS
ifeq ($(UNAME_S),Darwin)
    LIMA_CHECK := $(shell limactl --version 2>/dev/null)
    ifneq ($(LIMA_CHECK),)
        # Lima is available on macOS
        PACK ?= lima pack
        DOCKER ?= lima docker
        LIMA_HOST := $(shell lima uname -n 2>/dev/null || echo "lima")
    else
        PACK ?= pack
        DOCKER ?= docker
    endif
else
    # Linux
    PACK ?= pack
    DOCKER ?= docker
endif

# ============================================================
# Registry configuration
# ============================================================

# Local development: localhost:5000 (zot registry)
# CI/CD: ghcr.io
REGISTRY ?= localhost:5000
IMAGE_PREFIX ?= aagumin
BUILD_IMAGE_TAG ?= 43
RUN_IMAGE_TAG ?= 43
BUILDER_IMAGE ?= $(REGISTRY)/$(IMAGE_PREFIX)/mlserver-builder:$(VERSION)

# Multi-platform configuration
PLATFORMS ?= linux/amd64,linux/arm64
comma := ,
space := $(null) $(null)
PLATFORM_LIST := $(subst $(comma),$(space),$(PLATFORMS))

# For multi-arch builds, we MUST push to registry (--publish required by pack)
# Local: uses localhost:5000 (zot)
# CI: uses ghcr.io
ifeq ($(REGISTRY),localhost:5000)
    # Local build - push to local zot registry
    PUSH ?= --push
    PUBLISH ?= --publish
else
    # CI/CD - explicit control
    PUSH ?=
    PUBLISH ?=
endif

# ============================================================
# Setup environment
# ============================================================

ZOT_VERSION := v2.1.1
ZOT_CONTAINER := zot-registry

setup-env:
	@echo "=== Setting up local development environment ==="
	@echo ""
	@echo "Detected environment:"
	@echo "  OS: $(UNAME_S)"
ifeq ($(UNAME_S),Darwin)
    ifneq ($(LIMA_CHECK),)
	@echo "  Lima: yes ($(LIMA_HOST))"
    else
	@echo "  Lima: no"
    endif
endif
	@echo "  Pack: $(PACK)"
	@echo "  Docker: $(DOCKER)"
	@echo "  Registry: $(REGISTRY)"
	@echo ""
	@$(MAKE) registry-start

registry-start:
	@echo "Starting zot registry on $(REGISTRY)..."
	@$(DOCKER) run -d --rm \
		--name $(ZOT_CONTAINER) \
		-p 5000:5000 \
		ghcr.io/project-zot/zot:$(ZOT_VERSION) \
		serve /etc/zot/config.json 2>/dev/null || echo "Registry already running or starting..."
	@sleep 2
	@echo "Checking registry health..."
	@curl -s http://localhost:5000/v2/ || echo "Warning: registry not responding"

registry-stop:
	@echo "Stopping zot registry..."
	@$(DOCKER) stop $(ZOT_CONTAINER) 2>/dev/null || echo "Registry not running"

registry-status:
	@curl -s http://localhost:5000/v2/ && echo "Registry is running" || echo "Registry is not running"

# ============================================================
# Build
# ============================================================

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

# Multi-arch: pushes to registry (required for multi-platform)
# Single-arch local: use stack-local
stack-build:
	$(DOCKER) buildx build $(PUSH) \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-build:$(BUILD_IMAGE_TAG) \
		stack/build

stack-run:
	$(DOCKER) buildx build $(PUSH) \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-run:$(RUN_IMAGE_TAG) \
		stack/run

# Single-platform local build (faster, loads to docker daemon)
stack-local: PLATFORMS = linux/$(shell uname -m)
stack-local: PUSH = --load
stack-local: stack-build stack-run

stack: stack-build stack-run

# ============================================================
# Builder
# ============================================================

# Generate builder config from template
builder.generated.toml: builder.toml.template
	sed -e 's|{{REGISTRY}}|$(REGISTRY)|g' \
	    -e 's|{{IMAGE_PREFIX}}|$(IMAGE_PREFIX)|g' \
	    -e 's|{{BUILD_IMAGE_TAG}}|$(BUILD_IMAGE_TAG)|g' \
	    -e 's|{{RUN_IMAGE_TAG}}|$(RUN_IMAGE_TAG)|g' \
	    $< > $@

# Local: single-platform builder (loads to docker daemon, no registry needed)
# Use this for local development
builder: stack-local package builder.generated.toml
	$(PACK) builder create $(BUILDER_IMAGE) \
		--config builder.generated.toml \
		--pull-policy if-not-present \
		--verbose

# Multi-arch builder with registry push (CI only - requires proper registry like ghcr.io)
# Does NOT work with localhost:5000/zot due to manifest list limitations
builder-multi: PLATFORMS = linux/amd64,linux/arm64
builder-multi: PUSH = --push
builder-multi: stack package builder.generated.toml
	$(PACK) builder create $(BUILDER_IMAGE) \
		--config builder.generated.toml \
		$(foreach p,$(PLATFORM_LIST),--target $(p)) \
		--publish \
		--pull-policy if-not-present \
		--verbose

# Alias for CI compatibility
push-builders: builder-multi

# CI target - multi-arch build for ghcr.io
ci: REGISTRY = ghcr.io
ci: builder-multi

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
