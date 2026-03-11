# modctl MLflow Provider Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate modctl's MLflow provider to enable environment variable credentials and simplify local development.

**Architecture:** Replace custom MLflow REST client with modctl's MLflow provider. Add environment variable fallback to CNB bindings. Change URL format from `models://` to `models:/`.

**Tech Stack:** Go, modctl (github.com/modelpack/modctl), Databricks SDK, AWS SDK

---

## Prerequisites

- modctl project at `~/projects/modctl` (for reference)
- Go 1.24+
- Access to MLflow registry for testing

---

## Task 1: Add modctl Dependency

**Files:**
- Modify: `buildpack/go.mod`
- Modify: `buildpack/go.sum`

**Step 1: Add modctl to go.mod**

Run in `buildpack/`:
```bash
cd buildpack && go get github.com/modelpack/modctl/pkg/modelprovider/mlflow@latest
```

**Step 2: Verify dependency added**

Run: `grep modctl go.mod`
Expected:
```
github.com/modelpack/modctl v0.x.x
```

**Step 3: Tidy dependencies**

Run: `go mod tidy`

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(deps): add modctl mlflow provider dependency"
```

---

## Task 2: Create MLflow Downloader Wrapper

**Files:**
- Create: `buildpack/internal/mlflow/downloader.go`
- Create: `buildpack/internal/mlflow/downloader_test.go`

**Step 1: Write the failing test**

Create `buildpack/internal/mlflow/downloader_test.go`:

```go
package mlflow

import (
	"testing"
)

