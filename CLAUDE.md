# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CNCF Buildpack for building container images with ML models from MLflow Model Registry. Uses MLServer as runtime and installs Python dependencies via uv. Works with kpack, pack, and custom Kubernetes operators in unprivileged mode.

## Build Commands

```bash
# Build buildpack binaries
make build

# Build with specific container tool (default: docker)
CONTAINER_TOOL=podman make build

# Run tests
cd buildpack && go test -v -race ./...

# Build stack images
make stack

# Package buildpack
make package

# Create builder
make builder

# Full build cycle
CONTAINER_TOOL=podman make stack package builder
```

## Test Commands

```bash
# kpack integration tests in kind cluster
make kpack-setup      # Setup kpack in cluster
make kpack-deploy      # Deploy test resources
make kpack-test-build  # Wait for build
make kpack-test-inference  # Run inference test
make kpack-cleanup     # Cleanup resources
make kpack-all        # Full test cycle

# Local test with pack
make create-test-model  # Create test sklearn model
make test-build        # Build with local pack
```

## Architecture

### Buildpack Phases

**Detect** (`cmd/detect`):
- Checks for `MLmodel` file or `BP_MLFLOW_MODEL_NAME` env var
- Returns build plan if model found

**Build** (`cmd/build`):
- Parses `MLmodel` file to detect flavor (sklearn, xgboost, lightgbm, etc.)
- Creates three layers: `python/`, `venv/`, `model/`
- Installs Python via uv (from conda.yaml or defaults to 3.10)
- Installs dependencies including required MLServer extension
- Copies model artifacts
- Sets MLServer environment variables

### Flavor Detection (`internal/mlflow/flavor.go`)

Maps MLflow model flavors to MLServer extensions:
| Flavor | Pip Package | Runtime |
|--------|-------------|---------|
| sklearn | mlserver-sklearn | mlserver_sklearn.SKLearnRuntime |
| xgboost | mlserver-xgboost | mlserver_xgboost.XGBoostRuntime |
| lightgbm | mlserver-lightgbm | mlserver_lightgbm.LightGBMRuntime |
| tensorflow | mlserver-tensorflow | mlserver_tensorflow.TensorFlowRuntime |
| pytorch | mlserver-torchserve | mlserver_torchserve.TorchServeRuntime |
| transformers | mlserver-huggingface | mlserver_huggingface.HuggingFaceRuntime |

Priority order: sklearn > xgboost > lightgbm > tensorflow > pytorch > transformers > mlflow > python_function

### Key Files

- `buildpack/buildpack.toml` - Buildpack metadata with `[[targets]]` (os/arch)
- `builder.toml` - Builder config with `[build]` and `[[run.images]]`
- `stack/build/Dockerfile` - Build image with uv, git, curl
- `stack/run/Dockerfile` - Run image with only mlserver + mlserver-mlflow

### Service Bindings

Buildpack reads credentials from `/bindings/mlflow/`:
```
/bindings/mlflow/
├── type           # "mlflow"
├── tracking_uri   # https://mlflow.example.com
├── username       # (optional)
├── password       # (optional)
└── s3/
    ├── endpoint
    ├── access_key
    └── secret_key
```

### Environment Variables

- `BP_MLFLOW_MODEL_NAME` - Model name from registry
- `BP_MLFLOW_MODEL_VERSION` - Model version (or "latest")
- `BP_MLFLOW_MODEL_STAGE` - Model stage (alternative to version)

## Image Requirements

- Run image: Only `mlserver==1.7.1` + `mlserver-mlflow` (minimal base)
- Buildpack dynamically installs flavor-specific extension (mlserver-sklearn, mlserver-xgboost, etc.)
- Uses Python 3.11 (compatible with MLServer 1.7.1 which requires Python <3.13)

## Stack Images

Base: `quay.io/fedora/fedora-minimal:43`
Uses Yandex mirror for package downloads.

Build image includes: git-core, curl, tar, gzip, shadow-utils, uv

Run image includes: python3.11, mlserver==1.7.1, mlserver-mlflow
