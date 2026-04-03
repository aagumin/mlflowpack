# Multi-Provider Architecture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the buildpack to use a provider interface pattern so multiple model registry providers (MLflow, ClearML, etc.) can coexist in a single builder.

**Architecture:** Introduce `internal/provider/` package with a `Provider` interface. Move all MLflow-specific detect/build logic from `internal/detect/` and `internal/build/` into `internal/mlflow/`. The `cmd/detect` and `cmd/build` become thin routers that delegate to registered providers. Build plan carries provider name in metadata.

**Tech Stack:** Go 1.x, CNB Buildpack API 0.12, TOML (BurntSushi/toml)

---

### Task 1: Add Metadata field to BuildPlanEntry

The current `cnb.BuildPlanEntry` has only a `Name` field. We need `Metadata` to pass provider name from detect to build.

**Files:**
- Modify: `buildpack/internal/cnb/types.go:79-81`
- Test: `buildpack/internal/cnb/types_test.go` (new)

**Step 1: Write the failing test**

Create `buildpack/internal/cnb/types_test.go`:

```go
package cnb

import (
	"testing"
)

func TestBuildPlanEntry_Metadata(t *testing.T) {
	entry := BuildPlanEntry{
		Name:     "model",
		Metadata: map[string]interface{}{"provider": "mlflow"},
	}

	meta, ok := entry.Metadata.(map[string]interface{})
	if !ok {
		t.Fatalf("Metadata type = %T, want map[string]interface{}", entry.Metadata)
	}

	if meta["provider"] != "mlflow" {
		t.Fatalf("Metadata[provider] = %v, want mlflow", meta["provider"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/cnb/ -run TestBuildPlanEntry_Metadata -v`
Expected: FAIL — `BuildPlanEntry` has no `Metadata` field

**Step 3: Write minimal implementation**

Modify `buildpack/internal/cnb/types.go` — change `BuildPlanEntry`:

```go
// BuildPlanEntry represents a single provides/requires entry.
type BuildPlanEntry struct {
	Name     string                 `toml:"name"`
	Metadata map[string]interface{} `toml:"metadata,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/cnb/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/cnb/types.go buildpack/internal/cnb/types_test.go
git commit -m "feat(cnb): add Metadata field to BuildPlanEntry"
```

---

### Task 2: Add ReadBuildPlan to cnb package

The build phase needs to read the build plan written by detect to extract the provider name. Currently only `WriteBuildPlan` exists in `cnb/toml.go`.

**Files:**
- Modify: `buildpack/internal/cnb/toml.go`
- Test: `buildpack/internal/cnb/toml_test.go` (new)

**Step 1: Write the failing test**

Create `buildpack/internal/cnb/toml_test.go`:

```go
package cnb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadBuildPlan(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.toml")

	original := BuildPlan{
		Provides: []BuildPlanEntry{
			{Name: "model"},
		},
		Requires: []BuildPlanEntry{
			{Name: "model", Metadata: map[string]interface{}{"provider": "mlflow"}},
		},
	}

	if err := WriteBuildPlan(planPath, original); err != nil {
		t.Fatalf("WriteBuildPlan() error = %v", err)
	}

	read, err := ReadBuildPlan(planPath)
	if err != nil {
		t.Fatalf("ReadBuildPlan() error = %v", err)
	}

	if len(read.Requires) != 1 {
		t.Fatalf("len(Requires) = %d, want 1", len(read.Requires))
	}

	if read.Requires[0].Name != "model" {
		t.Fatalf("Requires[0].Name = %q, want %q", read.Requires[0].Name, "model")
	}

	meta, ok := read.Requires[0].Metadata["provider"].(string)
	if !ok || meta != "mlflow" {
		t.Fatalf("Requires[0].Metadata[provider] = %v, want %q", read.Requires[0].Metadata["provider"], "mlflow")
	}
}

