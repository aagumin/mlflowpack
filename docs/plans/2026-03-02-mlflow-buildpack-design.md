# MLflow Model Buildpack Design

**Date:** 2026-03-02
**Author:** Claude
**Status:** Approved

## Overview

CNCF Buildpack для сборки container образов с ML моделями из MLflow Model Registry. Buildpack работает в unprivileged режиме и совместим с kpack, pack, и кастомными Kubernetes операторами.

## Goals

- Сборка образов для ML моделей из MLflow Model Registry
- Unprivileged (rootless) сборка с минимальными security constraints
- Поддержка MLServer от Seldon как runtime
- Работа локально (pack) и в Kubernetes (kpack, кастомный оператор)
- Установка Python зависимостей через uv

## Non-Goals

- Поддержка других ML runtimes (Triton, TensorFlow Serving)
- Обучение моделей (только упаковка готовых)
- Multi-model контейнеры

## Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        Platform (kpack/оператор)                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Lifecycle                             │    │
│  │  detector → analyzer → restorer → builder → exporter    │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              MLflow Model Buildpack                      │    │
│  │                                                          │    │
│  │  ┌──────────┐    ┌──────────┐    ┌──────────────────┐  │    │
│  │  │  Detect  │    │  Build   │    │                  │  │    │
│  │  │          │───▶│          │───▶│  Layers Created  │  │    │
│  │  │ MLmodel? │    │ Download │    │                  │  │    │
│  │  └──────────┘    │ Parse    │    │ • python/        │  │    │
│  │                  │ Install  │    │ • venv/          │  │    │
│  │                  └──────────┘    │ • model/         │  │    │
│  │                                  └──────────────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    Run Image                             │    │
│  │                  (Fedora + MLServer)                     │    │
│  │                                                          │    │
│  │  /layers/python/     → Python X.Y                        │    │
│  │  /layers/venv/       → Dependencies                      │    │
│  │  /layers/model/      → MLflow model artifacts            │    │
│  │                                                          │    │
│  │  ENTRYPOINT: mlserver start /layers/model                │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Input Parameters

**Service Bindings (credentials):**
```
/bindings/mlflow/
├── type           # "mlflow"
├── tracking_uri   # https://mlflow.example.com
├── username       # (optional)
├── password       # (optional)
└── s3/            # S3 credentials для артефактов
    ├── endpoint
    ├── access_key
    └── secret_key
```

**Environment Variables (model parameters):**
```
BP_MLFLOW_MODEL_NAME=bert-classifier
BP_MLFLOW_MODEL_VERSION=1        # или "latest"
BP_MLFLOW_MODEL_STAGE=Production # альтернатива version
```

## Project Structure

```
aipack/
├── buildpack/                    # CNB Buildpack
│   ├── cmd/
│   │   ├── detect/
│   │   │   └── main.go
│   │   └── build/
│   │       └── main.go
│   ├── internal/
│   │   ├── detect/
│   │   │   └── detector.go
│   │   ├── build/
│   │   │   └── builder.go
│   │   ├── mlflow/
│   │   │   ├── client.go
│   │   │   ├── model.go
│   │   │   └── storage/
│   │   │       └── s3.go
│   │   ├── conda/
│   │   │   └── parser.go
│   │   ├── python/
│   │   │   └── installer.go
│   │   └── layer/
│   │       └── layers.go
│   ├── buildpack.toml
│   ├── package.toml
│   └── go.mod
├── stack/
│   ├── build/
│   │   └── Dockerfile
│   ├── run/
│   │   └── Dockerfile
│   └── stack.toml
├── dev/
│   ├── train.py
│   └── requirements.txt
├── tests/
│   ├── unit/
│   │   ├── conda/
│   │   ├── mlflow/
│   │   ├── detect/
│   │   └── python/
│   └── integration/
│       ├── kind-config.yaml
│       ├── mlflow-deployment.yaml
│       └── test_build.sh
├── docs/
│   └── plans/
├── Makefile
└── builder.toml
```

## Components

### 1. Detect Phase (`bin/detect`)

```
Вход: CNB_APPLICATION_PATH (папка с исходниками)
Логика:
  1. Проверить наличие MLmodel файла в корне
  2. Если есть → Pass: true, Provides: ["mlflow-model"]
  3. Если нет → Pass: false

Build Plan:
  Provides: mlflow-model
  Requires: - (ничего, самодостаточный)
```

### 2. Build Phase (`bin/build`)

```
Вход:
  - CNB_LAYERS_DIR (папка для слоёв)
  - CNB_APPLICATION_PATH (папка с моделью или пустая)
  - Env: BP_MLFLOW_*, Service Bindings

Логика:
  1. Определить источник модели:
     - Если MLmodel в CNB_APPLICATION_PATH → используем
     - Иначе если BP_MLFLOW_MODEL_NAME → скачиваем из MLflow

  2. Создать слои:
     a. python/    — Python нужной версии (из conda.yaml)
     b. venv/      — Virtual environment с зависимостями
     c. model/     — Сами файлы модели

  3. Настроить env:
     - PATH включает python/bin, venv/bin
     - PYTHONPATH указывает на model/
     - MLServer переменные
```

