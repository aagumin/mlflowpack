# Buildpack Improvements Design

Date: 2026-03-25

## Overview

Four improvements to align MLflow Model Buildpack with Cloud Native Buildpacks best practices:
1. Build Plan in detect phase
2. Layer reuse based on model_uuid
3. OCI and MLflow-specific labels
4. Fix file.Close() error handling

---

## 1. Build Plan in Detect Phase

### Goal
Declare what the buildpack provides for integration with other buildpacks.

### Implementation

**File: `internal/cnb/types.go`**
```go
// BuildPlan represents the build plan written during detect.
type BuildPlan struct {
    Provides []BuildPlanEntry `toml:"provides"`
}

// BuildPlanEntry represents a single provides/requires entry.
type BuildPlanEntry struct {
    Name string `toml:"name"`
}
```

**File: `internal/cnb/toml.go`**
```go
// WriteBuildPlan writes the build plan to the specified path.
func WriteBuildPlan(path string, plan BuildPlan) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()

    return toml.NewEncoder(f).Encode(plan)
}
```

**File: `internal/detect/detector.go`**
- Call `cnb.WriteBuildPlan(ctx.BuildPlanPath, ...)` when detection passes
- Provide "mlflow-model"

### Output Example
```toml
[[provides]]
name = "mlflow-model"
```

---

## 2. Layer Reuse Based on model_uuid

### Goal
Skip rebuilding venv layer when model hasn't changed.

### Logic

1. Before creating layers, check cached `model.toml` for `model_uuid`
2. After parsing MLmodel, compare current `model_uuid` with cached
3. If match: skip venv installation, reuse cached layers
4. If mismatch or no cache: rebuild all layers
5. Save new `model_uuid` in model layer metadata

### Implementation

**File: `internal/mlflow/model.go`**
```go
// UUID returns the model UUID from MLmodel file.
func (m *Model) UUID() string {
    if m.mlmodel == nil {
        return ""
    }
    if uuid, ok := m.mlmodel["model_uuid"].(string); ok {
        return uuid
    }
    return ""
}
```

**File: `internal/build/builder.go`**
```go
// Check cached model UUID
cachedMeta, _ := cnb.ReadLayerToml(ctx.LayersDir, layer.ModelLayerName)
cachedUUID, _ := cachedMeta.Metadata["model_uuid"].(string)

// After parsing MLmodel
currentUUID := model.UUID()
reuseLayers := cachedUUID != "" && cachedUUID == currentUUID

if reuseLayers {
    fmt.Printf("Model unchanged (UUID: %s), reusing cached layers\n", currentUUID)
    // Skip venv rebuild, return existing layer metadata
}
```

**File: `internal/build/builder.go` — Save metadata**
```go
result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{
    Types: layer.DefaultModelLayerTypes(),
    Metadata: map[string]interface{}{
        "model_uuid":      model.UUID(),
        "python_version":  pythonVersion,
        "flavor":          flavor,
    },
}
```

---

## 3. OCI and MLflow-Specific Labels

### Goal
Add image labels for observability and debugging.

### Labels

| Label | Source | Example |
|-------|--------|---------|
| `org.opencontainers.image.title` | modelSource.Name | `sklearn-iris` |
| `org.opencontainers.image.version` | modelSource.Version | `1` or `latest` |
| `org.opencontainers.image.description` | static | `MLflow model served by MLServer` |
| `io.github.aagumin.model-flavor` | model.GetPrimaryFlavor() | `sklearn` |
| `io.github.aagumin.model-name` | modelSource.Name | `sklearn-iris` |
| `io.github.aagumin.mlserver-runtime` | mlserverExt.Runtime | `mlserver_sklearn.MLServerRuntime` |

### Implementation

**File: `internal/build/builder.go`**
```go
result.Launch.Labels = []cnb.Label{
    {Key: "org.opencontainers.image.title", Value: modelSource.Name},
    {Key: "org.opencontainers.image.version", Value: modelSource.Version},
    {Key: "org.opencontainers.image.description", Value: "MLflow model served by MLServer"},
    {Key: "io.github.aagumin.model-flavor", Value: flavor},
    {Key: "io.github.aagumin.model-name", Value: modelSource.Name},
    {Key: "io.github.aagumin.mlserver-runtime", Value: mlserverExt.Runtime},
}
```

---

## 4. Fix file.Close() Error Handling

### Goal
Properly handle file.Close() return value in sbom/writer.go.

### Current Code
```go
func writeJSON(path string, data interface{}) error {
    file, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("creating SBOM file: %w", err)
    }
    defer file.Close()  // Return value ignored
    // ...
}
```

### Fixed Code
```go
func writeJSON(path string, data interface{}) (err error) {
    file, err := os.Create(path)
    if err != nil {
        return fmt.Errorf("creating SBOM file: %w", err)
    }
    defer func() {
        if closeErr := file.Close(); closeErr != nil {
            err = errors.Join(err, fmt.Errorf("closing %q: %w", path, closeErr))
        }
    }()
    // ...
}
```

---

## Files Changed

| File | Change |
|------|--------|
| `internal/cnb/types.go` | Add BuildPlan types |
| `internal/cnb/toml.go` | Add WriteBuildPlan() |
| `internal/detect/detector.go` | Write build plan on pass |
| `internal/mlflow/model.go` | Add UUID() method |
| `internal/build/builder.go` | Layer reuse logic + labels |
| `internal/sbom/writer.go` | Fix file.Close() handling |

---

## Testing

1. **Build Plan**: Run detect phase, verify build plan file content
2. **Layer Reuse**: Build twice with same model, verify venv not rebuilt second time
3. **Labels**: Build image, run `docker inspect --format='{{json .Config.Labels}}'`
4. **file.Close()**: Existing tests should pass