func TestReadBuildPlan_FileNotFound(t *testing.T) {
	_, err := ReadBuildPlan("/nonexistent/plan.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/cnb/ -run TestWriteAndReadBuildPlan -v`
Expected: FAIL — `ReadBuildPlan` undefined

**Step 3: Write minimal implementation**

Add to `buildpack/internal/cnb/toml.go`:

```go
// ReadBuildPlan reads a build plan from the specified path.
func ReadBuildPlan(path string) (BuildPlan, error) {
	var plan BuildPlan
	_, err := toml.DecodeFile(path, &plan)
	if err != nil {
		return BuildPlan{}, fmt.Errorf("reading build plan: %w", err)
	}
	return plan, nil
}
```

Ensure `fmt` is in imports.

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/cnb/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/cnb/toml.go buildpack/internal/cnb/toml_test.go
git commit -m "feat(cnb): add ReadBuildPlan function"
```

---

### Task 3: Create provider package

New package with the Provider interface and registry.

**Files:**
- Create: `buildpack/internal/provider/provider.go`
- Test: `buildpack/internal/provider/provider_test.go`

**Step 1: Write the failing tests**

Create `buildpack/internal/provider/provider_test.go`:

```go
package provider

import (
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

type mockProvider struct {
	name   string
	pass   bool
	detectErr error
	buildErr  error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	return cnb.DetectResult{Pass: m.pass}, m.detectErr
}

func (m *mockProvider) Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	return cnb.BuildResult{}, m.buildErr
}

func TestRegisterAndAll(t *testing.T) {
	// Reset registry
	registry = nil

	p1 := &mockProvider{name: "alpha"}
	p2 := &mockProvider{name: "beta"}

	Register(p1)
	Register(p2)

	all := All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d providers, want 2", len(all))
	}
	if all[0].Name() != "alpha" {
		t.Fatalf("All()[0] = %q, want %q", all[0].Name(), "alpha")
	}
	if all[1].Name() != "beta" {
		t.Fatalf("All()[1] = %q, want %q", all[1].Name(), "beta")
	}
}

func TestByName(t *testing.T) {
	registry = nil

	p := &mockProvider{name: "mlflow"}
	Register(p)

	got := ByName("mlflow")
	if got == nil {
		t.Fatal("ByName(mlflow) = nil, want provider")
	}
	if got.Name() != "mlflow" {
		t.Fatalf("ByName(mlflow).Name() = %q, want %q", got.Name(), "mlflow")
	}

	missing := ByName("nonexistent")
	if missing != nil {
		t.Fatal("ByName(nonexistent) should return nil")
	}
}

func TestDetectFirst(t *testing.T) {
	registry = nil

	p1 := &mockProvider{name: "fail-first", pass: false}
	p2 := &mockProvider{name: "pass-second", pass: true}

	Register(p1)
	Register(p2)

	provider, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if !result.Pass {
		t.Fatal("DetectFirst() result.Pass = false, want true")
	}
	if provider.Name() != "pass-second" {
		t.Fatalf("DetectFirst() provider = %q, want %q", provider.Name(), "pass-second")
	}
}

func TestDetectFirst_NonePass(t *testing.T) {
	registry = nil

	Register(&mockProvider{name: "fail1", pass: false})
	Register(&mockProvider{name: "fail2", pass: false})

	_, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if result.Pass {
		t.Fatal("DetectFirst() should not pass when all providers fail")
	}
}

func TestDetectFirst_EmptyRegistry(t *testing.T) {
	registry = nil

	_, result, err := DetectFirst(cnb.DetectContext{})
	if err != nil {
		t.Fatalf("DetectFirst() error = %v", err)
	}
	if result.Pass {
		t.Fatal("DetectFirst() should not pass with empty registry")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/provider/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Write implementation**

Create `buildpack/internal/provider/provider.go`:

```go
// Package provider defines the interface for model registry providers
// and a registry for runtime provider lookup.
package provider

import (
	"github.com/aagumin/mlflowpack/internal/cnb"
)

// Provider implements detect and build for a specific model registry/format.
type Provider interface {
	// Name returns the provider identifier (e.g. "mlflow", "clearml").
	Name() string

	// Detect checks if this provider can handle the given context.
	Detect(ctx cnb.DetectContext) (cnb.DetectResult, error)

	// Build executes the build phase for this provider's model.
	Build(ctx cnb.BuildContext) (cnb.BuildResult, error)
}

// registry holds all registered providers in registration order.
var registry []Provider

// Register adds a provider to the registry.
func Register(p Provider) {
	registry = append(registry, p)
}

// All returns all registered providers.
func All() []Provider {
	return registry
}

// ByName returns the provider with the given name, or nil if not found.
func ByName(name string) Provider {
	for _, p := range registry {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// DetectFirst iterates providers in registration order and returns
// the first one that passes detect. If none pass, returns (nil, DetectResult{Pass: false}, nil).
func DetectFirst(ctx cnb.DetectContext) (Provider, cnb.DetectResult, error) {
	for _, p := range registry {
		result, err := p.Detect(ctx)
		if err != nil {
			return nil, cnb.DetectResult{}, err
		}
		if result.Pass {
			return p, result, nil
		}
	}
	return nil, cnb.DetectResult{Pass: false}, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd buildpack && go test ./internal/provider/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/provider/
git commit -m "feat(provider): add Provider interface and registry"
```

---

### Task 4: Create internal/mlflow/ package and move files

Move all files from `internal/detect/` and `internal/build/` into `internal/mlflow/`, updating package declarations and imports. This is the largest task — a pure mechanical move.

**Files to move from `internal/detect/` to `internal/mlflow/`:**
- `detector.go` → `detect.go`
- `detector_test.go` → `detect_test.go`
- `local_model.go` → `local_model.go`

**Files to move from `internal/build/` to `internal/mlflow/`:**
- `builder.go` → `build.go`
- `source_test.go` → `source_test.go`
- `dependencies_test.go` → `dependencies_test.go`
- `cache.go` → `cache.go`
- `cache_test.go` → `cache_test.go`
- `workdir.go` → `workdir.go`
- `workdir_test.go` → `workdir_test.go`
- `installer_env_test.go` → `installer_env_test.go`

**Changes in each moved file:**
1. Change `package detect` → `package mlflow`
2. Change `package build` → `package mlflow`
3. Update imports: remove `"github.com/aagumin/mlflowpack/internal/detect"` and `"github.com/aagumin/mlflowpack/internal/build"` references (they now refer to types in the same package)
4. Rename `detect.Detect` → `Detect` (already unexported-friendly), `detect.MLmodelFile` → `MLmodelFile` etc.
5. Rename `build.Build` → `Build`, etc.

**Important note on conflicts:** `internal/mlflow/model.go` already defines `MLmodelFile = "MLmodel"` and `internal/detect/detector.go` also defines `MLmodelFile = "MLmodel"`. After the move, one constant definition must be removed. Keep the one in `model.go` (already in `internal/mlflow/`) and remove the duplicate from the moved `detect.go`.

Similarly, `detect.EnvModelPath` and `detect.MLmodelFile` are used externally by `internal/build/` files. After merge into the same package, these become direct references.

**Step 1: Create directory**

Run: `mkdir -p buildpack/internal/mlflow`

**Step 2: Move files from internal/detect/**

Run:
```bash
# Move detect files
cp buildpack/internal/detect/detector.go buildpack/internal/mlflow/detect.go
cp buildpack/internal/detect/detector_test.go buildpack/internal/mlflow/detect_test.go
cp buildpack/internal/detect/local_model.go buildpack/internal/mlflow/local_model.go
```

**Step 3: Move files from internal/build/**

Run:
```bash
cp buildpack/internal/build/builder.go buildpack/internal/mlflow/build.go
cp buildpack/internal/build/source_test.go buildpack/internal/mlflow/source_test.go
cp buildpack/internal/build/dependencies_test.go buildpack/internal/mlflow/dependencies_test.go
cp buildpack/internal/build/cache.go buildpack/internal/mlflow/cache.go
cp buildpack/internal/build/cache_test.go buildpack/internal/mlflow/cache_test.go
cp buildpack/internal/build/workdir.go buildpack/internal/mlflow/workdir.go
cp buildpack/internal/build/workdir_test.go buildpack/internal/mlflow/workdir_test.go
cp buildpack/internal/build/installer_env_test.go buildpack/internal/mlflow/installer_env_test.go
```

**Step 4: Update package declarations and imports in all moved files**

In every moved file:
- Change `package detect` → `package mlflow`
- Change `package build` → `package mlflow`

Specific import changes per file:

`detect.go`:
- Remove `"github.com/aagumin/mlflowpack/internal/detect"` import (self-reference)
- Remove constant block `MLmodelFile` and `EnvModelPath` (duplicates from `model.go` and will conflict). Instead, add references: these are already defined in `model.go` (MLmodelFile) and need to be added to model.go if not there. Actually `EnvModelPath` is not in model.go — keep it in detect.go but just remove the import.
- `DetectStoragePath` uses `EnvModelPath` which will be in the same package.

`detect_test.go`:
- Remove `"github.com/aagumin/mlflowpack/internal/detect"` import
- References to `detect.FindLocalModelDir` → `FindLocalModelDir`
- References to `detect.EnvModelPath` → `EnvModelPath`
- References to `detect.MLmodelFile` → `MLmodelFile`
- References to `detect.ErrLocalModelNotFound` → `ErrLocalModelNotFound`
- References to `detect.Detect` → `Detect`
- References to `detect.DetectStoragePath` → `DetectStoragePath`

`local_model.go`:
- No external import changes needed (already only uses stdlib)
- Keep `ErrLocalModelNotFound`, `FindLocalModelDir` as exported

`build.go`:
- Remove import `"github.com/aagumin/mlflowpack/internal/detect"` — references become same-package
- Remove import `"github.com/aagumin/mlflowpack/internal/build"` (was self-reference)
- `detect.DetectStoragePath` → `DetectStoragePath`
- `detect.FindLocalModelDir` → `FindLocalModelDir`
- `detect.ErrLocalModelNotFound` → `ErrLocalModelNotFound`
- `detect.MLmodelFile` → `MLmodelFile`
- `detect.EnvModelPath` → `EnvModelPath`

`source_test.go`:
- Remove `"github.com/aagumin/mlflowpack/internal/detect"` import
- `detect.EnvModelPath` → `EnvModelPath`
- `detect.MLmodelFile` → `MLmodelFile`

`dependencies_test.go`:
- Already imports `"github.com/aagumin/mlflowpack/internal/mlflow"` — no change needed (same package now)

`cache.go`:
- No changes needed (only imports `os`, `cnb`, `layer`)

`cache_test.go`:
- No import changes needed

`workdir.go`:
- No changes needed (only imports `fmt`, `os`, `path/filepath`, `cnb`)

`workdir_test.go`:
- No import changes needed

`installer_env_test.go`:
- No import changes needed

**Step 5: Resolve MLmodelFile constant conflict**

`model.go` already has: `MLmodelFile = "MLmodel"` in `package mlflow`.
`detect.go` (moved) also defines: `MLmodelFile = "MLmodel"`.

Remove the constant block from `detect.go`:
```go
const (
    MLmodelFile = "MLmodel"
    EnvModelPath = "BP_MLFLOW_MODEL_PATH"
)
```

Keep `EnvModelPath` as a standalone const in `detect.go`:
```go
const EnvModelPath = "BP_MLFLOW_MODEL_PATH"
```

The `MLmodelFile` constant already exists in `model.go`.

**Step 6: Run tests to verify everything compiles and passes**

Run: `cd buildpack && go test ./internal/mlflow/ -v`
Expected: PASS (all tests from the moved files)

**Step 7: Delete old packages**

Run:
```bash
rm -rf buildpack/internal/detect/
rm -rf buildpack/internal/build/
```

**Step 8: Run full test suite**

Run: `cd buildpack && go test ./... -v`
Expected: PASS

**Step 9: Commit**

```bash
git add -A buildpack/internal/
git commit -m "refactor: move detect/build into internal/mlflow package"
```

---

### Task 5: Create MLflow provider adapter

Create the `provider.go` file in `internal/mlflow/` that implements the `provider.Provider` interface and registers via `init()`.

**Files:**
- Create: `buildpack/internal/mlflow/provider.go`
- Test: `buildpack/internal/mlflow/provider_test.go`

**Step 1: Write the failing tests**

Create `buildpack/internal/mlflow/provider_test.go`:

```go
package mlflow

import (
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestMlflowProvider_Name(t *testing.T) {
	p := &mlflowProvider{}
	if p.Name() != "mlflow" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "mlflow")
	}
}

func TestMlflowProvider_Detect_PassesWithS3Path(t *testing.T) {
	p := &mlflowProvider{}
	appDir := t.TempDir()
	t.Setenv(EnvModelPath, "s3://bucket/models/v1")
	buildPlanPath := t.TempDir() + "/buildplan.toml"

	result, err := p.Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !result.Pass {
		t.Fatal("Detect() should pass with S3 path")
	}
}

func TestMlflowProvider_Detect_FailsWithoutModel(t *testing.T) {
	p := &mlflowProvider{}
	appDir := t.TempDir()

	result, err := p.Detect(cnb.DetectContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Pass {
		t.Fatal("Detect() should fail without model")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/mlflow/ -run TestMlflowProvider -v`
Expected: FAIL — `mlflowProvider` undefined

**Step 3: Write implementation**

Create `buildpack/internal/mlflow/provider.go`:

```go
package mlflow

import (
	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/provider"
)

type mlflowProvider struct{}

func (p *mlflowProvider) Name() string { return "mlflow" }

func (p *mlflowProvider) Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	return Detect(ctx)
}

func (p *mlflowProvider) Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	return Build(ctx)
}

func init() {
	provider.Register(&mlflowProvider{})
}
```

**Step 4: Run tests to verify they pass**

Run: `cd buildpack && go test ./internal/mlflow/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/mlflow/provider.go buildpack/internal/mlflow/provider_test.go
git commit -m "feat(mlflow): add provider adapter with init registration"
```

---

### Task 6: Update cmd/detect to use provider router

Rewrite `cmd/detect/main.go` to import providers and delegate to `provider.DetectFirst()`.

**Files:**
- Modify: `buildpack/cmd/detect/main.go`

**Step 1: Rewrite cmd/detect/main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/provider"

	// Register providers via init()
	_ "github.com/aagumin/mlflowpack/internal/mlflow"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx := cnb.DetectContext{
		PlatformDir:   os.Getenv("CNB_PLATFORM_DIR"),
		BuildPlanPath: os.Getenv("CNB_BUILD_PLAN_PATH"),
		BuildpackDir:  os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:       os.Getenv("CNB_EXEC_ENV"),
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: getting working directory: %v\n", err)
		return cnb.ExitCodeErr
	}
	ctx.AppDir = wd

	p, result, err := provider.DetectFirst(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return cnb.ExitCodeErr
	}

	if !result.Pass {
		return cnb.ExitCodeFail
	}

	// Write build plan with provider name in metadata
	plan := cnb.BuildPlan{
		Provides: []cnb.BuildPlanEntry{
			{Name: "model"},
		},
		Requires: []cnb.BuildPlanEntry{
			{Name: "model", Metadata: map[string]interface{}{"provider": p.Name()}},
		},
	}

	if err := cnb.WriteBuildPlan(ctx.BuildPlanPath, plan); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: writing build plan: %v\n", err)
		return cnb.ExitCodeErr
	}

	return cnb.ExitCodePass
}
```

**Step 2: Verify it compiles**

Run: `cd buildpack && go build ./cmd/detect/`
Expected: no errors

**Step 3: Commit**

```bash
git add buildpack/cmd/detect/main.go
git commit -m "refactor(detect): use provider router instead of direct detect call"
```

---

### Task 7: Update cmd/build to read provider from plan and delegate

Rewrite `cmd/build/main.go` to read provider name from build plan and delegate to the correct provider.

**Files:**
- Modify: `buildpack/cmd/build/main.go`

**Step 1: Rewrite cmd/build/main.go**

```go
package main

import (
	"fmt"
	"os"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/provider"

	// Register providers via init()
	_ "github.com/aagumin/mlflowpack/internal/mlflow"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := cnb.BuildContext{
		LayersDir:    os.Getenv("CNB_LAYERS_DIR"),
		PlatformDir:  os.Getenv("CNB_PLATFORM_DIR"),
		BpPlanPath:   os.Getenv("CNB_BP_PLAN_PATH"),
		BuildpackDir: os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:      os.Getenv("CNB_EXEC_ENV"),
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	ctx.AppDir = wd

	// Read provider name from build plan
	plan, err := cnb.ReadBuildPlan(ctx.BpPlanPath)
	if err != nil {
		return fmt.Errorf("reading build plan: %w", err)
	}

	var providerName string
	for _, req := range plan.Requires {
		if v, ok := req.Metadata["provider"].(string); ok {
			providerName = v
			break
		}
	}
	if providerName == "" {
		return fmt.Errorf("build plan does not specify a provider")
	}

	p := provider.ByName(providerName)
	if p == nil {
		return fmt.Errorf("unknown provider: %s", providerName)
	}

	result, err := p.Build(ctx)
	if err != nil {
		return err
	}

	// Write layer TOML files
	for layerName, metadata := range result.Layers {
		if err := cnb.WriteLayerToml(ctx.LayersDir, layerName, metadata); err != nil {
			return fmt.Errorf("writing layer %s: %w", layerName, err)
		}
	}

	// Write launch.toml
	if err := cnb.WriteLaunchToml(ctx.LayersDir, result.Launch); err != nil {
		return fmt.Errorf("writing launch.toml: %w", err)
	}

	return nil
}
```

**Step 2: Verify it compiles**

Run: `cd buildpack && go build ./cmd/build/`
Expected: no errors

**Step 3: Commit**

```bash
git add buildpack/cmd/build/main.go
git commit -m "refactor(build): read provider from plan and delegate"
```

---

### Task 8: Update MLflow detect.go to write provider-agnostic plan

Now that `cmd/detect/main.go` writes the build plan, the MLflow `Detect()` function should NOT write a build plan itself — it should only return pass/fail. The plan writing is now the responsibility of the cmd layer.

**Files:**
- Modify: `buildpack/internal/mlflow/detect.go`

**Step 1: Simplify Detect function**

The current `Detect()` calls `writePlanAndPass()`. After refactoring, detect only returns pass/fail — the cmd layer writes the plan.

Replace the `Detect` function in `buildpack/internal/mlflow/detect.go`:

```go
// Detect checks if an MLmodel file exists in the application directory
// or a storage path is provided via BP_MLFLOW_MODEL_PATH.
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	// Check for storage path (s3:// or file:// or absolute path)
	if _, _, ok := DetectStoragePath(); ok {
		return cnb.DetectResult{Pass: true}, nil
	}

	// Check for local MLmodel file
	if _, err := FindLocalModelDir(ctx.AppDir); err != nil {
		if errors.Is(err, ErrLocalModelNotFound) {
			return cnb.DetectResult{Pass: false}, nil
		}
		return cnb.DetectResult{}, err
	}

	return cnb.DetectResult{Pass: true}, nil
}
```

Remove the `writePlanAndPass` function entirely.

Remove the unused import of `"github.com/aagumin/mlflowpack/internal/cnb"` if it's no longer needed — but it IS still needed for `cnb.DetectContext` and `cnb.DetectResult`.

**Step 2: Update detect tests**

In `buildpack/internal/mlflow/detect_test.go`, tests that verify plan file creation no longer apply (detect no longer writes plans). Update them:

- `TestDetect` subtest `"passes with nested local model"` — no longer checks build plan file, just checks `result.Pass == true`
- `TestDetect` subtest `"s3 uri in model path env passes detection"` — same, just check `result.Pass`
- Remove any assertions about build plan file existence or content from detect tests

The updated tests should be:

```go
func TestDetect(t *testing.T) {
	t.Run("passes with nested local model", func(t *testing.T) {
		appDir := t.TempDir()
		writeMLmodelFile(t, filepath.Join(appDir, "nested", "model"))

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true")
		}
	})

	t.Run("fails with no local model and no storage env", func(t *testing.T) {
		appDir := t.TempDir()

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if res.Pass {
			t.Fatal("Detect() = true, want false")
		}
	})

	t.Run("returns error on multiple local models", func(t *testing.T) {
		appDir := t.TempDir()
		writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
		writeMLmodelFile(t, filepath.Join(appDir, "models", "second"))

		_, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), EnvModelPath) {
			t.Fatalf("error %q does not mention %s", err, EnvModelPath)
		}
	})

	t.Run("s3 uri in model path env passes detection", func(t *testing.T) {
		appDir := t.TempDir()
		t.Setenv(EnvModelPath, "s3://bucket/path/to/model")

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true (S3 path should pass detection)")
		}
	})
}
```

**Step 3: Run full test suite**

Run: `cd buildpack && go test ./... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add buildpack/internal/mlflow/detect.go buildpack/internal/mlflow/detect_test.go
git commit -m "refactor(mlflow): simplify Detect to return pass/fail only"
```

---

### Task 9: Run full build and test verification

Verify the entire project compiles, all tests pass, and lint is clean.

**Step 1: Run all tests**

Run: `cd buildpack && go test -v -race ./...`
Expected: ALL PASS

**Step 2: Run lint**

Run: `cd buildpack && golangci-lint run ./...`
Expected: no issues

**Step 3: Run make build**

Run: `make build`
Expected: binaries compile successfully for amd64 and arm64

**Step 4: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "fix: address lint/test issues from multi-provider refactor"
```

---

### Summary of Changes

| Action | File | Description |
|--------|------|-------------|
| Modify | `buildpack/internal/cnb/types.go` | Add `Metadata` field to `BuildPlanEntry` |
| Create | `buildpack/internal/cnb/types_test.go` | Test for BuildPlanEntry.Metadata |
| Modify | `buildpack/internal/cnb/toml.go` | Add `ReadBuildPlan` function |
| Create | `buildpack/internal/cnb/toml_test.go` | Test for ReadBuildPlan |
| Create | `buildpack/internal/provider/provider.go` | Provider interface + registry |
| Create | `buildpack/internal/provider/provider_test.go` | Tests for registry |
| Move | `internal/detect/*` → `internal/mlflow/*` | MLflow detect files |
| Move | `internal/build/*` → `internal/mlflow/*` | MLflow build files |
| Create | `buildpack/internal/mlflow/provider.go` | MLflow provider adapter + init() |
| Create | `buildpack/internal/mlflow/provider_test.go` | Tests for mlflow provider |
| Modify | `buildpack/internal/mlflow/detect.go` | Remove plan writing from Detect() |
| Modify | `buildpack/internal/mlflow/detect_test.go` | Update tests for simplified Detect |
| Rewrite | `buildpack/cmd/detect/main.go` | Provider router |
| Rewrite | `buildpack/cmd/build/main.go` | Provider dispatcher |
| Delete | `buildpack/internal/detect/` | Removed (moved to mlflow) |
| Delete | `buildpack/internal/build/` | Removed (moved to mlflow) |
