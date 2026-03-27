# Design: Layer Caching Optimization for Model Updates

**Date**: 2026-03-27
**Status**: Approved
**Author**: Design session

## Overview

Optimize buildpack layer caching to avoid rebuilding Python and venv layers when only the model version changes but dependencies remain the same. This significantly reduces build time for frequent model updates.

## Goals

1. Skip Python layer rebuild when Python version unchanged
2. Skip venv layer rebuild when dependencies (conda.yaml + requirements.txt) unchanged
3. Only rebuild model layer when model version changes
4. Remove MLflow API dependency from buildpack - accept only S3/local paths
5. Support two-phase download: metadata first, then artifacts

## Non-Goals

- MLflow Model Registry API integration (moved to external operator)
- Automatic detection of dependency changes via MLflow API

## Architecture

### Layer Structure and Metadata

```
python/          # Type: build, launch
  Metadata:
    python_version: "3.11"

venv/            # Type: build, launch, cache
  Metadata:
    deps_hash: "sha256:abc123..."     # Hash of conda.yaml + requirements.txt
    deps_files: ["conda.yaml"]        # Which files were used

model/           # Type: launch
  Metadata:
    model_uuid: "..."                 # As currently
    model_name: "my-model"
    model_version: "5"
```

### Caching Logic

| Layer | Cache Key | Reuse Condition |
|-------|-----------|-----------------|
| python | `python_version` | Version matches cached |
| venv | `deps_hash` | Hash of dependencies matches |
| model | `model_uuid` | UUID matches (as currently) |

### Build Flow with Two-Phase Download

```
┌─────────────────────────────────────────────────────────────┐
│  PHASE 1: Metadata                                          │
├─────────────────────────────────────────────────────────────┤
│  1. Download only MLmodel, conda.yaml, requirements.txt     │
│  2. Compute deps_hash                                       │
│  3. Read prev_deps_hash (from env or venv layer metadata)   │
│  4. Read prev_python_version (from python layer metadata)   │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│  DECISION: Dependencies changed?                            │
└─────────────────────────────────────────────────────────────┘
           ↓                                    ↓
    [YES: Full rebuild]               [NO: Model only]
           ↓                                    ↓
┌──────────────────────┐          ┌──────────────────────────┐
│ python: install      │          │ python: REUSE (by SHA256)│
│ venv: install        │          │ venv: REUSE (by SHA256)  │
│ model: download      │          │ model: download          │
└──────────────────────┘          └──────────────────────────┘
```

CNB lifecycle handles reuse automatically - buildpack returns `Launch: true, Build: false, Cache: true` for unchanged layers. Layer blobs remain in registry, referenced by digest.

## Simplification: Remove MLflow API

### Current State
- Buildpack uses `modctl` to download models from MLflow Registry
- Requires MLFLOW_TRACKING_URI, credentials, etc.

### New State
- Buildpack accepts only **model path** (`BP_MLFLOW_MODEL_PATH`)
- Path can be:
  - `s3://bucket/path/to/model` - download from S3
  - `/path/to/model` - use locally
  - `file:///path/to/model` - local path
- **No MLflow API** - all registry logic is external

### Operator/CI Responsibility
```yaml
# Operator does:
1. Request artifact path from MLflow (s3://...)
2. Pass path to buildpack: BP_MLFLOW_MODEL_PATH=s3://...
3. Pass S3 credentials via bindings/env
```

## Environment Variables

### Removed (MLflow API)
- ~~`BP_MLFLOW_MODEL_NAME`~~ (for registry download)
- ~~`BP_MLFLOW_MODEL_VERSION`~~ (for registry download)
- ~~`BP_MLFLOW_MODEL_STAGE`~~

### New
| Variable | Description | Example |
|----------|-------------|---------|
| `BP_MLFLOW_MODEL_PATH` | Path to model (required) | `s3://bucket/models/v1` or `/workspace/model` |
| `BP_MLFLOW_PREV_DEPS_HASH` | Dependencies hash from previous build | `sha256:abc123...` |
| `BP_MLFLOW_MODEL_NAME` | Model name (for labels only) | `my-model` |
| `BP_MLFLOW_MODEL_VERSION` | Version (for labels only) | `5` |

### S3 Credentials (standard)
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`
- Or via Service Binding `/bindings/s3/`

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `BP_MLFLOW_MODEL_PATH` not specified | Fail in detect phase |
| Path does not exist | Fail in build phase |
| S3 bucket unavailable | Fail with clear message |
| No `BP_MLFLOW_PREV_DEPS_HASH` and no cache | Full rebuild (expected) |
| Hash mismatch | Rebuild venv (expected) |

## File Structure Changes

```
buildpack/
├── internal/
│   ├── storage/           # NEW: S3/local storage client
│   │   ├── storage.go     # Storage interface
│   │   ├── s3.go          # S3 implementation
│   │   ├── local.go       # Local filesystem
│   │   └── metadata.go    # Two-phase download
│   ├── build/
│   │   └── builder.go     # CHANGED: new layer logic
│   ├── detect/
│   │   └── detector.go    # CHANGED: path-only detection
│   └── deps/
│       └── hash.go        # NEW: dependency hash computation
```

## Implementation Order

```
1. [REFACTOR] Remove MLflow API → keep only S3/local paths
2. [FEATURE] Two-phase download from S3 (metadata → artifacts)
3. [FEATURE] Layer caching by hashes (python, venv)
4. [FEATURE] External hash via BP_MLFLOW_PREV_DEPS_HASH
```

## Testing

| Test Type | Scenario |
|-----------|----------|
| **Unit** | Compute `deps_hash` from files |
| **Unit** | Hash comparison, reuse decision |
| **Unit** | Parse `BP_MLFLOW_MODEL_PATH` (s3://, file://, /local) |
| **Integration** | Two-phase download from S3 (minio in tests) |
| **Integration** | Reuse python layer with same version |
| **Integration** | Reuse venv layer with same `deps_hash` |
| **E2E** | Full rebuild → model change → partial rebuild (model only) |

**Commands**:
```bash
make test          # Unit + integration
make test-e2e      # With real S3/minio
```

## Open Questions

None - design approved.

## References

- CNB Layer Caching: https://buildpacks.io/docs/concepts/operations/lifecycle/
- Service Bindings: https://servicebinding.io/
