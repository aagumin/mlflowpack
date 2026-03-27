# MLflow Model Buildpack

CNCF Buildpack for building container images with ML models. Supports local models and S3 storage paths. Uses MLServer as runtime and installs Python dependencies via uv. Works with pack and custom Kubernetes operators in unprivileged mode.

## Features

- **Unprivileged builds** — works in rootless mode without special privileges
- **MLServer runtime** — uses Seldon MLServer for inference
- **Auto flavor detection** — automatically selects the correct MLServer extension
- **Fast dependency installation** — uses uv instead of pip
- **Multi-architecture** — supports linux/amd64 and linux/arm64
- **Intelligent layer caching** — reuses layers based on model UUID and dependency hash
- **S3 support** — build directly from S3 with AWS SDK integration
- **Two-phase download** — metadata-first download for optimized cache decisions
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

### Option 2: Model from S3

```bash
pack build my-model-image \
  --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always --dirty) \
  --env BP_MLFLOW_MODEL_PATH="s3://my-bucket/models/my-classifier/v1" \
  --env BP_MLFLOW_MODEL_NAME="my-classifier" \
  --env BP_MLFLOW_MODEL_VERSION="1" \
  --env AWS_ACCESS_KEY_ID="your-access-key" \
  --env AWS_SECRET_ACCESS_KEY="your-secret-key" \
  --env AWS_REGION="us-east-1" \
  --pull-policy never \
  --trust-builder
```

For custom S3 endpoints (MinIO, etc.):
```bash
--env AWS_ENDPOINT_URL="https://minio.example.com"
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
| `BP_MLFLOW_MODEL_PATH` | Path to model: `s3://bucket/path`, `/local/path`, or relative path | auto-detect |
| `BP_MLFLOW_MODEL_NAME` | Model name (for image labels) | `model` |
| `BP_MLFLOW_MODEL_VERSION` | Model version (for image labels) | `latest` |
| `BP_MLFLOW_PREV_DEPS_HASH` | Previous dependency hash for cache optimization | — |
| `BP_MLFLOW_WORK_DIR` | Scratch directory for downloads | `<layers>/work` |

### Path Detection

1. If `BP_MLFLOW_MODEL_PATH` starts with `s3://` → S3 storage
2. If `BP_MLFLOW_MODEL_PATH` is an absolute path (`/...`) → Local absolute path
3. If `BP_MLFLOW_MODEL_PATH` is a relative path → Relative to `--path`
4. `MLmodel` in root of `--path` → Auto-detected
5. Recursive search for single `MLmodel` under `--path`

### S3 Authentication

The buildpack uses standard AWS SDK authentication:

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
export AWS_ENDPOINT_URL="https://minio.example.com"  # Optional: custom endpoint

# Build
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --env BP_MLFLOW_MODEL_PATH="s3://bucket/models/v1"
```

## Buildpack Features

### Build Plan

The buildpack provides and requires `mlflow-model` in the build plan during detect phase. This enables:
- Standalone operation (self-contained requires/provides)
- Other buildpacks can depend on this buildpack

### Layer Caching

The buildpack implements intelligent layer caching:

**Local Models (UUID-based):**
```
Model unchanged (UUID: abc123...), reusing cached layers
```

**S3/Storage Models (Dependency Hash-based):**
```
Dependency hash: sha256:abc123...
Previous hash: sha256:abc123...
Dependencies unchanged, reusing cached Python and venv layers
```

For S3 models, the buildpack uses two-phase download:
1. Download only metadata (MLmodel, conda.yaml, requirements.txt)
2. Compute dependency hash and compare with cache
3. Skip Python/venv rebuild if dependencies unchanged
4. Download full model to model layer

This significantly speeds up rebuilds when only model version changes.

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
