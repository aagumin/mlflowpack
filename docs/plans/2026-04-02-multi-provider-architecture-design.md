# Multi-Provider Buildpack Architecture

Date: 2026-04-02

## Problem

The buildpack is tightly coupled to MLflow — detect and build logic lives in `internal/detect/` and `internal/build/` with MLflow-specific code mixed in. Adding support for other model registries (ClearML, etc.) requires duplicating or hacking this structure.

## Decision

Refactor to a **provider interface** pattern. One Go module, one buildpack (`io.github.aagumin.mlflow-model`), but internally routed through a provider registry. Adding a new provider means adding a package under `internal/` and registering it via `init()`.

## Architecture

### Provider Interface

```go
// internal/provider/provider.go
type Provider interface {
    Name() string
    Detect(ctx cnb.DetectContext) (cnb.DetectResult, error)
    Build(ctx cnb.BuildContext) (cnb.BuildResult, error)
}
```

Global registry with `Register()`, `All()`, `ByName()`, `DetectFirst()`.

### Package Reorganization

| Current | After | Notes |
|---------|-------|-------|
| `internal/detect/detector.go` | `internal/mlflow/detect.go` | MLflow detect |
| `internal/detect/local_model.go` | `internal/mlflow/local_model.go` | MLmodel search |
| `internal/build/builder.go` | `internal/mlflow/build.go` | MLflow build |
| `internal/build/cache.go` | `internal/mlflow/cache.go` | deps hash |
| `internal/build/workdir.go` | `internal/mlflow/workdir.go` | work dir |
| `internal/build/cache_test.go` | `internal/mlflow/cache_test.go` | |
| `internal/build/*_test.go` | `internal/mlflow/*_test.go` | |
| `internal/detect/detector_test.go` | `internal/mlflow/detect_test.go` | |
| `internal/mlflow/*` | `internal/mlflow/*` | unchanged |
| — | `internal/provider/provider.go` | new |
| — | `internal/mlflow/provider.go` | new, registers via init() |
| `internal/python/` | `internal/python/` | shared, unchanged |
| `internal/storage/` | `internal/storage/` | shared, unchanged |
| `internal/conda/` | `internal/conda/` | shared, unchanged |
| `internal/cnb/` | `internal/cnb/` | shared, unchanged |
| `internal/layer/` | `internal/layer/` | shared, unchanged |
| `internal/sbom/` | `internal/sbom/` | shared, unchanged |
| `internal/deps/` | `internal/deps/` | shared, unchanged |

`internal/detect/` and `internal/build/` are deleted entirely.

### cmd/ Routing

**cmd/detect/main.go** — thin router:
- Imports providers via `_ "internal/mlflow"` (triggers init() registration)
- Calls `provider.DetectFirst(ctx)` — iterates registered providers
- First provider that returns `pass` wins
- Writes build plan with `provider` name in metadata
- Build plan entry renamed from `"mlflow-model"` to `"model"` (provider-agnostic)

**cmd/build/main.go** — reads provider from plan:
- Reads build plan, extracts `provider` name from metadata
- Calls `provider.ByName(name).Build(ctx)`

### Auto-Detection Logic

Providers self-detect via env vars and files:
- MLflow: `BP_MLFLOW_MODEL_PATH` env var or `MLmodel` file presence
- ClearML (future): `BP_CLEARML_*` env vars or ClearML-specific files

No explicit `BP_MODEL_PROVIDER` env var needed. Detection order = registration order.

## Backward Compatibility

All external interfaces unchanged:
- `BP_MLFLOW_MODEL_PATH` env var
- Layer structure (python, venv, model)
- `buildpack.toml` id, version, homepage
- `package.toml`, `builder.toml.template`
- Stack images
- Makefile targets (`make build`, `test`, `lint`, `package`, `builder`)
- Image labels, process types, model-settings.json format

## Scope

- ~12 files move from `internal/detect/` + `internal/build/` to `internal/mlflow/`
- 2 new files: `internal/provider/provider.go`, `internal/mlflow/provider.go`
- 2 files rewritten: `cmd/detect/main.go`, `cmd/build/main.go`
- `internal/detect/` and `internal/build/` directories deleted
