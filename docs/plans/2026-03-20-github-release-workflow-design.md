# GitHub Release Workflow Design

## Overview

GitHub Actions workflow для автоматической публикации образов в ghcr.io при создании тега с семантической версией.

## Requirements

- Триггер на теги семантических версий (включая пре-релизы)
- Публикация build, run, builder образов в ghcr.io
- Консистентность с локальной сборкой через Makefile
- Платформа: linux/amd64

## Trigger

```yaml
on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
      - 'v[0-9]+.[0-9]+.[0-9]+-*'
```

Примеры: `v1.0.0`, `v1.0.0-beta.1`, `v2.1.0-rc.2`

## Images

| Образ | ghcr.io tag |
|-------|-------------|
| build | `ghcr.io/${OWNER}/fedora-mlserver-build:${VERSION}` |
| run | `ghcr.io/${OWNER}/fedora-mlserver-run:${VERSION}` |
| builder | `ghcr.io/${OWNER}/mlserver-builder:${VERSION}` |

`${VERSION}` — тег без префикса `v` (например `1.0.0-beta.1`)

Дополнительно: тег `latest` для стабильных версий (без суффикса `-beta`, `-rc`, etc.)

## Workflow Structure

```yaml
jobs:
  build-image:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - docker/setup-buildx-action
      - make stack-build (с параметрами ghcr.io)

  run-image:
    needs: build-image
    steps:
      - checkout
      - docker/setup-buildx-action
      - make stack-run (с параметрами ghcr.io)

  builder-image:
    needs: [build-image, run-image]
    steps:
      - checkout
      - docker/setup-buildx-action
      - setup-pack
      - make package
      - create builder
      - push to ghcr.io
```

## Makefile Parameterization

Добавить переменные для кастомизации registry и image prefix:

```makefile
REGISTRY ?= docker.io
IMAGE_PREFIX ?= aagumin
BUILD_TAG ?= 43
```

Workflow вызывает:
```bash
make stack-build REGISTRY=ghcr.io IMAGE_PREFIX=${OWNER} BUILD_TAG=${VERSION}
make stack-run REGISTRY=ghcr.io IMAGE_PREFIX=${OWNER} BUILD_TAG=${VERSION}
```

## Permissions

```yaml
permissions:
  contents: read
  packages: write
```

## Version Extraction

```yaml
- name: Get version
  id: version
  run: echo "VERSION=${GITHUB_REF#refs/tags/v}" >> $GITHUB_OUTPUT
```

## Latest Tag Logic

Для стабильных версий (без `-beta`, `-rc`, etc.) добавлять тег `latest`:

```yaml
- name: Check if stable
  id: stable
  run: |
    if [[ ! "${VERSION}" =~ -(beta|rc|alpha) ]]; then
      echo "IS_STABLE=true" >> $GITHUB_OUTPUT
    fi
```
