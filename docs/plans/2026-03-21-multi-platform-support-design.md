# Multi-Platform Support Design

## Overview

Добавление поддержки multi-architecture (amd64 + arm64) для buildpack, stack images и builder с использованием Docker manifest lists для прозрачного UX.

## Requirements

- Пользователь делает `pack build myapp --builder .../mlserver-builder` без указания платформы
- Docker автоматически выбирает нужную архитектуру через manifest list
- Makefile — единая точка входа для всех команд сборки
- CI только вызывает make targets с переменными

## Target Platforms

| Platform | Use Case |
|----------|----------|
| linux/amd64 | Intel Mac, Linux servers, GitHub Actions runners |
| linux/arm64 | Apple Silicon (M1-M4), ARM servers |

## Architecture

```
ghcr.io/owner/mlserver-builder:latest (manifest list)
├── linux/amd64 → mlserver-builder:latest-amd64
└── linux/arm64 → mlserver-builder:latest-arm64
```

## Components

### 1. Buildpack Binaries

Go cross-compilation для обеих архитектур:

```
buildpack/bin/
├── linux-amd64/
│   ├── detect
│   └── build
└── linux-arm64/
    ├── detect
    └── build
```

### 2. Stack Images

Multi-arch через `docker buildx`:

```
ghcr.io/owner/fedora-mlserver-build:VERSION (manifest list)
ghcr.io/owner/fedora-mlserver-run:VERSION (manifest list)
```

Base image `quay.io/fedora/fedora-minimal:43` уже multi-arch — Dockerfiles не меняются.

### 3. Builder

`pack builder create` не умеет multi-arch напрямую, поэтому:

1. Создаём отдельные builders для каждой архитектуры
2. Объединяем через `docker manifest create`

### 4. Buildpack Package

`buildpack/package.toml` объявляет оба target:

```toml
[[targets]]
os = "linux"
arch = "amd64"

[[targets]]
os = "linux"
arch = "arm64"
```

## Makefile Design

### New Variables

```makefile
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER_ARCHS := $(subst linux/,,$(PLATFORMS))  # amd64 arm64
```

### New/Updated Targets

| Target | Description |
|--------|-------------|
| `build` | Cross-compile Go binaries for amd64 + arm64 |
| `stack-build` | Multi-arch build via buildx |
| `stack-run` | Multi-arch build via buildx |
| `builder-%` | Create builder for specific architecture |
| `builders` | Create all architecture-specific builders |
| `manifest` | Create manifest list from builders |
| `builder` | Full cycle: stack + builders + manifest |

### Example Usage

```bash
# Local development (single platform, fast)
make build PLATFORMS=linux/arm64

# Full CI build (multi-platform)
make builder \
  REGISTRY=ghcr.io \
  IMAGE_PREFIX=owner \
  BUILD_IMAGE_TAG=1.0.0 \
  RUN_IMAGE_TAG=1.0.0 \
  BUILDER_IMAGE=ghcr.io/owner/mlserver-builder:1.0.0 \
  PLATFORMS=linux/amd64,linux/arm64
```

## CI/CD Changes

GitHub Actions workflow упрощается до вызовов make:

```yaml
- name: Build and push stack images
  run: make stack-build REGISTRY=ghcr.io ... PLATFORMS=linux/amd64,linux/arm64

- name: Create and push builder
  run: make builder REGISTRY=ghcr.io ... PLATFORMS=linux/amd64,linux/arm64
```

## Files Changed

| File | Change |
|------|--------|
| `Makefile` | Add multi-arch build, builder-% targets, manifest |
| `buildpack/package.toml` | Add `[[targets]]` for arm64 |
| `.github/workflows/release.yml` | Simplify to make calls |

## Files Unchanged

| File | Reason |
|------|--------|
| `stack/build/Dockerfile` | Fedora base already multi-arch |
| `stack/run/Dockerfile` | Fedora base already multi-arch |
| `buildpack/buildpack.toml` | Already has both targets |
| `builder.toml.template` | No changes needed |

## Trade-offs

| Aspect | Decision |
|--------|----------|
| Storage | 2x images in registry (acceptable) |
| CI Speed | Slower due to QEMU emulation for arm64 on amd64 runners (acceptable) |
| UX | Transparent platform selection (achieved) |
| Complexity | Moderate, encapsulated in Makefile |

## Success Criteria

- [ ] `make build` produces binaries for both architectures
- [ ] `make stack` creates multi-arch stack images
- [ ] `make builder` creates manifest list with both platforms
- [ ] `pack build --builder .../mlserver-builder:latest` works on both amd64 and arm64 without `--platform` flag
- [ ] CI workflow only calls make targets
