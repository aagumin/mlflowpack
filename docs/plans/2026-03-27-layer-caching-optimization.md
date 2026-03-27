# Layer Caching Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Optimize buildpack to skip rebuilding Python and venv layers when only model version changes, removing MLflow API dependency and supporting S3/local paths only.

**Architecture:** Two-phase download (metadata first, then artifacts), layer caching by Python version and dependency hash, graceful fallback when no previous build info available.

**Tech Stack:** Go 1.21+, AWS SDK for S3, CNB Layer API v0.10+

---

## Phase 1: Storage Package (Remove MLflow API)

### Task 1: Create Storage Interface

**Files:**
- Create: `buildpack/internal/storage/storage.go`
- Create: `buildpack/internal/storage/storage_test.go`

**Step 1: Write the failing test**

```go
// buildpack/internal/storage/storage_test.go
package storage

import (
	"context"
	"testing"
)

func TestStorageInterface(t *testing.T) {
	// Test that LocalStorage implements Storage interface
	var _ Storage = (*LocalStorage)(nil)
	// Test that S3Storage implements Storage interface (will fail until implemented)
	// var _ Storage = (*S3Storage)(nil)
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "s3 path",
			path:     "s3://bucket/path/to/model",
			wantType: "s3",
			wantPath: "bucket/path/to/model",
			wantErr:  false,
		},
		{
			name:     "local path",
			path:     "/workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantErr:  false,
		},
		{
			name:     "file uri",
			path:     "file:///workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantErr:  false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPath, err := ParsePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotType != tt.wantType {
					t.Errorf("ParsePath() gotType = %v, want %v", gotType, tt.wantType)
				}
				if gotPath != tt.wantPath {
					t.Errorf("ParsePath() gotPath = %v, want %v", gotPath, tt.wantPath)
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/storage/... -v`
Expected: FAIL with "package not found" or "undefined: Storage"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/storage/storage.go
// Package storage provides model storage backends for buildpack.
package storage

import (
	"context"
	"fmt"
	"strings"
)

// Storage defines the interface for model storage backends.
type Storage interface {
	// Download downloads model files to destDir.
	Download(ctx context.Context, destDir string) error

	// DownloadMetadata downloads only metadata files (MLmodel, conda.yaml, requirements.txt).
	DownloadMetadata(ctx context.Context, destDir string) error

	// Exists checks if the model path exists.
	Exists(ctx context.Context) (bool, error)

	// String returns a human-readable representation of the storage.
	String() string
}

// ParsePath parses a model path and returns the storage type and normalized path.
// Supported formats:
//   - s3://bucket/path/to/model
//   - /local/path/to/model
//   - file:///local/path/to/model
func ParsePath(path string) (storageType, normalizedPath string, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", fmt.Errorf("model path cannot be empty")
	}

	if strings.HasPrefix(path, "s3://") {
		// s3://bucket/path -> bucket/path
		normalized := strings.TrimPrefix(path, "s3://")
		parts := strings.SplitN(normalized, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			return "", "", fmt.Errorf("invalid s3 path: %s (expected s3://bucket/path)", path)
		}
		return "s3", normalized, nil
	}

	if strings.HasPrefix(path, "file://") {
		return "local", strings.TrimPrefix(path, "file://"), nil
	}

	// Assume local path
	return "local", path, nil
}

