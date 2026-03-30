package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/detect"
)

func TestDetermineModelSource_LocalNestedModel(t *testing.T) {
	appDir := t.TempDir()
	modelDir := filepath.Join(appDir, "models", "iris")
	writeMLmodelFile(t, modelDir)

	source, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("determineModelSource() error = %v", err)
	}
	if source.Type != "local" {
		t.Fatalf("Type = %q, want %q", source.Type, "local")
	}
	if source.Path != modelDir {
		t.Fatalf("Path = %q, want %q", source.Path, modelDir)
	}
}

func TestDetermineModelSource_UsesModelPathEnvForAmbiguousModels(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
	selected := filepath.Join(appDir, "models", "second")
	writeMLmodelFile(t, selected)
	t.Setenv(detect.EnvModelPath, filepath.Join("models", "second"))

	source, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("determineModelSource() error = %v", err)
	}
	if source.Type != "local" {
		t.Fatalf("Type = %q, want %q", source.Type, "local")
	}
	if source.Path != selected {
		t.Fatalf("Path = %q, want %q", source.Path, selected)
	}
}

func TestDetermineModelSource_ReturnsAmbiguousError(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
	writeMLmodelFile(t, filepath.Join(appDir, "models", "second"))

	_, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), detect.EnvModelPath) {
		t.Fatalf("error %q does not mention %s", err, detect.EnvModelPath)
	}
}

func TestDetermineModelSource_StoragePath(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv(detect.EnvModelPath, "s3://my-bucket/models/v1")

	source, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("determineModelSource() error = %v", err)
	}
	if source.Type != "storage" {
		t.Fatalf("Type = %q, want %q", source.Type, "storage")
	}
	if source.StorageType != "s3" {
		t.Fatalf("StorageType = %q, want %q", source.StorageType, "s3")
	}
	if source.Path != "my-bucket/models/v1" {
		t.Fatalf("Path = %q, want %q", source.Path, "my-bucket/models/v1")
	}
	if source.Name != "model" {
		t.Fatalf("Name = %q, want %q", source.Name, "model")
	}
}

func TestDetermineModelSource_NoModelFound(t *testing.T) {
	appDir := t.TempDir()

	_, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err == nil {
		t.Fatal("expected error for no model found, got nil")
	}
}

func writeMLmodelFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, detect.MLmodelFile)
	if err := os.WriteFile(path, []byte("flavors:\n  sklearn: {}\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
