# Multi-Platform Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add amd64 + arm64 support for buildpack with transparent platform selection via Docker manifest lists.

**Architecture:** Go cross-compilation for binaries, docker buildx for multi-arch stack images, separate builders per architecture merged via manifest list. Makefile as single source of truth.

**Tech Stack:** Go, Docker buildx, Pack CLI, GitHub Actions

---

## Task 1: Update buildpack/package.toml for arm64

**Files:**
- Modify: `buildpack/package.toml`

**Step 1: Add arm64 target**

Edit `buildpack/package.toml`:

```toml
[buildpack]
uri = "."

[[targets]]
os = "linux"
arch = "amd64"

[[targets]]
os = "linux"
arch = "arm64"
```

**Step 2: Verify buildpack.toml already has both targets**

Run: `cat buildpack/buildpack.toml | grep -A2 "\[\[targets\]\]"`
Expected: Shows both amd64 and arm64 targets already defined

**Step 3: Commit**

```bash
git add buildpack/package.toml
git commit -m "feat(buildpack): add arm64 target to package config"
```

---

## Task 2: Update Makefile for multi-arch Go build

**Files:**
- Modify: `Makefile:28-32`

**Step 1: Replace single-arch build with multi-arch**

Replace the `build:` target (lines 28-32):

```makefile
build:
	@echo "Building buildpack binaries for amd64 and arm64..."
	mkdir -p buildpack/bin/linux-amd64
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/detect ./cmd/detect
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux-amd64/build ./cmd/build
	mkdir -p buildpack/bin/linux-arm64
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/detect ./cmd/detect
	cd buildpack && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux-arm64/build ./cmd/build
	chmod +x buildpack/bin/linux-amd64/detect buildpack/bin/linux-amd64/build
	chmod +x buildpack/bin/linux-arm64/detect buildpack/bin/linux-arm64/build
```

**Step 2: Update clean target**

Replace the `clean:` target (lines 98-101):

```makefile
clean:
	rm -rf buildpack/bin/linux-amd64
	rm -rf buildpack/bin/linux-arm64
	rm -rf out/
	rm -f builder.generated.toml
```

**Step 3: Test local build**

Run: `make build`
Expected: Creates `buildpack/bin/linux-amd64/` and `buildpack/bin/linux-arm64/` with detect and build binaries

**Step 4: Commit**

```bash
git add Makefile
git commit -m "feat(make): add multi-arch Go build for amd64 and arm64"
```

---

## Task 3: Add multi-arch variables to Makefile

**Files:**
- Modify: `Makefile` (add after line 12)

**Step 1: Add PLATFORMS and BUILDER_ARCHS variables**

Add after line 12 (after `BUILDER_IMAGE`):

```makefile
# Multi-platform configuration
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER_ARCHS := $(subst linux/,,$(PLATFORMS))
```

**Step 2: Verify variable expansion**

Run: `make -n build | head -1`
Expected: No errors, variables expand correctly

**Step 3: Commit**

```bash
git add Makefile
git commit -m "feat(make): add PLATFORMS variable for multi-arch builds"
```

---

## Task 4: Update stack-build and stack-run for multi-arch

**Files:**
- Modify: `Makefile:51-56`

**Step 1: Update stack-build target**

Replace `stack-build:` (lines 51-52):

```makefile
stack-build:
	docker buildx build --push \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-build:$(BUILD_IMAGE_TAG) \
		stack/build
```

**Step 2: Update stack-run target**

Replace `stack-run:` (lines 54-55):

```makefile
stack-run:
	docker buildx build --push \
		--platform $(PLATFORMS) \
		-t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-run:$(RUN_IMAGE_TAG) \
		stack/run
```

**Step 3: Update stack target dependency**

Replace `stack:` (line 57):

```makefile
stack: stack-build stack-run
```

**Step 4: Commit**

```bash
git add Makefile
git commit -m "feat(make): add multi-arch support for stack images via buildx"
```

---

## Task 5: Add builder-per-architecture targets

**Files:**
- Modify: `Makefile` (add after builder.generated.toml target, around line 70)

**Step 1: Add builder-% pattern target**

Add after the `builder.generated.toml:` target:

```makefile
# Create builder for specific architecture
builder-%: builder.generated.toml
	$(PACK) builder create $(BUILDER_IMAGE)-$* \
		--config $< \
		--pull-policy never \
		--verbose
```

**Step 2: Add builders target (all architectures)**

```makefile
# Create builders for all architectures
builders: $(foreach arch,$(BUILDER_ARCHS),builder-$(arch))
```

**Step 3: Add manifest target**