### 3. Layers

| Layer | Тип | Содержимое | Кэширование |
|-------|-----|------------|-------------|
| `python` | build, launch | Python X.Y от uv | По версии Python |
| `venv` | launch | Зависимости модели | По hash conda.yaml |
| `model` | launch | Файлы модели | По model version |

### 4. MLflow Integration

**MLflow Client Interface:**
```go
type Client interface {
    // GetModelVersion возвращает информацию о версии модели
    GetModelVersion(ctx context.Context, name, version string) (*ModelVersion, error)

    // DownloadModel скачивает артефакты модели в destDir
    DownloadModel(ctx context.Context, artifactURI, destDir string) error
}

type ModelVersion struct {
    Name        string
    Version     string
    ArtifactURI string  // s3://bucket/path/to/artifacts
    Status      string
}
```

**Storage Backend Interface:**
```go
type StorageBackend interface {
    Supports(uri string) bool
    Download(ctx context.Context, uri, destDir string) error
}
```

**Download Flow:**
```
1. Parse BP_MLFLOW_MODEL_NAME, BP_MLFLOW_MODEL_VERSION
2. Get bindings from CNB_BINDINGS_DIR
3. Client.GetModelVersion() → получить ArtifactURI
4. Parse ArtifactURI scheme (s3://, gs://, file://)
5. Select StorageBackend by scheme
6. StorageBackend.Download() → скачать в layer
```

Reference implementation: `modelpack/modctl` pkg/modelprovider

### 5. Conda Parser

**Conda.yaml structure (from MLflow):**
```yaml
channels:
  - defaults
  - conda-forge
dependencies:
  - python=3.10.13
  - pip
  - pip:
      - torch==2.1.0
      - transformers==4.35.0
      - numpy>=1.24.0
```

**Parser Interface:**
```go
type CondaFile struct {
    Channels    []string
    Dependencies []Dependency
}

type Dependency struct {
    Name    string  // python
    Version string  // 3.10.13
    Pip     []string // pip dependencies
}

func ParseFile(path string) (*CondaFile, error)
func (c *CondaFile) PythonVersion() string
func (c *CondaFile) PipDependencies() []string
```

### 6. Python Installer (uv)

```go
type Installer struct {
    uvPath string
}

func (i *Installer) InstallPython(ctx context.Context, version, destDir string) error
// uv python install 3.10.13 --python-dir destDir

func (i *Installer) CreateVenv(ctx context.Context, pythonDir, venvDir string) error
// uv venv --python pythonDir/bin/python venvDir

func (i *Installer) InstallDeps(ctx context.Context, venvDir string, deps []string) error
// uv pip install --python venvDir/bin/python deps...
```

uv предустановлен в build image stack.

## Stack

### Build Image

```dockerfile
# stack/build/Dockerfile
FROM fedora:40

RUN dnf install -y \
    git \
    curl \
    tar \
    gzip \
    && dnf clean all

# Install uv
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:$PATH"

# CNB user
ENV CNB_USER_ID=1000
ENV CNB_GROUP_ID=1000
RUN groupadd -g ${CNB_GROUP_ID} cnb && \
    useradd -u ${CNB_USER_ID} -g cnb -m -s /bin/bash cnb

# CNB directories
ENV CNB_STACK_ID="io.amazme.fedora-mlserver"
ENV CNB_BUILDPACKS_DIR="/cnb/buildpacks"
ENV CNB_LAYERS_DIR="/layers"
ENV CNB_APP_DIR="/workspace"

RUN mkdir -p ${CNB_BUILDPACKS_DIR} ${CNB_LAYERS_DIR} ${CNB_APP_DIR} && \
    chown -R cnb:cnb /cnb /layers /workspace

USER cnb
WORKDIR /workspace
```

### Run Image

```dockerfile
# stack/run/Dockerfile
FROM fedora:40

RUN dnf install -y \
    python3 \
    python3-pip \
    && dnf clean all

# Install MLServer
RUN pip3 install mlserver mlserver-sklearn mlserver-xgboost mlserver-lightgbm

# CNB user (non-root)
ENV CNB_USER_ID=1000
ENV CNB_GROUP_ID=1000
RUN groupadd -g ${CNB_GROUP_ID} cnb && \
    useradd -u ${CNB_USER_ID} -g cnb -m -s /bin/bash cnb

ENV CNB_STACK_ID="io.amazme.fedora-mlserver"
LABEL io.buildpacks.stack.id="io.amazme.fedora-mlserver"

# MLServer defaults
ENV MLSERVER_MODEL_NAME="model"
ENV MLSERVER_MODEL_URI="/opt/mlflow/model"
ENV MLSERVER_MODEL_IMPLEMENTATION="mlserver_mlflow.MLflowRuntime"

RUN mkdir -p /opt/mlflow && chown -R cnb:cnb /opt/mlflow

USER cnb
WORKDIR /home/cnb

EXPOSE 8080
ENTRYPOINT ["mlserver", "start", "/opt/mlflow/model"]
```

