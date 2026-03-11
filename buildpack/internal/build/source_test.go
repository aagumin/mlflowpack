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

func TestDetermineModelSource_FallsBackToRegistry(t *testing.T) {
	appDir := t.TempDir()
	t.Setenv("BP_MLFLOW_MODEL_NAME", "wine-model")
	t.Setenv("BP_MLFLOW_MODEL_VERSION", "7")

	source, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("determineModelSource() error = %v", err)
	}
	if source.Type != "registry" {
		t.Fatalf("Type = %q, want %q", source.Type, "registry")
	}
	if source.Name != "wine-model" {
		t.Fatalf("Name = %q, want %q", source.Name, "wine-model")
	}
	if source.Version != "7" {
		t.Fatalf("Version = %q, want %q", source.Version, "7")
	}
}

func TestDetermineModelSource_PrefersModelsURIOverLocalFiles(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
	writeMLmodelFile(t, filepath.Join(appDir, "models", "second"))
	t.Setenv(detect.EnvModelPath, "models://wine-model/42")

	source, err := determineModelSource(cnb.BuildContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("determineModelSource() error = %v", err)
	}
	if source.Type != "registry" {
		t.Fatalf("Type = %q, want %q", source.Type, "registry")
	}
	if source.Name != "wine-model" {
		t.Fatalf("Name = %q, want %q", source.Name, "wine-model")
	}
	if source.Version != "42" {
		t.Fatalf("Version = %q, want %q", source.Version, "42")
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
