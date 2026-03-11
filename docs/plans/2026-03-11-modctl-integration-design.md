# Design: modctl MLflow Provider Integration

**Date:** 2026-03-11
**Status:** Approved
**Author:** Claude

## Overview

Integrate modctl's MLflow provider to simplify local development and add environment variable support for credentials.

## Goals

1. Import `github.com/modelpack/modctl/pkg/modelprovider/mlflow` for model downloading
2. Change URL format from `models://` to `models:/`
3. Add environment variables as fallback for credentials (CNB bindings remain primary)
4. Remove duplicate MLflow client and S3 storage code

## Non-Goals

- Full replacement of all modctl providers (only MLflow)
- Changes to the build phase architecture
- Breaking changes to existing Kubernetes deployments with bindings

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Build Phase                            │
├─────────────────────────────────────────────────────────────┤
│  determineModelSource()                                     │
│       │                                                     │
│       ▼                                                     │
│  DetectFromModelPathEnv()  ◄── "models:/name/version"      │
│       │                                                     │
│       ▼                                                     │
│  getModel()                                                 │
│       │                                                     │
│       ├─────► bindings.Reader (primary)                     │
│       │            │                                        │
│       │            └── ReadMLflowBinding()                  │
│       │            └── ReadS3Binding()                      │
│       │                                                     │
│       ├─────► env vars fallback (NEW)                       │
│       │            │                                        │
│       │            └── MLFLOW_TRACKING_URI                  │
│       │            └── MLFLOW_TRACKING_USERNAME             │
│       │            └── MLFLOW_TRACKING_PASSWORD             │
│       │            └── AWS_ACCESS_KEY_ID                    │
│       │            └── AWS_SECRET_ACCESS_KEY                │
│       │                                                     │
│       ▼                                                     │
│  modctl.MlFlowClient.PullModelByName()  ◄── NEW            │
│       │                                                     │
│       ▼                                                     │
│  mlflow.Model (unchanged)                                   │
└─────────────────────────────────────────────────────────────┘
```

## File Changes

### Modified Files

| File | Changes |
|------|---------|
| `go.mod` | Add modctl dependency |
| `detect/detector.go` | Change `models://` to `models:/` |
| `bindings/bindings.go` | Add env vars fallback functions |
| `build/builder.go` | Use new modctl-based downloader |

### New Files

| File | Purpose |
|------|---------|
| `mlflow/downloader.go` | Wrapper for modctl MLflow client |

### Deleted Files

| File | Reason |
|------|--------|
| `mlflow/client.go` | Replaced by modctl |
| `mlflow/storage/s3.go` | Replaced by modctl |
| `mlflow/storage/` | Entire package replaced |

## Environment Variables

### MLflow Registry

| Variable | Description | Required |
|----------|-------------|----------|
| `MLFLOW_TRACKING_URI` | MLflow server URL | Yes (or DATABRICKS_HOST) |
| `DATABRICKS_HOST` | Databricks workspace URL | Alternative to MLFLOW_TRACKING_URI |
| `MLFLOW_TRACKING_USERNAME` | Basic auth username | No |
| `MLFLOW_TRACKING_PASSWORD` | Basic auth password | No |

### S3 Storage

| Variable | Description | Required |
|----------|-------------|----------|
| `AWS_ACCESS_KEY_ID` | S3 access key | Yes |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key | Yes |
| `AWS_REGION` | AWS region | No (default: us-east-1) |
| `AWS_ENDPOINT_URL` | Custom S3 endpoint | No (for MinIO, etc.) |

## Credential Priority

1. **CNB Bindings** (primary) - For Kubernetes deployments
2. **Environment Variables** (fallback) - For local development

## Usage Examples

### Before (bindings only)

```bash
mkdir -p bindings/mlflow/s3
echo "https://mlflow.company.com" > bindings/mlflow/tracking_uri
echo "user" > bindings/mlflow/username
echo "pass" > bindings/mlflow/password
echo "https://s3.company.com" > bindings/mlflow/s3/endpoint
echo "access-key" > bindings/mlflow/s3/access_key
echo "secret-key" > bindings/mlflow/s3/secret_key

pack build my-model \
  --volume ./bindings:/bindings \
  --env BP_MLFLOW_MODEL_PATH="models:/my-model/1"
```

### After (env vars)

```bash
export MLFLOW_TRACKING_URI="https://mlflow.company.com"
export MLFLOW_TRACKING_USERNAME="user"
export MLFLOW_TRACKING_PASSWORD="pass"
export AWS_ACCESS_KEY_ID="access-key"
export AWS_SECRET_ACCESS_KEY="secret-key"
export AWS_ENDPOINT_URL="https://s3.company.com"

pack build my-model \
  --env BP_MLFLOW_MODEL_PATH="models:/my-model/1"
```

## Testing Strategy

1. **Unit Tests**
   - `ReadMLflowBindingWithFallback()` with various env var combinations
   - URL parsing for `models:/` format

2. **Integration Tests**
   - Build with env vars only (no bindings)
   - Build with bindings (env vars ignored)

3. **E2E Tests**
   - Full build cycle with real MLflow registry
   - S3 artifact download verification

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| modctl dependency breaks | Pin version in go.mod |
| Databricks SDK adds overhead | Only import mlflow package |
| Breaking change for `models://` users | Add deprecation warning, support both temporarily |

## Rollout Plan

1. Add modctl dependency and downloader wrapper
2. Add env vars fallback to bindings
3. Change URL format (with backwards compatibility)
4. Update documentation
5. Remove old code after validation