### Stack Metadata

```toml
# stack/stack.toml
[stack]
id = "io.amazme.fedora-mlserver"
build-image = "registry.example.com/amazme/fedora-mlserver-build:40"
run-image = "registry.example.com/amazme/fedora-mlserver-run:40"
```

## Dev Example

```python
# dev/train.py
import mlflow
from sklearn.datasets import load_iris
from sklearn.ensemble import RandomForestClassifier
import yaml
import os

def train():
    X, y = load_iris(return_X_y=True)
    model = RandomForestClassifier(n_estimators=100)
    model.fit(X, y)

    mlflow.set_tracking_uri(os.getenv("MLFLOW_TRACKING_URI", "http://localhost:5000"))

    with mlflow.start_run():
        mlflow.sklearn.log_model(model, "model", registered_model_name="iris-classifier")

        conda_env = {
            "channels": ["defaults", "conda-forge"],
            "dependencies": [
                "python=3.10.13",
                {"pip": ["scikit-learn==1.3.0", "mlserver==1.3.0"]}
            ]
        }
        with open("conda.yaml", "w") as f:
            yaml.dump(conda_env, f)

        print("Model registered: iris-classifier")

if __name__ == "__main__":
    train()
```

## Testing

### Unit Tests

```
tests/unit/
├── conda/
│   ├── parser_test.go
│   └── testdata/
│       ├── basic.yaml
│       └── complex.yaml
├── mlflow/
│   ├── client_test.go
│   └── storage/s3_test.go
├── detect/
│   └── detector_test.go
└── python/
    └── installer_test.go
```

### Integration Tests (Kind)

```yaml
# tests/integration/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
```

Test flow:
1. Create Kind cluster
2. Deploy MLflow server
3. Train sample model
4. Build image with pack
5. Verify image works
6. Cleanup

## Configuration Files

### buildpack.toml

```toml
api = "0.10"

[buildpack]
id = "io.amazme.buildpacks.mlflow-model"
version = "0.1.0"
name = "MLflow Model Buildpack"
homepage = "https://github.com/amazme/aipack"
description = "Buildpack for MLflow models with MLServer runtime"

[[targets]]
os = "linux"
arch = "amd64"

[[targets]]
os = "linux"
arch = "arm64"

[[stacks]]
id = "io.amazme.fedora-mlserver"

[metadata]
include-files = ["bin/detect", "bin/build"]
```

### Makefile

```makefile
.PHONY: all build test lint clean package

BUILDPACK_ID := io.amazme.buildpacks.mlflow-model
VERSION := $(shell git describe --tags --always --dirty)

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/detect ./cmd/detect
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/build ./cmd/build

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

package: build
	pack buildpack package ${BUILDPACK_ID} \
		--config package.toml \
		--tag ${BUILDPACK_ID}:${VERSION}

stack-build:
	podman build -t amazme/fedora-mlserver-build:40 stack/build

stack-run:
	podman build -t amazme/fedora-mlserver-run:40 stack/run

stack: stack-build stack-run

builder: stack package
	pack builder create amazme/mlserver-builder:${VERSION} \
		--config builder.toml \
		--pull-policy if-not-present

test-integration:
	./tests/integration/test_build.sh

clean:
	rm -rf bin/
	rm -rf out/
```

### go.mod

```go
module github.com/amazme/aipack/buildpack

go 1.25

require (
    github.com/buildpacks/libcnb/v2 v2.0.0
    github.com/paketo-buildpacks/libpak/v2 v2.0.0
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/config v1.26.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.47.0
    gopkg.in/yaml.v3 v3.0.1
)
```

## Security Considerations

Buildpack работает в unprivileged режиме с следующими constraints:

```yaml
securityContext:
  privileged: false
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
  runAsNonRoot: true
  runAsUser: 1001
  runAsGroup: 1001
  fsGroup: 1001
```

volumes:
```yaml
volumes:
  - name: cache-volume
    emptyDir:
      medium: Memory
      sizeLimit: 4096Mi
```

## References

- CNCF Buildpacks: https://buildpacks.io
- Lifecycle: https://github.com/buildpacks/lifecycle
- libcnb: https://github.com/buildpacks/libcnb
- libpak: https://github.com/paketo-buildpacks/libpak
- Model provider reference: https://github.com/modelpack/modctl (pkg/modelprovider)
- MLServer: https://github.com/SeldonIO/MLServer