// NewStorage creates a Storage instance based on the path scheme.
func NewStorage(path string) (Storage, error) {
	storageType, normalizedPath, err := ParsePath(path)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case "s3":
		return NewS3Storage(normalizedPath)
	case "local":
		return NewLocalStorage(normalizedPath)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/storage/... -v`
Expected: PASS (except S3Storage check which we'll add later)

**Step 5: Commit**

```bash
git add buildpack/internal/storage/storage.go buildpack/internal/storage/storage_test.go
git commit -m "feat(storage): add storage interface and path parsing"
```

---

### Task 2: Implement Local Storage

**Files:**
- Modify: `buildpack/internal/storage/storage.go`
- Modify: `buildpack/internal/storage/storage_test.go`

**Step 1: Write the failing test**

```go
// Add to buildpack/internal/storage/storage_test.go

func TestLocalStorageExists(t *testing.T) {
	// Create temp dir with model files
	tmpDir := t.TempDir()

	// Create MLmodel file
	if err := os.WriteFile(filepath.Join(tmpDir, "MLmodel"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "existing path",
			path: tmpDir,
			want: true,
		},
		{
			name: "non-existing path",
			path: "/nonexistent/path",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewLocalStorage(tt.path)
			got, err := s.Exists(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalStorage.Exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LocalStorage.Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalStorageDownloadMetadata(t *testing.T) {
	// Create source dir with model files
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create metadata files
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt"} {
		if err := os.WriteFile(filepath.Join(srcDir, file), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Create artifact file (should not be copied)
	if err := os.WriteFile(filepath.Join(srcDir, "model.pkl"), []byte("big data"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLocalStorage(srcDir)
	if err := s.DownloadMetadata(context.Background(), destDir); err != nil {
		t.Fatalf("DownloadMetadata() error = %v", err)
	}

	// Check metadata files exist in dest
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(destDir, file)); err != nil {
			t.Errorf("expected %s to exist in dest", file)
		}
	}

	// Check artifact file does NOT exist in dest
	if _, err := os.Stat(filepath.Join(destDir, "model.pkl")); err == nil {
		t.Error("model.pkl should not be copied in metadata download")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/storage/... -v -run "LocalStorage"`
Expected: FAIL with "undefined: NewLocalStorage"

**Step 3: Write minimal implementation**

```go
// Add to buildpack/internal/storage/storage.go

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// metadataFiles is the list of files to download in metadata-only mode.
var metadataFiles = []string{"MLmodel", "conda.yaml", "requirements.txt"}

// LocalStorage implements Storage for local filesystem.
type LocalStorage struct {
	path string
}

// NewLocalStorage creates a new LocalStorage.
func NewLocalStorage(path string) *LocalStorage {
	return &LocalStorage{path: path}
}

func (s *LocalStorage) String() string {
	return fmt.Sprintf("local:%s", s.path)
}

func (s *LocalStorage) Exists(ctx context.Context) (bool, error) {
	_, err := os.Stat(s.path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) DownloadMetadata(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	for _, file := range metadataFiles {
		srcPath := filepath.Join(s.path, file)
		dstPath := filepath.Join(destDir, file)

		if err := copyFile(srcPath, dstPath); err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist in source, skip
				continue
			}
			return fmt.Errorf("copying %s: %w", file, err)
		}
	}

	return nil
}

func (s *LocalStorage) Download(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	return filepath.Walk(s.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.path, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/storage/... -v -run "LocalStorage"`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/storage/
git commit -m "feat(storage): implement local storage backend"
```

---

### Task 3: Implement S3 Storage (Metadata Only First)

**Files:**
- Create: `buildpack/internal/storage/s3.go`
- Create: `buildpack/internal/storage/s3_test.go`

**Step 1: Write the failing test**

```go
// buildpack/internal/storage/s3_test.go
package storage

import (
	"context"
	"testing"
)

func TestS3StorageParsePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantBucket string
		wantKey   string
	}{
		{
			name:       "simple path",
			path:       "s3://mybucket/models/v1",
			wantBucket: "mybucket",
			wantKey:    "models/v1",
		},
		{
			name:       "nested path",
			path:       "s3://mybucket/path/to/model/v2",
			wantBucket: "mybucket",
			wantKey:    "path/to/model/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewS3StorageFromPath(tt.path)
			if err != nil {
				t.Fatalf("NewS3StorageFromPath() error = %v", err)
			}
			if s.bucket != tt.wantBucket {
				t.Errorf("bucket = %v, want %v", s.bucket, tt.wantBucket)
			}
			if s.keyPrefix != tt.wantKey {
				t.Errorf("keyPrefix = %v, want %v", s.keyPrefix, tt.wantKey)
			}
		})
	}
}

func TestS3StorageImplementsInterface(t *testing.T) {
	var _ Storage = (*S3Storage)(nil)
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/storage/... -v -run "S3"`
Expected: FAIL with "undefined: S3Storage"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/storage/s3.go
package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage for S3 backend.
type S3Storage struct {
	client    *s3.Client
	bucket    string
	keyPrefix string
}

// NewS3Storage creates a new S3Storage from bucket/key format.
func NewS3Storage(bucketKey string) (*S3Storage, error) {
	parts := strings.SplitN(bucketKey, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid s3 path format: %s", bucketKey)
	}
	return &S3Storage{
		bucket:    parts[0],
		keyPrefix: parts[1],
	}, nil
}

// NewS3StorageFromPath creates a new S3Storage from full s3:// URL.
func NewS3StorageFromPath(url string) (*S3Storage, error) {
	path := strings.TrimPrefix(url, "s3://")
	return NewS3Storage(path)
}

// InitClient initializes the S3 client. Must be called before Download methods.
func (s *S3Storage) InitClient(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}
	s.client = s3.NewFromConfig(cfg)
	return nil
}

func (s *S3Storage) String() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.keyPrefix)
}

func (s *S3Storage) Exists(ctx context.Context) (bool, error) {
	if s.client == nil {
		if err := s.InitClient(ctx); err != nil {
			return false, err
		}
	}

	// Check if MLmodel exists
	key := s.keyPrefix + "/MLmodel"
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check for NotFound error
		return false, nil
	}
	return true, nil
}

func (s *S3Storage) DownloadMetadata(ctx context.Context, destDir string) error {
	if s.client == nil {
		if err := s.InitClient(ctx); err != nil {
			return err
		}
	}

	for _, file := range metadataFiles {
		key := s.keyPrefix + "/" + file
		if err := s.downloadFile(ctx, key, destDir, file); err != nil {
			// File doesn't exist, skip
			continue
		}
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, destDir string) error {
	if s.client == nil {
		if err := s.InitClient(ctx); err != nil {
			return err
		}
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.keyPrefix + "/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing s3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			relPath := strings.TrimPrefix(*obj.Key, s.keyPrefix+"/")
			if relPath == "" {
				continue
			}

			if err := s.downloadFile(ctx, *obj.Key, destDir, relPath); err != nil {
				return fmt.Errorf("downloading %s: %w", *obj.Key, err)
			}
		}
	}

	return nil
}

func (s *S3Storage) downloadFile(ctx context.Context, key, destDir, relPath string) error {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dstPath := filepath.Join(destDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, resp.Body)
	return err
}
```

**Step 4: Add AWS SDK dependency**

Run: `cd buildpack && go get github.com/aws/aws-sdk-go-v2 github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/s3`

**Step 5: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/storage/... -v -run "S3"`
Expected: PASS

**Step 6: Commit**

```bash
git add buildpack/internal/storage/s3.go buildpack/internal/storage/s3_test.go buildpack/go.mod buildpack/go.sum
git commit -m "feat(storage): add S3 storage backend"
```

---

## Phase 2: Dependency Hash Package

### Task 4: Create Dependency Hash Package

**Files:**
- Create: `buildpack/internal/deps/hash.go`
- Create: `buildpack/internal/deps/hash_test.go`

**Step 1: Write the failing test**

```go
// buildpack/internal/deps/hash_test.go
package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeHash(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	condaYaml := `
name: test
dependencies:
  - python=3.11
  - pip:
    - pandas==2.0.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte(condaYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	requirements := `pandas==2.0.0
numpy==1.24.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(requirements), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() error = %v", err)
	}

	if hash1 == "" {
		t.Error("ComputeHash() returned empty string")
	}

	// Same files should produce same hash
	hash2, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() second call error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("ComputeHash() not deterministic: %s != %s", hash1, hash2)
	}
}

func TestComputeHashEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	hash, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatalf("ComputeHash() error = %v", err)
	}

	// Empty dir should still produce a hash (default dependencies)
	if hash == "" {
		t.Error("ComputeHash() returned empty string for empty dir")
	}
}

func TestComputeHashChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial conda.yaml
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte("name: test"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Modify conda.yaml
	if err := os.WriteFile(filepath.Join(tmpDir, "conda.yaml"), []byte("name: test2"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash2, err := ComputeHash(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("ComputeHash() should change when files change")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/deps/... -v`
Expected: FAIL with "package not found"

**Step 3: Write minimal implementation**

```go
// buildpack/internal/deps/hash.go
// Package deps provides dependency hash computation for layer caching.
package deps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ComputeHash computes a hash of dependency files in the given directory.
// It hashes conda.yaml and requirements.txt if they exist.
func ComputeHash(dir string) (string, error) {
	h := sha256.New()

	// Files to hash, in sorted order for determinism
	files := []string{"conda.yaml", "requirements.txt"}
	sort.Strings(files)

	hasFiles := false
	for _, file := range files {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("reading %s: %w", file, err)
		}

		hasFiles = true
		h.Write([]byte(file + ":"))
		h.Write(data)
		h.Write([]byte("\n"))
	}

	// If no files, hash a constant to represent default dependencies
	if !hasFiles {
		h.Write([]byte("default-deps"))
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/deps/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/deps/
git commit -m "feat(deps): add dependency hash computation"
```

---

## Phase 3: Update Detect Phase

### Task 5: Update Detect for Storage Paths

**Files:**
- Modify: `buildpack/internal/detect/detector.go`
- Modify: `buildpack/internal/detect/detector_test.go`

**Step 1: Write the failing test**

Add to `buildpack/internal/detect/detector_test.go`:

```go
func TestDetectFromModelPathEnv_StoragePaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType string
		wantPath string
		wantOK   bool
	}{
		{
			name:     "s3 path",
			path:     "s3://bucket/models/v1",
			wantType: "s3",
			wantPath: "bucket/models/v1",
			wantOK:   true,
		},
		{
			name:     "local path",
			path:     "/workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantOK:   true,
		},
		{
			name:     "models:/ path (deprecated, should fail)",
			path:     "models:/my-model/1",
			wantType: "",
			wantPath: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BP_MLFLOW_MODEL_PATH", tt.path)
			storageType, path, ok := DetectStoragePath()
			if ok != tt.wantOK {
				t.Errorf("DetectStoragePath() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok {
				if storageType != tt.wantType {
					t.Errorf("DetectStoragePath() type = %v, want %v", storageType, tt.wantType)
				}
				if path != tt.wantPath {
					t.Errorf("DetectStoragePath() path = %v, want %v", path, tt.wantPath)
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/detect/... -v -run "StoragePaths"`
Expected: FAIL with "undefined: DetectStoragePath"

**Step 3: Update implementation**

Add to `buildpack/internal/detect/detector.go`:

```go
// DetectStoragePath detects a storage path from BP_MLFLOW_MODEL_PATH.
// Returns (storageType, path, ok) where storageType is "s3" or "local".
func DetectStoragePath() (storageType, path string, ok bool) {
	raw := strings.TrimSpace(os.Getenv(EnvModelPath))
	if raw == "" {
		return "", "", false
	}

	// Check for S3 path
	if strings.HasPrefix(raw, "s3://") {
		normalized := strings.TrimPrefix(raw, "s3://")
		return "s3", normalized, true
	}

	// Check for file:// URI
	if strings.HasPrefix(raw, "file://") {
		return "local", strings.TrimPrefix(raw, "file://"), true
	}

	// Check for deprecated models:/ prefix
	if strings.HasPrefix(raw, "models:/") {
		// Deprecated - return false to indicate not supported
		return "", "", false
	}

	// Assume local path
	return "local", raw, true
}
```

Update `Detect` function to use new path detection:

```go
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	// Check for storage path (s3:// or local)
	if _, _, ok := DetectStoragePath(); ok {
		return writePlanAndPass(ctx)
	}

	// Check for local model directory
	if _, err := FindLocalModelDir(ctx.AppDir); err == nil {
		return writePlanAndPass(ctx)
	}

	return cnb.DetectResult{Pass: false}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/detect/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/detect/
git commit -m "feat(detect): support s3:// and local paths in BP_MLFLOW_MODEL_PATH"
```

---

## Phase 4: Update Build Phase

### Task 6: Add Layer Metadata Functions

**Files:**
- Modify: `buildpack/internal/build/builder.go`
- Create: `buildpack/internal/build/cache.go`
- Create: `buildpack/internal/build/cache_test.go`

**Step 1: Write the failing test**

```go
// buildpack/internal/build/cache_test.go
package build

import (
	"testing"
)

func TestCachedDepsHash(t *testing.T) {
	// This will test reading deps_hash from venv layer metadata
	// We'll add implementation after
}

func TestCachedPythonVersion(t *testing.T) {
	// This will test reading python_version from python layer metadata
}
```

**Step 2: Run test to verify it fails**

Run: `cd buildpack && go test ./internal/build/... -v -run "Cached"`
Expected: Tests pass (empty) or we implement

**Step 3: Write implementation**

```go
// buildpack/internal/build/cache.go
package build

import (
	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/layer"
)

// CachedDepsHash returns the dependency hash from cached venv layer metadata.
func CachedDepsHash(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.VenvLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	metadata, ok := meta.Metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	hash, _ := metadata["deps_hash"].(string)
	return hash
}

// CachedPythonVersion returns the Python version from cached python layer metadata.
func CachedPythonVersion(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.PythonLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	metadata, ok := meta.Metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	version, _ := metadata["python_version"].(string)
	return version
}

// PrevDepsHashFromEnv returns the previous deps hash from environment variable.
func PrevDepsHashFromEnv() string {
	return os.Getenv("BP_MLFLOW_PREV_DEPS_HASH")
}
```

**Step 4: Run test to verify it passes**

Run: `cd buildpack && go test ./internal/build/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add buildpack/internal/build/cache.go buildpack/internal/build/cache_test.go
git commit -m "feat(build): add layer cache metadata helpers"
```

---

### Task 7: Update Build Function for Two-Phase Logic

**Files:**
- Modify: `buildpack/internal/build/builder.go`

This is the main refactoring task. The updated Build function should:

1. Download metadata first
2. Compute deps hash
3. Compare with cached/external hash
4. Decide which layers to rebuild

**Step 1: Update imports**

```go
import (
	// ... existing imports
	"github.com/aagumin/mlflowpack/internal/deps"
	"github.com/aagumin/mlflowpack/internal/storage"
)
```

**Step 2: Update Build function**

Replace the existing Build function with the new logic. Key changes:

```go
func Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	result := cnb.BuildResult{
		Layers: make(map[string]cnb.LayerMetadata),
	}

	// Determine model source
	modelSource, err := determineModelSource(ctx)
	if err != nil {
		return result, err
	}

	// Phase 1: Download metadata only
	tempMetaDir, err := TempDir(ctx, "mlflow-metadata-")
	if err != nil {
		return result, fmt.Errorf("creating temp metadata dir: %w", err)
	}
	defer os.RemoveAll(tempMetaDir)

	store, err := storage.NewStorage(modelSource.Path)
	if err != nil {
		return result, fmt.Errorf("creating storage: %w", err)
	}

	if err := store.DownloadMetadata(context.Background(), tempMetaDir); err != nil {
		return result, fmt.Errorf("downloading model metadata: %w", err)
	}

	// Compute dependency hash
	currentDepsHash, err := deps.ComputeHash(tempMetaDir)
	if err != nil {
		return result, fmt.Errorf("computing dependency hash: %w", err)
	}

	// Get previous hash (from env or cache)
	prevDepsHash := PrevDepsHashFromEnv()
	if prevDepsHash == "" {
		prevDepsHash = CachedDepsHash(ctx.LayersDir)
	}

	// Parse MLmodel to get flavor and Python version
	model := mlflow.NewLocalModel(tempMetaDir)
	if err := model.ParseMLmodel(); err != nil {
		return result, fmt.Errorf("parsing MLmodel: %w", err)
	}

	mlserverExt, err := model.GetMLServerExtension()
	if err != nil {
		return result, fmt.Errorf("detecting MLServer extension: %w", err)
	}

	flavor := model.GetPrimaryFlavor()
	fmt.Printf("Model flavor: %s\n", flavor)
	fmt.Printf("Deps hash: %s (prev: %s)\n", currentDepsHash, prevDepsHash)

	// Decide if we need to rebuild dependencies
	depsChanged := currentDepsHash != prevDepsHash

	// Handle caching logic
	if !depsChanged && prevDepsHash != "" {
		fmt.Println("Dependencies unchanged, reusing cached layers")
		// Reuse python and venv layers
		result.Layers[layer.PythonLayerName] = cnb.LayerMetadata{Types: layer.DefaultPythonLayerTypes()}
		result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{Types: layer.DefaultVenvLayerTypes()}
	} else {
		// Rebuild python and venv
		// ... (existing logic for building layers)

		// Store deps hash in venv layer metadata
		result.Layers[layer.VenvLayerName] = cnb.LayerMetadata{
			Types: layer.DefaultVenvLayerTypes(),
			Metadata: map[string]interface{}{
				"deps_hash": currentDepsHash,
			},
		}
	}

	// Phase 2: Download full model
	// ... (existing logic for model layer)

	return result, nil
}
```

**Step 3: Run tests**

Run: `cd buildpack && go test ./... -v`
Expected: Some tests may need updates

**Step 4: Commit**

```bash
git add buildpack/internal/build/
git commit -m "feat(build): implement two-phase download and layer caching"
```

---

## Phase 5: Integration and Cleanup

### Task 8: Remove MLflow API Downloader

**Files:**
- Delete: `buildpack/internal/mlflow/downloader.go`
- Delete: `buildpack/internal/mlflow/downloader_test.go`
- Modify: `buildpack/go.mod` (remove modctl dependency)

**Step 1: Delete old files**

```bash
rm buildpack/internal/mlflow/downloader.go
rm buildpack/internal/mlflow/downloader_test.go
```

**Step 2: Update go.mod**

Run: `cd buildpack && go mod tidy`

**Step 3: Run tests**

Run: `cd buildpack && go test ./... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove MLflow API downloader, use storage package"
```

---

### Task 9: Update Documentation

**Files:**
- Modify: `docs/USAGE.md`
- Modify: `README.md`

Update documentation to reflect new environment variables and removed MLflow API support.

**Step 1: Update docs**

Add new env vars section:
```markdown
### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `BP_MLFLOW_MODEL_PATH` | Yes | Path to model (`s3://bucket/path` or `/local/path`) |
| `BP_MLFLOW_MODEL_NAME` | No | Model name (for image labels) |
| `BP_MLFLOW_MODEL_VERSION` | No | Model version (for image labels) |
| `BP_MLFLOW_PREV_DEPS_HASH` | No | Previous dependency hash for cache optimization |
```

**Step 2: Commit**

```bash
git add docs/USAGE.md README.md
git commit -m "docs: update for storage-based model paths"
```

---

### Task 10: Final Verification

**Step 1: Run full test suite**

```bash
cd buildpack && go test -v -race ./...
```

**Step 2: Build binaries**

```bash
make build
```

**Step 3: Manual test with local model**

```bash
pack build test-model --builder my-builder --env BP_MLFLOW_MODEL_PATH=/path/to/model
```

**Step 4: Commit final**

```bash
git add -A
git commit -m "feat: complete layer caching optimization"
```

---

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-3 | Storage package (interface, local, S3) |
| 2 | 4 | Dependency hash computation |
| 3 | 5 | Update detect phase |
| 4 | 6-7 | Update build phase with caching |
| 5 | 8-10 | Cleanup and documentation |
