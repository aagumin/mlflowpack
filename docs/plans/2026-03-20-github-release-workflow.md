# GitHub Release Workflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add GitHub Actions workflow для автоматической публикации build/run/builder образов в ghcr.io при push тега семантической версии.

**Architecture:** Workflow с 3 последовательными job'ами (build → run → builder), использующий make команды с параметризацией registry/image prefix. Makefile расширяется переменными для кастомизации.

**Tech Stack:** GitHub Actions, Docker Buildx, Pack CLI, Make

---

## Task 1: Parameterize Makefile for registry flexibility

**Files:**
- Modify: `Makefile:49-55`

**Step 1: Add registry variables to Makefile**

Add after line 5 (after VERSION definition):

```makefile
# Registry configuration (override for ghcr.io)
REGISTRY ?= docker.io
IMAGE_PREFIX ?= aagumin
BUILD_IMAGE_TAG ?= 43
RUN_IMAGE_TAG ?= 43
```

**Step 2: Update stack-build target**

Replace `stack-build` target (lines 49-50):

```makefile
stack-build:
	$(CONTAINER_TOOL) build -t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-build:$(BUILD_IMAGE_TAG) stack/build
```

**Step 3: Update stack-run target**

Replace `stack-run` target (lines 52-53):

```makefile
stack-run:
	$(CONTAINER_TOOL) build -t $(REGISTRY)/$(IMAGE_PREFIX)/fedora-mlserver-run:$(RUN_IMAGE_TAG) stack/run
```

**Step 4: Verify local build still works**

Run: `make stack-build stack-run`
Expected: Images built with default docker.io/aagumin/... names

**Step 5: Commit**

```bash
git add Makefile
git commit -m "refactor(make): add registry parameterization for CI support"
```

---

## Task 2: Create GitHub Actions workflow file

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create workflow directory**

Run: `mkdir -p .github/workflows`

**Step 2: Create release.yml with trigger and permissions**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
      - 'v[0-9]+.[0-9]+.[0-9]+-*'

permissions:
  contents: read
  packages: write

env:
  REGISTRY: ghcr.io

jobs:
```

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add release workflow skeleton"
```

---

## Task 3: Add build-image job

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Add version extraction and build-image job**

Append to `.github/workflows/release.yml` after `jobs:`:

```yaml
  build-image:
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

      - name: Build and push build image
        run: |
          make stack-build \
            REGISTRY=${{ env.REGISTRY }} \
            IMAGE_PREFIX=${{ github.repository_owner }} \
            BUILD_IMAGE_TAG=${{ steps.version.outputs.VERSION }}

      - name: Tag latest for stable release
        if: steps.stable.outputs.IS_STABLE == 'true'
        run: |
          docker tag ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:${{ steps.version.outputs.VERSION }} \
            ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:latest
          docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:latest

      - name: Push build image
        run: docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:${{ steps.version.outputs.VERSION }}
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add build-image job"
```

---

## Task 4: Add run-image job

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Add run-image job**

Append after `build-image` job:

```yaml
  run-image:
    needs: build-image
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

      - name: Build and push run image
        run: |
          make stack-run \
            REGISTRY=${{ env.REGISTRY }} \
            IMAGE_PREFIX=${{ github.repository_owner }} \
            RUN_IMAGE_TAG=${{ needs.build-image.outputs.version }}

      - name: Tag latest for stable release
        if: needs.build-image.outputs.is_stable == 'true'
        run: |
          docker tag ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:${{ needs.build-image.outputs.version }} \
            ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:latest
          docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:latest

      - name: Push run image
        run: docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:${{ needs.build-image.outputs.version }}
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add run-image job"
```

---

## Task 5: Add builder-image job

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Add builder-image job**

Append after `run-image` job:

