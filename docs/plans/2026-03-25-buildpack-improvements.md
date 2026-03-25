# Buildpack Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add CNB best practices to MLflow buildpack: Build Plan, layer reuse, image labels, and error handling fix.

**Architecture:** Minimal changes to existing codebase - add types, helper functions, and integrate into detect/build phases. Layer reuse checks model_uuid from MLmodel file against cached metadata.

**Tech Stack:** Go, Cloud Native Buildpacks API 0.12, TOML, CycloneDX SBOM

---

### Task 1: Add BuildPlan Types

**Files:**
- Modify: `buildpack/internal/cnb/types.go:1-78`

**Step 1: Add BuildPlan types to types.go**

Add after `DetectResult` struct (around line 70):

```go
// BuildPlan represents the build plan written during detect.
type BuildPlan struct {
	Provides []BuildPlanEntry `toml:"provides"`
}

// BuildPlanEntry represents a single provides entry.
type BuildPlanEntry struct {
	Name string `toml:"name"`
}
```

**Step 2: Run tests to verify no breakage**

Run: `cd buildpack && go build ./...`
Expected: Success, no errors

**Step 3: Commit**

```bash
git add buildpack/internal/cnb/types.go
git commit -m "feat(cnb): add BuildPlan types for detect phase"
```

---

### Task 2: Add WriteBuildPlan Function

**Files:**
- Modify: `buildpack/internal/cnb/toml.go:1-67`

**Step 1: Add WriteBuildPlan function to toml.go**

Add after `ReadLayerToml` function (around line 66):

```go
// WriteBuildPlan writes the build plan to the specified path.
func WriteBuildPlan(path string, plan BuildPlan) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing %q: %w", path, closeErr))
		}
	}()

	return toml.NewEncoder(f).Encode(plan)
}
```

**Step 2: Run tests to verify no breakage**

Run: `cd buildpack && go build ./...`
Expected: Success, no errors

**Step 3: Commit**

```bash
git add buildpack/internal/cnb/toml.go
git commit -m "feat(cnb): add WriteBuildPlan function"
```

---

### Task 3: Write Build Plan in Detect Phase

**Files:**
- Modify: `buildpack/internal/detect/detector.go:1-113`

**Step 1: Update Detect function to write build plan**

Replace the `Detect` function (lines 27-48) with:

```go
// Detect checks if an MLmodel file exists in the application directory.
// If found, it returns a build plan that provides "mlflow-model".
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	if _, _, ok, err := DetectFromModelPathEnv(); err != nil {
		return cnb.DetectResult{}, err
	} else if ok {
		return writePlanAndPass(ctx)
	}

	if _, err := FindLocalModelDir(ctx.AppDir); err != nil {
		// Not a local model build; registry mode may still apply.
		if errors.Is(err, ErrLocalModelNotFound) {
			if _, _, ok, err := DetectFromEnv(); err != nil {
				return cnb.DetectResult{}, err
			} else if !ok {
				return cnb.DetectResult{Pass: false}, nil
			}
		} else {
			return cnb.DetectResult{}, err
		}
	}

	return writePlanAndPass(ctx)
}

// writePlanAndPass writes the build plan and returns a passing result.
func writePlanAndPass(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	plan := cnb.BuildPlan{
		Provides: []cnb.BuildPlanEntry{
			{Name: "mlflow-model"},
		},
	}

	if err := cnb.WriteBuildPlan(ctx.BuildPlanPath, plan); err != nil {
		return cnb.DetectResult{}, fmt.Errorf("writing build plan: %w", err)
	}

	return cnb.DetectResult{Pass: true}, nil
}
```

**Step 2: Run tests**

Run: `cd buildpack && go test -v ./internal/detect/...`
Expected: All tests pass

**Step 3: Commit**

```bash
git add buildpack/internal/detect/detector.go
git commit -m "feat(detect): write build plan with mlflow-model provide"
```

---

### Task 4: Add UUID Method to Model

**Files:**
- Modify: `buildpack/internal/mlflow/model.go`

**Step 1: Read current model.go to understand structure**

Run: Read the file first to see existing methods

**Step 2: Add UUID method**

Find the `Model` struct and its methods. Add after existing methods:

```go
// UUID returns the model UUID from the MLmodel file.
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

Note: If `mlmodel` field is not accessible, adjust based on actual struct definition.

**Step 3: Run tests**

Run: `cd buildpack && go build ./...`
Expected: Success, no errors

**Step 4: Commit**

```bash
git add buildpack/internal/mlflow/model.go
git commit -m "feat(mlflow): add UUID method to Model for layer reuse"
```

---

### Task 5: Implement Layer Reuse Logic

**Files:**
- Modify: `buildpack/internal/build/builder.go:35-170`

**Step 1: Add helper function to check cached layers**

Add before the `Build` function:

```go
// cachedModelUUID returns the model UUID from cached model layer metadata.
func cachedModelUUID(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.ModelLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	uuid, _ := meta.Metadata["model_uuid"].(string)
	return uuid
}
```

**Step 2: Add layer reuse check in Build function**

After parsing MLmodel (around line 62), add check:

```go
// Check for cached layers with same model UUID
cachedUUID := cachedModelUUID(ctx.LayersDir)
currentUUID := model.UUID()

