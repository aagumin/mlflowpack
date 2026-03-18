# Single Writable Root Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the buildpack operate correctly on a read-only filesystem with a single writable root, ensuring all temporary and tool-generated files are written under a controlled directory instead of the system temp directory.

**Architecture:** Treat the writable root as a platform contract. The buildpack will derive a work directory from `BP_MLFLOW_WORK_DIR` or default to a temporary subdirectory under `CNB_LAYERS_DIR`, and all internal temp paths will flow through that helper. External tools such as `uv` will inherit explicitly controlled `TMPDIR`, `HOME`, and cache variables so they also stay within the writable root.

**Tech Stack:** Go, Cloud Native Buildpacks lifecycle, `uv`, MLflow registry download path management, Go `testing`

### Task 1: Add a build-time work directory contract

**Files:**
- Create: `buildpack/internal/build/workdir.go`
- Create: `buildpack/internal/build/workdir_test.go`

**Step 1: Write the failing tests**

Add table-driven tests for:
- `BP_MLFLOW_WORK_DIR` wins when set.
- Fallback path is `<layersDir>/work`.
- The helper creates the directory when missing.
- Empty `LayersDir` with no override returns an error.

Example test cases:

```go
func TestWorkDir_UsesExplicitOverride(t *testing.T) {
	t.Setenv("BP_MLFLOW_WORK_DIR", filepath.Join(t.TempDir(), "custom-work"))

	got, err := WorkDir(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")})
	if err != nil {
		t.Fatalf("WorkDir() error = %v", err)
	}
	if got != os.Getenv("BP_MLFLOW_WORK_DIR") {
		t.Fatalf("WorkDir() = %q, want %q", got, os.Getenv("BP_MLFLOW_WORK_DIR"))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/build -run TestWorkDir -count=1`

Expected: FAIL because `WorkDir` does not exist yet.

**Step 3: Write minimal implementation**

Implement helpers with signatures like:

```go
func WorkDir(ctx cnb.BuildContext) (string, error)
func TempDir(ctx cnb.BuildContext, pattern string) (string, error)
```

Implementation rules:
- Read `BP_MLFLOW_WORK_DIR` first.
- Otherwise use `filepath.Join(ctx.LayersDir, "work")`.
- `os.MkdirAll(workDir, 0o755)` before returning.
- `TempDir` should create a `tmp` child and call `os.MkdirTemp(tmpDir, pattern)`.

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/build -run TestWorkDir -count=1`

Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/build/workdir.go buildpack/internal/build/workdir_test.go
git commit -m "feat(build): add writable work directory helpers"
```

### Task 2: Route registry model downloads through the work directory

**Files:**
- Modify: `buildpack/internal/build/builder.go`
- Create: `buildpack/internal/build/get_model_test.go`

**Step 1: Write the failing test**

Add a focused test for registry downloads that avoids real network access by injecting a fake downloader. Verify:
- `getModel()` creates its temp directory under the work root.
- The downloader receives a destination inside that temp directory.
- The returned model path matches the fake download path.

Recommended seam:

```go
type modelDownloader interface {
	DownloadModel(ctx context.Context, name, version, destDir string) (string, error)
}

var newModelDownloader = func() (modelDownloader, error) {
	return mlflow.NewDownloader()
}
```

Example assertion:

```go
if !strings.HasPrefix(fake.destDir, filepath.Join(layersDir, "work")) {
	t.Fatalf("destDir = %q, want prefix %q", fake.destDir, filepath.Join(layersDir, "work"))
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/build -run TestGetModel_UsesWorkDirForRegistryDownloads -count=1`

Expected: FAIL because `getModel()` still uses `os.MkdirTemp("", ...)`.

**Step 3: Write minimal implementation**

In `getModel()`:
- Replace `os.MkdirTemp("", "mlflow-model-")` with `TempDir(ctx, "mlflow-model-")`.
- Swap direct `mlflow.NewDownloader()` construction for the injectable factory.
- Keep existing cleanup behavior on error and deferred cleanup in `Build()`.

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/build -run TestGetModel_UsesWorkDirForRegistryDownloads -count=1`

Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/build/builder.go buildpack/internal/build/get_model_test.go
git commit -m "fix(build): keep registry downloads inside writable root"
```

### Task 3: Propagate writable-root temp and cache env into `uv`

**Files:**
- Modify: `buildpack/internal/python/installer.go`
- Create: `buildpack/internal/python/installer_test.go`
- Modify: `buildpack/internal/build/builder.go`

**Step 1: Write the failing tests**

Add installer tests that execute a fake `uv` script and capture the environment it receives. Cover:
- Existing `TMPDIR` is forwarded unchanged.
- Existing `HOME` is forwarded unchanged.
- `UV_CACHE_DIR` and `PIP_CACHE_DIR` are forwarded when set.
- Builder-derived defaults are passed when explicit env is absent.

Use a temporary shell script as fake `uv`, for example:

