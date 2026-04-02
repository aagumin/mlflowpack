package mlflow

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
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
	t.Setenv(EnvModelPath, filepath.Join("models", "second"))

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
	if !strings.Contains(err.Error(), EnvModelPath) {
		t.Fatalf("error %q does not mention %s", err, EnvModelPath)
	}
}

func TestDetermineModelSource_StoragePath(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv(EnvModelPath, "s3://my-bucket/models/v1")

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