if cachedUUID != "" && cachedUUID == currentUUID {
	fmt.Printf("Model unchanged (UUID: %s), reusing cached layers\n", currentUUID)

	// Return cached layer metadata
	result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}
	result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{Types: layer.DefaultVenvLayerTypes()}
	result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{Types: layer.DefaultModelLayerTypes()}

	// Add process for cached venv
	result.Launch.Processes = append(result.Launch.Processes, cnb.ProcessEntry{
		Type:    "web",
		Command: []string{filepath.Join(ctx.LayersDir, layer.VenvLayerName, "bin", "mlserver"), "start", filepath.Join(ctx.LayersDir, layer.ModelLayerName)},
		Default: true,
	})

	return result, nil
}
```

**Step 3: Update model layer metadata to include UUID**

Find where model layer metadata is set (around line 98) and update:

```go
result.Layers[layer.ModelLayerName] = cnb.LayerMetadata{
	Types: layer.DefaultModelLayerTypes(),
	Metadata: map[string]interface{}{
		"model_uuid":     model.UUID(),
		"flavor":         flavor,
		"python_version": pythonVersion,
	},
}
```

**Step 4: Run tests**

Run: `cd buildpack && go test -v ./internal/build/...`
Expected: All tests pass

**Step 5: Commit**

```bash
git add buildpack/internal/build/builder.go
git commit -m "feat(build): add layer reuse based on model_uuid"
```

---

### Task 6: Add Image Labels

**Files:**
- Modify: `buildpack/internal/build/builder.go`

**Step 1: Add labels before return in Build function**

Before the final `return result, nil` (around line 169), add:

```go
// Add image labels
result.Launch.Labels = []cnb.Label{
	{Key: "org.opencontainers.image.title", Value: modelSource.Name},
	{Key: "org.opencontainers.image.version", Value: modelSource.Version},
	{Key: "org.opencontainers.image.description", Value: "MLflow model served by MLServer"},
	{Key: "io.github.aagumin.model-flavor", Value: flavor},
	{Key: "io.github.aagumin.model-name", Value: modelSource.Name},
	{Key: "io.github.aagumin.mlserver-runtime", Value: mlserverExt.Runtime},
}
```

**Step 2: Run tests**

Run: `cd buildpack && go test -v ./internal/build/...`
Expected: All tests pass

**Step 3: Commit**

```bash
git add buildpack/internal/build/builder.go
git commit -m "feat(build): add OCI and MLflow-specific image labels"
```

---

### Task 7: Fix file.Close() in SBOM Writer

**Files:**
- Modify: `buildpack/internal/sbom/writer.go:73-88`

**Step 1: Fix writeJSON function**

Replace the `writeJSON` function with:

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

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding SBOM JSON: %w", err)
	}

	return nil
}
```

**Step 2: Add errors import if not present**

Check imports at top of file. If `errors` is not imported, add:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)
```

**Step 3: Run tests**

Run: `cd buildpack && go test -v ./internal/sbom/...`
Expected: All tests pass

**Step 4: Commit**

```bash
git add buildpack/internal/sbom/writer.go
git commit -m "fix(sbom): handle file.Close() return value properly"
```

---

### Task 8: Full Build and Test

**Step 1: Run full test suite**

Run: `cd buildpack && go test -v -race ./...`
Expected: All tests pass

**Step 2: Build the buildpack**

Run: `make build`
Expected: Binaries created successfully

**Step 3: Rebuild builder and test**

Run: `make builder`
Expected: Builder created successfully

**Step 4: Test with a model**

Run: `lima pack build test-model:latest --builder localhost:5000/aagumin/mlserver-builder:$(git describe --tags --always) --path e2e/models/sklearn --trust-builder`
Expected: Build succeeds with new labels

**Step 5: Verify labels**

Run: `lima docker inspect --format='{{json .Config.Labels}}' test-model:latest`
Expected: JSON with all 6 labels

**Step 6: Test layer reuse**

Run pack build again with same model
Expected: "Model unchanged (UUID: ...), reusing cached layers" message

**Step 7: Commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address test failures from buildpack improvements"
```

---

### Task 9: Update Documentation

**Files:**
- Modify: `docs/plans/2026-03-24-sbom-implementation.md` or create new doc if needed

**Step 1: Document new features**

Add note about:
- Build plan provides "mlflow-model"
- Layer reuse when model_uuid unchanged
- Available image labels

**Step 2: Commit**

```bash
git add docs/
git commit -m "docs: document buildpack improvements"
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Add BuildPlan types | `internal/cnb/types.go` |
| 2 | Add WriteBuildPlan | `internal/cnb/toml.go` |
| 3 | Write plan in detect | `internal/detect/detector.go` |
| 4 | Add Model.UUID() | `internal/mlflow/model.go` |
| 5 | Layer reuse logic | `internal/build/builder.go` |
| 6 | Add image labels | `internal/build/builder.go` |
| 7 | Fix file.Close() | `internal/sbom/writer.go` |
| 8 | Full build and test | All |
| 9 | Update docs | `docs/` |