```sh
#!/bin/sh
env > "$UV_ENV_DUMP"
exit 0
```

Test flow:
- Set `UV_ENV_DUMP` to a temp file.
- Run `InstallPython` or `InstallDeps`.
- Read the dump file and assert the expected env variables are present.

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/python -run TestInstaller -count=1`

Expected: FAIL because commands currently inherit the raw parent env without buildpack-controlled defaults.

**Step 3: Write minimal implementation**

Refactor `Installer` to carry command environment overrides, for example:

```go
type Installer struct {
	uvPath string
	env    []string
}

func NewInstallerWithEnv(uvPath string, env []string) *Installer
```

Implementation rules:
- Each `exec.CommandContext` should set `cmd.Env` to a merged environment.
- Preserve the parent environment.
- Override or append only the controlled variables: `TMPDIR`, `TMP`, `TEMP`, `HOME`, `XDG_CACHE_HOME`, `UV_CACHE_DIR`, `PIP_CACHE_DIR`.
- In `build.Build()`, derive defaults from the writable root, for example:
  - `<workRoot>/tmp`
  - `<workRoot>/home`
  - `<workRoot>/cache/uv`
  - `<workRoot>/cache/pip`

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/python -run TestInstaller -count=1`

Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/python/installer.go buildpack/internal/python/installer_test.go buildpack/internal/build/builder.go
git commit -m "feat(python): route uv temp and cache paths through writable root"
```

### Task 4: Document the platform contract for operator and `pack`

**Files:**
- Modify: `docs/USAGE.md`

**Step 1: Write the failing doc expectation**

Add a short checklist in your working notes describing the required docs additions:
- Explain the single writable root contract.
- Document `BP_MLFLOW_WORK_DIR`.
- Show operator directory layout under one writable mount.
- Show `pack build` best-effort example with `--workspace`, `--volume`, `--env`, and bind caches.
- State clearly that strict guarantees apply to the custom operator, while `pack` is compatibility mode.

**Step 2: Verify the docs section does not exist yet**

Run: `rg -n "BP_MLFLOW_WORK_DIR|single writable root|TMPDIR" docs/USAGE.md`

Expected: No relevant section or incomplete coverage.

**Step 3: Write the minimal documentation**

Add a dedicated section such as `Read-only Filesystem / Single Writable Root`.

Include:
- Required writable tree example:

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

- Required lifecycle env for the operator.
- `pack` example command with explicit `TMPDIR`, `HOME`, and cache locations.
- Limitation note: `pack` support is best-effort because the platform abstraction is owned by `pack`.

**Step 4: Verify the docs**

Run: `rg -n "BP_MLFLOW_WORK_DIR|single writable root|Read-only Filesystem" docs/USAGE.md`

Expected: The new section and examples are present.

**Step 5: Commit**

```bash
git add docs/USAGE.md
git commit -m "docs: describe single writable root deployment contract"
```

### Task 5: Run full verification and tighten regressions

**Files:**
- Modify if needed after failures: `buildpack/internal/build/workdir.go`
- Modify if needed after failures: `buildpack/internal/build/builder.go`
- Modify if needed after failures: `buildpack/internal/python/installer.go`
- Modify if needed after failures: `docs/USAGE.md`

**Step 1: Run focused package tests**

Run:

```bash
cd buildpack && go test ./internal/build ./internal/python -count=1
```

Expected: PASS

**Step 2: Run full module tests**

Run:

```bash
cd buildpack && go test ./... -count=1
```

Expected: PASS

**Step 3: Run formatting if needed**

Run:

```bash
gofmt -w buildpack/internal/build/workdir.go buildpack/internal/build/workdir_test.go buildpack/internal/build/builder.go buildpack/internal/build/get_model_test.go buildpack/internal/python/installer.go buildpack/internal/python/installer_test.go
```

Expected: No output, files reformatted in place if necessary.

**Step 4: Re-run the full module tests**

Run:

```bash
cd buildpack && go test ./... -count=1
```

Expected: PASS

**Step 5: Final commit**

```bash
git add buildpack/internal/build/workdir.go buildpack/internal/build/workdir_test.go buildpack/internal/build/builder.go buildpack/internal/build/get_model_test.go buildpack/internal/python/installer.go buildpack/internal/python/installer_test.go docs/USAGE.md
git commit -m "feat(build): support single writable root builds"
```

## Notes for the Implementer

- Do not rely on `/tmp`, `/var/tmp`, or implicit temp behavior anywhere in the buildpack code.
- Keep the writable-root logic build-time only; do not leak these paths into launch-time app configuration unless explicitly required.
- Prefer small seams for testability over broad refactors.
- Do not add a cache persistence feature; the requirement here is one writable root for a single build, not cross-build cache reuse.
- If a third-party tool still writes outside the root after these changes, capture it with a targeted reproduction and handle it as a follow-up bug rather than widening the current scope.
