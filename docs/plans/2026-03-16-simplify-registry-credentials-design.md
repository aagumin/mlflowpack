# Simplify Registry Credentials: Design Document

## Problem

Current implementation has duplicate credential handling:
1. `bindings.go` reads MLflow/S3 credentials from bindings files or environment variables
2. `modctl` library (via databricks-sdk-go and aws-sdk-go-v2) reads the same credentials directly

The credentials read by bindings are never used - only checked for existence. This creates unnecessary complexity.

## Solution

Remove the `bindings/` package entirely. Let modctl handle all credential reading directly.

## Changes

### 1. Delete Files

| File | Action |
|------|--------|
| `buildpack/internal/bindings/bindings.go` | Delete |
| `buildpack/internal/bindings/bindings_test.go` | Delete |

### 2. Modify builder.go

Remove:
- Import of `bindings` package
- Credential validation code (lines 213-223)

The `getModel()` function will no longer validate credentials upfront. modctl will fail with clear error if credentials are missing.

### 3. Update go.mod

Run `go mod tidy` to remove unused dependencies.

### 4. Update Documentation

Simplify `docs/USAGE.md`:
- Remove "Service Bindings" sections
- Show minimal env var setup
- Remove volume mount examples

## User Experience After

```bash
# Before (complex):
mkdir -p bindings/mlflow/s3
echo "mlflow" > bindings/mlflow/type
echo "https://..." > bindings/mlflow/tracking_uri
pack build ... --volume $(pwd)/bindings:/bindings

# After (simple):
pack build my-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-model/1" \
  --env MLFLOW_TRACKING_URI="https://mlflow.company.com" \
  --env MLFLOW_TRACKING_USERNAME="user" \
  --env MLFLOW_TRACKING_PASSWORD="pass"
```

## Supported Credentials

MLflow (via databricks-sdk-go):
- `MLFLOW_TRACKING_URI`, `MLFLOW_TRACKING_USERNAME`, `MLFLOW_TRACKING_PASSWORD`
- `DATABRICKS_HOST`, `DATABRICKS_TOKEN`

S3 (via aws-sdk-go-v2):
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `AWS_ENDPOINT_URL`
- `~/.aws/credentials` and `~/.aws/config` (if available in build environment)
