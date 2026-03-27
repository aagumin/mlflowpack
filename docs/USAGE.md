# MLflow Model Buildpack: User Guide

Complete guide for using the buildpack to package ML models into container images.

## Table of Contents

1. [Installation](#installation)
2. [Basic Usage](#basic-usage)
3. [Building from S3](#building-from-s3)
4. [Configuration](#configuration)
5. [Inference](#inference)
6. [Buildpack Features](#buildpack-features)
7. [Troubleshooting](#troubleshooting)

---

## Installation

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| pack | >= 0.38.0 | CLI for working with buildpacks |
| Docker or Podman | any | Container runtime |

### Installing pack

**macOS:**
```bash
brew install pack
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y pack

# Or download binary
(curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.38.2/pack-v0.38.2-linux-$(uname -m | sed 's/x86_64/amd64/').tgz" | sudo tar -C /usr/local/bin/ --no-same-owner -xz pack)
```

### Getting the Builder

**Option 1: Build locally**
```bash
git clone https://github.com/aagumin/mlflowpack.git
cd mlflowpack
make builder
```

**Option 2: Use pre-built (when published)**
```bash
pack builder pull ghcr.io/aagumin/mlserver-builder:latest
```

---

## Basic Usage

### Scenario 1: Local Model

If you have a folder with an MLflow model (with `MLmodel` file):

```
my-model/
├── MLmodel
├── model.pkl
├── conda.yaml
└── requirements.txt
```

```bash
pack build my-model-image \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --path my-model
```

### Scenario 2: Full e2e cycle on test models (pyfunc + sklearn)

The repository contains two ready-to-use MLflow models:
- `e2e/models/pyfunc` (`python_function`)
- `e2e/models/sklearn` (`sklearn`)

You can run them through e2e scripts.

#### Option A: Full cycle with one command

```bash
# 1) Build local builder
make builder

# 2) For both models execute:
#    - build image via buildpack
#    - run container
#    - check readiness (/v2/health/ready)
#    - inference request
#    - compare response with expected-response.json
make e2e
```

#### Option B: Full cycle manually (sklearn example)

```bash
# 1) Build image from e2e model
./e2e/scripts/verify-build.sh sklearn

# 2) Run container
docker run --rm --name e2e-sklearn -p 8080:8080 \
  -e MLSERVER_PARALLEL_WORKERS=0 \
  aipack-e2e-sklearn:local
```

In another terminal:

```bash
# 3) Check readiness
curl -fsS http://localhost:8080/v2/health/ready

# 4) Send inference request from test file
curl -fsS -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json

# 5) Quick preview of predictions (expected [0, 1])
curl -fsS -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d @e2e/models/sklearn/test-request.json | \
python3 -c 'import json,sys; print(json.load(sys.stdin)["outputs"][0]["data"])'
```

#### Option C: Full cycle manually (pyfunc example)

```bash
# Build + run + readiness + infer + check expected-response.json
./e2e/scripts/verify-runtime.sh pyfunc

# Expected inference output_data: [4.0, 5.0]
```

---

## Building from S3

### Building with S3 Model Path

Build directly from models stored in S3:

```bash
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --env BP_MLFLOW_MODEL_PATH="s3://my-bucket/models/my-model/v1" \
  --env BP_MLFLOW_MODEL_NAME="my-model" \
  --env BP_MLFLOW_MODEL_VERSION="1" \
  --env AWS_ACCESS_KEY_ID="your-access-key" \
  --env AWS_SECRET_ACCESS_KEY="your-secret-key" \
  --env AWS_REGION="us-east-1"
```

### S3 Authentication

The buildpack uses standard AWS SDK authentication:

| Variable | Description |
|----------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region (default: us-east-1) |
| `AWS_ENDPOINT_URL` | Custom S3 endpoint (MinIO, etc.) |

### Custom S3 Endpoints (MinIO, etc.)

```bash
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --env BP_MLFLOW_MODEL_PATH="s3://mlflow-models/my-model/1" \
  --env AWS_ENDPOINT_URL="https://minio.example.com" \
  --env AWS_ACCESS_KEY_ID="minioadmin" \
  --env AWS_SECRET_ACCESS_KEY="minioadmin"
```

## Read-only Filesystem / Single Writable Root

If you run the buildpack in a read-only filesystem, provide the lifecycle and buildpack with a single writable root and keep all service data inside it.

### Contract for Operator

For a custom operator, this is the recommended strict-mode scheme. It provides much tighter control over layout and temporary paths than `pack`.

```text
/work
  /app
  /layers
  /platform
  /cache
  /launch-cache
  /tmp
  /home
```

Run lifecycle with `CNB_*` paths inside this root and a separate writable mount:

```bash
CNB_APP_DIR=/work/app \
CNB_LAYERS_DIR=/work/layers \
CNB_PLATFORM_DIR=/work/platform \
CNB_CACHE_DIR=/work/cache \
CNB_LAUNCH_CACHE_DIR=/work/launch-cache \
TMPDIR=/work/tmp \
TMP=/work/tmp \
TEMP=/work/tmp \
HOME=/work/home \
XDG_CACHE_HOME=/work/home/.cache \
UV_CACHE_DIR=/work/cache/uv \
PIP_CACHE_DIR=/work/cache/pip \
BP_MLFLOW_WORK_DIR=/work/layers/work
```

`BP_MLFLOW_WORK_DIR` sets the buildpack's internal scratch root. If not set, the buildpack uses `<layers>/work`.

### Best-effort for pack

`pack` can be used as a compatible mode, but with less strict path control than operator-controlled layout. In this case, mount a single writable root, pass it as `--workspace`, and explicitly set service variables:

```bash
pack build my-model-image \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --workspace /work/app \
  --volume "$PWD/.cnb-work:/work:rw" \
  --cache "type=build;format=bind;source=$PWD/.cnb-work/cache/build" \
  --cache "type=launch;format=bind;source=$PWD/.cnb-work/cache/launch" \
  --env TMPDIR=/work/tmp \
  --env TMP=/work/tmp \
  --env TEMP=/work/tmp \
  --env HOME=/work/home \
  --env XDG_CACHE_HOME=/work/home/.cache \
  --env UV_CACHE_DIR=/work/cache/uv \
  --env PIP_CACHE_DIR=/work/cache/pip \
  --env BP_MLFLOW_WORK_DIR=/work/layers/work
```

### Managed Variables

The buildpack and helper layers use these variables to redirect all service writes to writable root:

| Variable | Purpose |
|----------|---------|
| `TMPDIR` | Temporary files for `uv` and other tools |
| `TMP` | Additional Windows/Unix alias for temp |
| `TEMP` | Additional alias for temp |
| `HOME` | Home directory for tools that need `~` |
| `XDG_CACHE_HOME` | Base cache directory for XDG-aware tools |
| `UV_CACHE_DIR` | `uv` cache |
| `PIP_CACHE_DIR` | pip cache |
| `UV_PYTHON_INSTALL_DIR` | Python installation directory managed by `uv`; if manually overridden must match actual python layer path |
| `BP_MLFLOW_WORK_DIR` | Buildpack's internal scratch root |

---

## Configuration

### Buildpack Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `BP_MLFLOW_MODEL_PATH` | No | Path to model: `s3://bucket/path`, `/absolute/path`, or relative path within `--path` |
| `BP_MLFLOW_MODEL_NAME` | No | Model name (used for image labels, default: "model") |
| `BP_MLFLOW_MODEL_VERSION` | No | Model version (used for image labels, default: "latest") |
| `BP_MLFLOW_PREV_DEPS_HASH` | No | Previous dependency hash for cache optimization (orchestrators) |

**Model path detection priority:**
1. If `BP_MLFLOW_MODEL_PATH` starts with `s3://` → S3 storage
2. If `BP_MLFLOW_MODEL_PATH` is an absolute path (`/...`) → Local storage
3. If `BP_MLFLOW_MODEL_PATH` is a relative path → Relative to `--path`
4. `MLmodel` in root of `--path`
5. Recursive search for single `MLmodel` under `--path`

If multiple `MLmodel` files found, buildpack fails with ambiguity error and asks to specify `BP_MLFLOW_MODEL_PATH`.

### Model Configuration (conda.yaml)

The buildpack reads `conda.yaml` from the model to determine:

- Python version
- pip dependencies

Example:
```yaml
channels:
  - defaults
  - conda-forge
dependencies:
  - python=3.11
  - pip:
    - scikit-learn==1.3.0
    - pandas>=2.0.0
    - numpy>=1.24.0
```

If `conda.yaml` is missing, Python 3.10 is used by default.
If `requirements.txt` exists alongside, buildpack additionally installs dependencies from it (fallback mode).

---

## Inference

### HTTP API (V2 Protocol)

MLServer implements [V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md).

**Check readiness:**
```bash
curl http://localhost:8080/v2/health/ready
```

**Model info:**
```bash
curl http://localhost:8080/v2/models/model
```

**Inference:**
```bash
curl -X POST http://localhost:8080/v2/models/model/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-1",
    "inputs": [{
      "name": "input-0",
      "shape": [1, 4],
      "datatype": "FP32",
      "data": [[5.1, 3.5, 1.4, 0.2]]
    }]
  }'
```

**Response:**
```json
{
  "id": "test-1",
  "model_name": "model",
  "outputs": [{
    "name": "predict",
    "shape": [1, 1],
    "datatype": "INT64",
    "data": [0]
  }]
}
```

### gRPC API

```bash
# Install grpcurl
brew install grpcurl  # macOS

# List services
grpcurl -plaintext localhost:9080 list

# Inference
grpcurl -plaintext \
  -d '{"model_name": "model", "id": "test-1", "inputs": [{"name": "input-0", "shape": [1, 4], "datatype": "FP32", "contents": {"fp32_contents": [5.1, 3.5, 1.4, 0.2]}}]}' \
  localhost:9080 \
  inference.GRPCInferenceService/ModelInfer
```

### Running in Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mlflow-model
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mlflow-model
  template:
    metadata:
      labels:
        app: mlflow-model
    spec:
      containers:
        - name: model
          image: registry.example.com/models/sklearn:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 9080
              name: grpc
          env:
            - name: MLSERVER_PARALLEL_WORKERS
              value: "0"
          resources:
            requests:
              memory: "512Mi"
              cpu: "250m"
            limits:
              memory: "2Gi"
              cpu: "1"
---
apiVersion: v1
kind: Service
metadata:
  name: mlflow-model
spec:
  selector:
    app: mlflow-model
  ports:
    - port: 8080
      name: http
    - port: 9080
      name: grpc
```

---

## Buildpack Features

### Build Plan

The buildpack provides and requires `mlflow-model` in the build plan during detect phase:

```toml
[[provides]]
name = "mlflow-model"

[[requires]]
name = "mlflow-model"
```

This enables:
- **Standalone operation**: Self-contained requires/provides allows the buildpack to work independently
- **Extensibility**: Other buildpacks can depend on this one by requiring `mlflow-model`

### Layer Reuse (Layer Caching)

The buildpack implements intelligent layer caching to speed up builds:

#### Model UUID Caching (Local Models)

For local models, the buildpack reuses cached layers when the model hasn't changed. Change detection uses `model_uuid` from the `MLmodel` file:

```
Model unchanged (UUID: abc123...), reusing cached layers
```

#### Dependency Hash Caching (S3/Storage Models)

For models from S3 or storage paths, the buildpack uses a two-phase download with dependency hash comparison:

1. **Phase 1**: Download only metadata files (MLmodel, conda.yaml, requirements.txt)
2. **Phase 2**: Compute dependency hash and compare with cached hash
3. **Phase 3**: Skip rebuilding Python/venv layers if dependencies unchanged

```
Dependency hash: sha256:abc123...
Previous hash: sha256:abc123...
Dependencies unchanged, reusing cached Python and venv layers
```

This significantly speeds up rebuilds when only the model version changes but dependencies remain the same.

#### Cache Optimization for Orchestrators

Orchestrators like kpack can pass the previous dependency hash to optimize layer reuse:

```bash
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --env BP_MLFLOW_MODEL_PATH="s3://bucket/models/v2" \
  --env BP_MLFLOW_PREV_DEPS_HASH="sha256:abc123..."
```

To force a full rebuild:
- Delete the cached venv layer
- Change dependency versions in conda.yaml or requirements.txt
- Omit `BP_MLFLOW_PREV_DEPS_HASH`

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
docker inspect --format='{{json .Config.Labels}}' my-model:latest | jq
```

### SBOM

The buildpack generates CycloneDX Software Bill of Materials (SBOM):
- Python packages from virtual environment
- Model metadata

SBOM is attached to the image and can be retrieved with:

```bash
pack inspect my-model:latest --sbom
```

---

## Troubleshooting

### Buildpack doesn't detect model

**Symptom:** `Detect: fail`

**Solution:**
- Ensure `MLmodel` file exists in root of `--path`
- Or set `BP_MLFLOW_MODEL_NAME` environment variable
- Check for multiple `MLmodel` files (ambiguity)

### MLflow connection error

**Symptom:** `reading MLflow binding: MLflow binding not found`

**Solution:**
- Check bindings path: `--volume $(pwd)/bindings:/bindings/mlflow`
- Ensure `type` file contains `mlflow`
- Check contents: `cat bindings/mlflow/type`

### Artifact download error

**Symptom:** `downloading model: ...`

**Solution:**
- Check S3 credentials in `bindings/mlflow/s3/`
- Ensure endpoint is correct (with `https://`)
- Check bucket access

### Python version not found

**Symptom:** `installing Python: version X.Y not found`

**Solution:**
- Use supported Python version (3.9-3.12)
- Update `conda.yaml` in model

### Enable debug logging

```bash
pack build my-model \
  --builder ghcr.io/aagumin/mlserver-builder:latest \
  --path my-model \
  --env CNB_LOG_LEVEL=debug \
  -v
```

### Inspect image contents

```bash
# Run shell in image
docker run --rm -it --entrypoint /bin/bash my-model

# View layers
ls -la /layers/

# Check environment variables
env | grep MLSERVER
env | grep PYTHON
```

---

## Release Process

Creating a new tag triggers automatic image publishing to GitHub Container Registry (ghcr.io).

```bash
# Create and push tag
git tag v1.0.0
git push origin v1.0.0
```

### Published Images

| Image | Tags |
|-------|------|
| `ghcr.io/aagumin/fedora-mlserver-build` | `VERSION`, `latest` (stable only) |
| `ghcr.io/aagumin/fedora-mlserver-run` | `VERSION`, `latest` (stable only) |
| `ghcr.io/aagumin/mlserver-builder` | `VERSION`, `latest` (stable only) |

### Pre-release Versions

Tags like `v1.0.0-beta.1` or `v1.0.0-rc.2` are published without `latest` tag.

### Using Pre-built Builder

```bash
# Stable version
pack builder pull ghcr.io/aagumin/mlserver-builder:latest

# Specific version
pack builder pull ghcr.io/aagumin/mlserver-builder:1.0.0

# Pre-release
pack builder pull ghcr.io/aagumin/mlserver-builder:1.0.0-beta.1
```

---

## References

- [Cloud Native Buildpacks](https://buildpacks.io/)
- [MLServer Documentation](https://mlserver.readthedocs.io/)
- [MLflow Documentation](https://mlflow.org/docs/latest/index.html)
- [V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