```makefile
# Create manifest list from architecture-specific builders
manifest: builders
	docker manifest create $(BUILDER_IMAGE) \
		$(foreach arch,$(BUILDER_ARCHS),$(BUILDER_IMAGE)-$(arch))
```

**Step 4: Add push-builders target**

```makefile
# Push all architecture-specific builders
push-builders: manifest
	@for arch in $(BUILDER_ARCHS); do \
		docker push $(BUILDER_IMAGE)-$$arch; \
	done
	docker manifest push $(BUILDER_IMAGE)
```

**Step 5: Update main builder target**

Replace the existing `builder:` target (lines 71-75):

```makefile
# Full builder cycle: stack + package + builders + manifest
builder: stack package manifest
```

**Step 6: Commit**

```bash
git add Makefile
git commit -m "feat(make): add per-architecture builder targets and manifest creation"
```

---

## Task 6: Simplify GitHub Actions workflow

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Update stack-images job**

Replace the `build-image` and `run-image` jobs with a combined `stack-images` job:

```yaml
  stack-images:
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.version.outputs.VERSION }}
      is_stable: ${{ steps.stable.outputs.IS_STABLE }}
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Get version from tag
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT

      - name: Check if stable release
        id: stable
        run: |
          if [[ ! "${{ steps.version.outputs.VERSION }}" =~ -(beta|rc|alpha|pre) ]]; then
            echo "IS_STABLE=true" >> $GITHUB_OUTPUT
          fi

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push stack images
        run: |
          make stack \
            REGISTRY=${{ env.REGISTRY }} \
            IMAGE_PREFIX=${{ github.repository_owner }} \
            BUILD_IMAGE_TAG=${{ steps.version.outputs.VERSION }} \
            RUN_IMAGE_TAG=${{ steps.version.outputs.VERSION }} \
            PLATFORMS=linux/amd64,linux/arm64

      - name: Tag latest for stable release
        if: steps.stable.outputs.IS_STABLE == 'true'
        run: |
          docker buildx imagetools create -t ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:latest ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:${{ steps.version.outputs.VERSION }}
          docker buildx imagetools create -t ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:latest ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:${{ steps.version.outputs.VERSION }}
```

**Step 2: Update builder-image job**

Replace the `builder-image` job:

```yaml
  builder-image:
    needs: stack-images
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Setup Pack CLI
        uses: buildpacks/github-actions/setup-pack@v5.1.0
        with:
          pack-version: 0.40.1

      - name: Create and push builder with manifest
        run: |
          make push-builders \
            REGISTRY=${{ env.REGISTRY }} \
            IMAGE_PREFIX=${{ github.repository_owner }} \
            BUILD_IMAGE_TAG=${{ needs.stack-images.outputs.version }} \
            RUN_IMAGE_TAG=${{ needs.stack-images.outputs.version }} \
            BUILDER_IMAGE=${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:${{ needs.stack-images.outputs.version }} \
            PLATFORMS=linux/amd64,linux/arm64

      - name: Tag latest for stable release
        if: needs.stack-images.outputs.is_stable == 'true'
        run: |
          docker buildx imagetools create -t ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:latest ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:${{ needs.stack-images.outputs.version }}
```

**Step 3: Remove old build-image and run-image jobs**

Delete the old `build-image:` (lines 17-62) and `run-image:` (lines 64-96) job definitions.

**Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "refactor(ci): simplify workflow to use make targets for multi-arch builds"
```

---

## Task 7: Verify local multi-arch build

**Files:**
- None (verification only)

**Step 1: Clean previous builds**

Run: `make clean`
Expected: Removes all binary directories

**Step 2: Build binaries**

Run: `make build`
Expected: Creates both `buildpack/bin/linux-amd64/` and `buildpack/bin/linux-arm64/`

**Step 3: Verify binary architectures**

Run: `file buildpack/bin/linux-amd64/detect buildpack/bin/linux-arm64/detect`
Expected:
- `buildpack/bin/linux-amd64/detect: ELF 64-bit LSB executable, x86-64`
- `buildpack/bin/linux-arm64/detect: ELF 64-bit LSB executable, ARM aarch64`

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass

---

## Task 8: Final commit and push

**Step 1: Review all changes**

Run: `git status && git diff --stat`
Expected: All changes staged or committed

**Step 2: Push to remote**

Run: `git push origin main`
Expected: All commits pushed successfully

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add arm64 to package.toml | `buildpack/package.toml` |
| 2 | Multi-arch Go build | `Makefile` |
| 3 | PLATFORMS variable | `Makefile` |
| 4 | Multi-arch stack images | `Makefile` |
| 5 | Per-arch builder targets | `Makefile` |
| 6 | Simplify CI workflow | `.github/workflows/release.yml` |
| 7 | Verify local build | - |
| 8 | Push changes | - |