```yaml
  builder-image:
    needs: [build-image, run-image]
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
        uses: buildpacks/github-actions/setup-pack@v5

      - name: Package buildpack
        run: |
          make build
          pack buildpack package io.github.aagumin.mlflow-model:${{ needs.build-image.outputs.version }} \
            --config buildpack/package.toml \
            --force-color

      - name: Create builder
        run: |
          pack builder create ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:${{ needs.build-image.outputs.version }} \
            --config builder.toml \
            --pull-policy never \
            --verbose

      - name: Push builder image
        run: docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:${{ needs.build-image.outputs.version }}

      - name: Tag and push latest for stable release
        if: needs.build-image.outputs.is_stable == 'true'
        run: |
          docker tag ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:${{ needs.build-image.outputs.version }} \
            ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:latest
          docker push ${{ env.REGISTRY }}/${{ github.repository_owner }}/mlserver-builder:latest
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add builder-image job"
```

---

## Task 6: Update builder.toml for ghcr.io images

**Files:**
- Modify: `builder.toml`

**Step 1: Read current builder.toml**

Run: `cat builder.toml`

**Step 2: Update builder.toml to use variable or document override**

The builder.toml currently references:
```toml
[stack]
id = "io.github.aagumin.mlflow-model"
run-image = "aagumin/fedora-mlserver-run:43"
build-image = "aagumin/fedora-mlserver-build:43"
```

For CI, we need to either:
- A) Create a separate `builder-ghcr.toml` template
- B) Use sed to replace image names before `pack builder create`

Add step in workflow (modify Task 5):

```yaml
      - name: Prepare builder config for ghcr.io
        run: |
          sed -i.bak \
            -e "s|aagumin/fedora-mlserver-run:43|${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-run:${{ needs.build-image.outputs.version }}|g" \
            -e "s|aagumin/fedora-mlserver-build:43|${{ env.REGISTRY }}/${{ github.repository_owner }}/fedora-mlserver-build:${{ needs.build-image.outputs.version }}|g" \
            builder.toml
```

**Step 3: Commit**

```bash
git add builder.toml
git commit -m "docs(ci): note builder.toml override in workflow"
```

---

## Task 7: Verify workflow syntax

**Files:**
- Verify: `.github/workflows/release.yml`

**Step 1: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No output (valid YAML)

**Step 2: Review final workflow**

Run: `cat .github/workflows/release.yml`

Verify structure:
- Trigger on semantic version tags
- 3 jobs: build-image → run-image → builder-image
- Each job: checkout → login → build → push
- Latest tagging for stable releases

**Step 3: Final commit if any fixes**

```bash
git add .github/workflows/release.yml
git commit -m "fix(ci): correct workflow syntax"  # if needed
```

---

## Task 8: Documentation

**Files:**
- Modify: `README.md` or `docs/USAGE.md`

**Step 1: Add release process section**

Add to documentation:

```markdown
## Release Process

Creating a new tag triggers automatic image publishing to ghcr.io:

```bash
# Create and push a tag
git tag v1.0.0
git push origin v1.0.0
```

### Published Images

| Image | Tag |
|-------|-----|
| `ghcr.io/aagumin/fedora-mlserver-build` | `VERSION`, `latest` (stable only) |
| `ghcr.io/aagumin/fedora-mlserver-run` | `VERSION`, `latest` (stable only) |
| `ghcr.io/aagumin/mlserver-builder` | `VERSION`, `latest` (stable only) |

### Pre-releases

Tags like `v1.0.0-beta.1` or `v1.0.0-rc.2` are published without the `latest` tag.
```

**Step 2: Commit**

```bash
git add docs/USAGE.md  # or README.md
git commit -m "docs: add release process documentation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Parameterize Makefile | `Makefile` |
| 2 | Create workflow skeleton | `.github/workflows/release.yml` |
| 3 | Add build-image job | `.github/workflows/release.yml` |
| 4 | Add run-image job | `.github/workflows/release.yml` |
| 5 | Add builder-image job | `.github/workflows/release.yml` |
| 6 | Update builder config handling | `builder.toml` note |
| 7 | Verify workflow syntax | `.github/workflows/release.yml` |
| 8 | Add documentation | `docs/USAGE.md` |