func TestNewDownloader(t *testing.T) {
	// This test verifies the downloader can be created
	// Actual download tests require MLflow server
	downloader, err := NewDownloader()
	if err != nil {
		t.Fatalf("NewDownloader() error = %v", err)
	}
	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/mlflow/... -run TestNewDownloader -v`
Expected: FAIL with "NewDownloader not defined"

**Step 3: Write minimal implementation**

Create `buildpack/internal/mlflow/downloader.go`:

```go
package mlflow

import (
	"context"
	"fmt"

	modctlmlflow "github.com/modelpack/modctl/pkg/modelprovider/mlflow"
)

// Downloader wraps modctl's MLflow client for model downloading.
type Downloader struct {
	client *modctlmlflow.MlFlowClient
}

// NewDownloader creates a new downloader using modctl's MLflow client.
func NewDownloader() (*Downloader, error) {
	client, err := modctlmlflow.NewMlFlowRegistry(nil)
	if err != nil {
		return nil, fmt.Errorf("creating mlflow client: %w", err)
	}
	return &Downloader{client: &client}, nil
}

// DownloadModel downloads a model from MLflow registry to destDir.
// Returns the path to the downloaded model.
func (d *Downloader) DownloadModel(ctx context.Context, name, version, destDir string) (string, error) {
	path, err := d.client.PullModelByName(ctx, name, version, destDir)
	if err != nil {
		return "", fmt.Errorf("downloading model %s:%s: %w", name, version, err)
	}
	return path, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/mlflow/... -run TestNewDownloader -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/mlflow/downloader.go buildpack/internal/mlflow/downloader_test.go
git commit -m "feat(mlflow): add modctl-based downloader wrapper"
```

---

## Task 3: Add Environment Variables Fallback to Bindings

**Files:**
- Modify: `buildpack/internal/bindings/bindings.go`
- Modify: `buildpack/internal/bindings/bindings_test.go`

**Step 1: Write the failing test**

Add to `buildpack/internal/bindings/bindings_test.go`:

```go
func TestReadMLflowBindingWithFallback_EnvVars(t *testing.T) {
	// Setup temp dir for bindings (empty)
	tempDir := t.TempDir()
	reader := NewReader(tempDir)

	// Set env vars
	t.Setenv("MLFLOW_TRACKING_URI", "https://mlflow.example.com")
	t.Setenv("MLFLOW_TRACKING_USERNAME", "testuser")
	t.Setenv("MLFLOW_TRACKING_PASSWORD", "testpass")

	binding, err := reader.ReadMLflowBindingWithFallback()
	if err != nil {
		t.Fatalf("ReadMLflowBindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadMLflowBindingWithFallback() returned nil")
	}
	if binding.TrackingURI != "https://mlflow.example.com" {
		t.Errorf("TrackingURI = %q, want %q", binding.TrackingURI, "https://mlflow.example.com")
	}
	if binding.Username != "testuser" {
		t.Errorf("Username = %q, want %q", binding.Username, "testuser")
	}
	if binding.Password != "testpass" {
		t.Errorf("Password = %q, want %q", binding.Password, "testpass")
	}
}

func TestReadMLflowBindingWithFallback_BindingsPriority(t *testing.T) {
	// Setup temp dir with binding
	tempDir := t.TempDir()
	mlflowDir := filepath.Join(tempDir, "mlflow")
	if err := os.MkdirAll(mlflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlflowDir, "type"), []byte("mlflow"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlflowDir, "tracking_uri"), []byte("https://binding.example.com"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set env vars (should be ignored)
	t.Setenv("MLFLOW_TRACKING_URI", "https://env.example.com")

	reader := NewReader(tempDir)
	binding, err := reader.ReadMLflowBindingWithFallback()
	if err != nil {
		t.Fatalf("ReadMLflowBindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadMLflowBindingWithFallback() returned nil")
	}
	// Bindings should take priority
	if binding.TrackingURI != "https://binding.example.com" {
		t.Errorf("TrackingURI = %q, want %q (from bindings)", binding.TrackingURI, "https://binding.example.com")
	}
}

func TestReadS3BindingWithFallback_EnvVars(t *testing.T) {
	tempDir := t.TempDir()
	reader := NewReader(tempDir)

	// Set env vars
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com")

	binding, err := reader.ReadS3BindingWithFallback()
	if err != nil {
		t.Fatalf("ReadS3BindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadS3BindingWithFallback() returned nil")
	}
	if binding.AccessKey != "test-access-key" {
		t.Errorf("AccessKey = %q, want %q", binding.AccessKey, "test-access-key")
	}
	if binding.SecretKey != "test-secret-key" {
		t.Errorf("SecretKey = %q, want %q", binding.SecretKey, "test-secret-key")
	}
	if binding.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", binding.Region, "us-west-2")
	}
	if binding.Endpoint != "https://s3.example.com" {
		t.Errorf("Endpoint = %q, want %q", binding.Endpoint, "https://s3.example.com")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/bindings/... -run "WithFallback" -v`
Expected: FAIL with "ReadMLflowBindingWithFallback not defined"

**Step 3: Write minimal implementation**

Add to `buildpack/internal/bindings/bindings.go`:

```go
// ReadMLflowBindingWithFallback reads MLflow binding with environment variable fallback.
// Bindings take priority over environment variables.
func (r *Reader) ReadMLflowBindingWithFallback() (*MLflowBinding, error) {
	// Try bindings first
	binding, err := r.ReadMLflowBinding()
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return binding, nil
	}

	// Fallback to environment variables
	return readMLflowFromEnv(), nil
}

// ReadS3BindingWithFallback reads S3 binding with environment variable fallback.
// Bindings take priority over environment variables.
func (r *Reader) ReadS3BindingWithFallback() (*S3Binding, error) {
	// Try bindings first
	binding, err := r.ReadS3Binding()
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return binding, nil
	}

	// Fallback to environment variables
	return readS3FromEnv(), nil
}

func readMLflowFromEnv() *MLflowBinding {
	uri := os.Getenv("MLFLOW_TRACKING_URI")
	if uri == "" {
		uri = os.Getenv("DATABRICKS_HOST")
	}
	if uri == "" {
		return nil
	}

	return &MLflowBinding{
		TrackingURI: uri,
		Username:    os.Getenv("MLFLOW_TRACKING_USERNAME"),
		Password:    os.Getenv("MLFLOW_TRACKING_PASSWORD"),
	}
}

func readS3FromEnv() *S3Binding {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil
	}

	return &S3Binding{
		Endpoint:  os.Getenv("AWS_ENDPOINT_URL"),
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    os.Getenv("AWS_REGION"),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/bindings/... -run "WithFallback" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/bindings/bindings.go buildpack/internal/bindings/bindings_test.go
git commit -m "feat(bindings): add environment variable fallback for credentials"
```

---

## Task 4: Update URL Format to models:/

**Files:**
- Modify: `buildpack/internal/detect/detector.go`
- Modify: `buildpack/internal/detect/detector_test.go`

**Step 1: Write the failing test**

Add to `buildpack/internal/detect/detector_test.go`:

```go
func TestDetectFromModelPathEnv_SingleSlash(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		wantName    string
		wantVersion string
		wantOK      bool
		wantErr     bool
	}{
		{
			name:        "models:/ with version",
			envValue:    "models:/my-model/1",
			wantName:    "my-model",
			wantVersion: "1",
			wantOK:      true,
		},
		{
			name:        "models:/ without version (defaults to latest)",
			envValue:    "models:/my-model",
			wantName:    "my-model",
			wantVersion: "latest",
			wantOK:      true,
		},
		{
			name:     "empty string",
			envValue: "",
			wantOK:   false,
		},
		{
			name:     "local path not affected",
			envValue: "/path/to/model",
			wantOK:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvModelPath, tt.envValue)
			gotName, gotVersion, gotOK, err := DetectFromModelPathEnv()
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectFromModelPathEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOK != tt.wantOK {
				t.Errorf("DetectFromModelPathEnv() ok = %v, want %v", gotOK, tt.wantOK)
				return
			}
			if gotName != tt.wantName {
				t.Errorf("DetectFromModelPathEnv() name = %v, want %v", gotName, tt.wantName)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("DetectFromModelPathEnv() version = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/detect/... -run TestDetectFromModelPathEnv_SingleSlash -v`
Expected: FAIL (models:/ not recognized)

**Step 3: Update implementation**

Modify `buildpack/internal/detect/detector.go`:

```go
const (
	// MLmodelFile is the name of the MLflow model descriptor file.
	MLmodelFile = "MLmodel"

	// EnvModelPath points to a local directory containing MLmodel.
	EnvModelPath = "BP_MLFLOW_MODEL_PATH"

	// ModelRegistryPrefix marks BP_MLFLOW_MODEL_PATH as a registry model URI.
	// Changed from "models://" to "models:/" for modctl compatibility.
	ModelRegistryPrefix = "models:/"
)
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/detect/... -run TestDetectFromModelPathEnv_SingleSlash -v`
Expected: PASS

**Step 5: Run all detect tests**

Run: `cd buildpack && go test ./internal/detect/... -v`
Expected: All PASS (update any tests that used models://)

**Step 6: Commit**

```bash
git add buildpack/internal/detect/detector.go buildpack/internal/detect/detector_test.go
git commit -m "feat(detect): change model registry URL format to models:/"
```

---

## Task 5: Update Builder to Use New Downloader

**Files:**
- Modify: `buildpack/internal/build/builder.go`

**Step 1: Update imports**

In `buildpack/internal/build/builder.go`, update imports:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aagumin/mlflowpack/internal/bindings"
	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/conda"
	"github.com/aagumin/mlflowpack/internal/detect"
	"github.com/aagumin/mlflowpack/internal/layer"
	"github.com/aagumin/mlflowpack/internal/mlflow"
	"github.com/aagumin/mlflowpack/internal/python"
)
```

**Step 2: Replace getModel function**

Replace the `getModel` function in `buildpack/internal/build/builder.go`:

```go
func getModel(ctx cnb.BuildContext, source *modelSource) (*mlflow.Model, error) {
	if source.Type == "local" {
		return mlflow.NewLocalModel(source.Path), nil
	}

	// Get bindings with env vars fallback
	bindingsDir := bindings.GetBindingsDir()
	reader := bindings.NewReader(bindingsDir)

	// Check for MLflow binding (optional - env vars work without it)
	mlflowBinding, err := reader.ReadMLflowBindingWithFallback()
	if err != nil {
		return nil, fmt.Errorf("reading MLflow binding: %w", err)
	}
	if mlflowBinding == nil {
		return nil, fmt.Errorf("MLflow credentials not found: provide bindings at %s or set MLFLOW_TRACKING_URI environment variable", bindingsDir)
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "mlflow-model-")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}

	// Use modctl-based downloader
	downloader, err := mlflow.NewDownloader()
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("creating downloader: %w", err)
	}

	downloadPath, err := downloader.DownloadModel(context.Background(), source.Name, source.Version, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("downloading model: %w", err)
	}

	return mlflow.NewRegistryModel(downloadPath, source.Name, source.Version), nil
}
```

**Step 3: Run tests**

Run: `cd buildpack && go test ./internal/build/... -v`
Expected: PASS (some tests may need updates)

**Step 4: Commit**

```bash
git add buildpack/internal/build/builder.go
git commit -m "feat(build): use modctl-based downloader for registry models"
```

---

## Task 6: Remove Old MLflow Client and Storage Code

**Files:**
- Delete: `buildpack/internal/mlflow/client.go`
- Delete: `buildpack/internal/mlflow/storage/s3.go`

**Step 1: Verify no imports of old code**

Run: `cd buildpack && grep -r "mlflow.NewClient\|storage\." --include="*.go" .`
Expected: No matches (or only in deleted files)

**Step 2: Delete client.go**

```bash
rm buildpack/internal/mlflow/client.go
```

**Step 3: Delete storage directory**

```bash
rm -rf buildpack/internal/mlflow/storage
```

**Step 4: Verify build still works**

Run: `cd buildpack && go build ./...`
Expected: Success

**Step 5: Run all tests**

Run: `cd buildpack && go test ./... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor(mlflow): remove old client and storage code, replaced by modctl"
```

---

## Task 7: Update Documentation

**Files:**
- Modify: `docs/USAGE.md`
- Modify: `README.md`

**Step 1: Update USAGE.md**

Update the "Сборка с MLflow Registry" section to include environment variable option:

```markdown
### Настройка через Environment Variables (для локальной разработки)

Вместо bindings можно использовать environment variables:

```bash
# MLflow Registry
export MLFLOW_TRACKING_URI="https://mlflow.your-company.com"
export MLFLOW_TRACKING_USERNAME="your-username"
export MLFLOW_TRACKING_PASSWORD="your-password"

# S3 (для артефактов)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
export AWS_ENDPOINT_URL="https://s3.your-company.com"  # опционально

# Сборка
pack build my-registry-model \
  --builder aagumin/mlserver-builder:0.1.0 \
  --env BP_MLFLOW_MODEL_PATH="models:/my-classifier/1"
```

### Переменные окружения

| Переменная | Описание |
|------------|----------|
| `MLFLOW_TRACKING_URI` | URL MLflow сервера (или DATABRICKS_HOST) |
| `MLFLOW_TRACKING_USERNAME` | Basic auth username |
| `MLFLOW_TRACKING_PASSWORD` | Basic auth password |
| `AWS_ACCESS_KEY_ID` | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key |
| `AWS_REGION` | AWS region |
| `AWS_ENDPOINT_URL` | Custom S3 endpoint (MinIO, etc.) |
```

**Step 2: Update README.md**

Add environment variable example to README.

**Step 3: Commit**

```bash
git add docs/USAGE.md README.md
git commit -m "docs: add environment variable configuration option"
```

---

## Task 8: Run Full Test Suite

**Step 1: Run all tests**

Run: `cd buildpack && go test ./... -race -v`
Expected: All PASS

**Step 2: Run linter**

Run: `cd buildpack && golangci-lint run`
Expected: No errors

**Step 3: Build the buildpack**

Run: `make build`
Expected: Success

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve test and lint issues"
```

---

## Verification Checklist

- [ ] `models:/name/version` format works
- [ ] Environment variables work as fallback
- [ ] CNB bindings still work (priority)
- [ ] All tests pass
- [ ] Linter passes
- [ ] Documentation updated
- [ ] Old code removed

---

## Rollback Plan

If issues arise:

1. Revert commits in reverse order
2. Old code is available in git history
3. Can temporarily re-add `models://` support if needed
