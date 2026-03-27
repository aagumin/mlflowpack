# MLflow Model Buildpack

CNCF Buildpack for building container images with ML models from MLflow Model Registry. Uses MLServer as runtime and installs Python dependencies via uv. Works with pack and custom Kubernetes operators in unprivileged mode.

## Features

- **Unprivileged builds** — works in rootless mode without special privileges
- **MLServer runtime** — uses Seldon MLServer for inference
- **Auto flavor detection** — automatically selects the correct MLServer extension
- **Fast dependency installation** — uses uv instead of pip
- **Multi-architecture** — supports linux/amd64 and linux/arm64
- **Layer caching** — reuses layers when model hasn't changed (UUID-based)
- **SBOM support** — generates CycloneDX SBOM for dependencies
- **Image labels** — OCI and MLflow-specific labels on output images
- **Versioned from git tags** — buildpack version follows repository tags

## Supported Models

| Flavor | Pip Package | Runtime |
|--------|-------------|---------|
| sklearn | mlserver-sklearn | mlserver_sklearn.SKLearnModel |
| xgboost | mlserver-xgboost | mlserver_xgboost.XGBoostModel |
| lightgbm | mlserver-lightgbm | mlserver_lightgbm.LightGBMModel |
| tensorflow | mlserver-tensorflow | mlserver_tensorflow.TensorFlowModel |
| pytorch | mlserver-torchserve | mlserver_torchserve.TorchServeModel |
| transformers | mlserver-huggingface | mlserver_huggingface.HuggingFaceModel |

## Quick Start

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [pack](https://buildpacks.io/docs/tools/pack/) | >= 0.38.0 | CLI for working with buildpacks |
| Docker or Podman | any | Container runtime |
| Go | >= 1.24 | For buildpack development |

### Installation

```bash
# macOS
brew install pack

# Linux
# pack: https://buildpacks.io/docs/tools/pack/
# Go: https://go.dev/doc/install
```

## Building the Buildpack Locally

```bash
# Clone repository
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack

# Build builder (stack images + buildpack package + builder)
make builder

# Or step by step:
make build     # Compile buildpack binaries (amd64 + arm64)
make test      # Run unit tests
make stack     # Build stack images (multi-arch)
make package   # Package buildpack
make builder   # Create builder image
```

The buildpack version is derived from git tags:
```bash
# Check current version
git describe --tags --always --dirty

# Create a release tag
git tag v1.0.0
make package  # Buildpack will be versioned as v1.0.0
```

## Building a Model Image

### Option 1: Local Model (e2e example)

```bash
# Use test model from e2e
pack build my-sklearn-model:latest \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --path e2e/models/sklearn \
  --pull-policy never \
  --trust-builder

# Run
docker run --rm -p 8080:8080 -e MLSERVER_PARALLEL_WORKERS=0 my-sklearn-model:latest

# Test inference
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json
```

### Option 2: Model from MLflow Registry

```bash
pack build my-model-image \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.example.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass" \
  --pull-policy never \
  --trust-builder
```

### Run and Test

```bash
# Check readiness
curl http://localhost:8080/v2/health/ready

# Arbitrary inference request
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{"inputs": [{"name": "input", "shape": [1, 4], "datatype": "FP32", "data": [[5.1, 3.5, 1.4, 0.2]]}]}'
```

### E2E Testing

```bash
# Full e2e cycle (pyfunc + sklearn models)
make e2e

# Or manually
./e2e/scripts/verify-build.sh sklearn
./e2e/scripts/verify-runtime.sh sklearn
```

## Makefile Commands

```bash
make build     # Build buildpack binaries (amd64 + arm64)
make test      # Run unit tests
make lint      # Run linter
make stack     # Build stack images (multi-arch)
make package   # Package buildpack (versioned from git tag)
make builder   # Create builder (stack + package)
make e2e       # Build+runtime checks for e2e models
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BP_MLFLOW_MODEL_NAME` | Model name in Registry | — |
| `BP_MLFLOW_MODEL_VERSION` | Model version | `latest` |
| `BP_MLFLOW_MODEL_STAGE` | Model stage (Production, Staging) | — |
| `BP_MLFLOW_MODEL_PATH` | Local path to model OR `models:/<name>[/<version-or-stage>]` for Registry | auto-detect |
| `BP_MLFLOW_WORK_DIR` | Scratch directory for downloads | `<layers>/work` |

If `BP_MLFLOW_MODEL_PATH` starts with `models:/`, the buildpack downloads the model from Registry.
Otherwise, local model is auto-detected by `MLmodel` file (root or recursive search).
If multiple `MLmodel` files found, specify `BP_MLFLOW_MODEL_PATH`.

### Service Bindings

```
/bindings/mlflow/
├── type           # "mlflow"
├── tracking_uri   # https://mlflow.example.com
├── username       # (optional)
├── password       # (optional)
└── s3/            # S3 credentials (optional)
    ├── endpoint
    ├── access_key
    └── secret_key
```

### Environment Variables (alternative to bindings)

```bash
# MLflow Registry
export MLFLOW_TRACKING_URI="https://mlflow.example.com"
export MLFLOW_TRACKING_USERNAME="your-username"
export MLFLOW_TRACKING_PASSWORD="your-password"

# S3 (for artifacts)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# Build
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/Production"
```

## Buildpack Features

### Build Plan

The buildpack provides and requires `mlflow-model` in the build plan during detect phase. This enables:
- Standalone operation (self-contained requires/provides)
- Other buildpacks can depend on this buildpack

### Layer Reuse

The buildpack automatically reuses cached layers when the model hasn't changed. Change detection uses `model_uuid` from the `MLmodel` file:

```
Model unchanged (UUID: abc123...), reusing cached layers
```

This significantly speeds up repeated builds of the same model.

### Image Labels

The buildpack adds labels to the output image:

| Label | Description |
|-------|-------------|
| `org.opencontainers.image.title` | Model name |
| `org.opencontainers.image.version` | Model version |
| `org.opencontainers.image.description` | Image description |
| `io.github.aagumin.model-flavor` | Model flavor (sklearn, pyfunc, etc.) |
| `io.github.aagumin.model-name` | Model name |
| `io.github.aagumin.mlserver-runtime` | MLServer runtime |

Check labels:

```bash
docker inspect --format='{{json .Config.Labels}}' my-model:latest
```

### SBOM

The buildpack generates CycloneDX SBOM for installed dependencies:
- Python packages from venv
- Model metadata

## Documentation

- [USAGE.md](docs/USAGE.md) — detailed user guide
- [CONTRIBUTING.md](CONTRIBUTING.md) — development guide

## License

MIT
