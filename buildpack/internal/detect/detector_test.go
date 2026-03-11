package detect

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestFindLocalModelDir_FindsSingleNestedModel(t *testing.T) {
	appDir := t.TempDir()
	modelDir := filepath.Join(appDir, "models", "iris")
	writeMLmodelFile(t, modelDir)

	got, err := FindLocalModelDir(appDir)
	if err != nil {
		t.Fatalf("FindLocalModelDir() error = %v", err)
	}
	if got != modelDir {
		t.Fatalf("FindLocalModelDir() = %q, want %q", got, modelDir)
	}
}

func TestFindLocalModelDir_PrefersRootModel(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, appDir)
	writeMLmodelFile(t, filepath.Join(appDir, "models", "backup"))

	got, err := FindLocalModelDir(appDir)
	if err != nil {
		t.Fatalf("FindLocalModelDir() error = %v", err)
	}
	if got != appDir {
		t.Fatalf("FindLocalModelDir() = %q, want %q", got, appDir)
	}
}

func TestFindLocalModelDir_UsesEnvModelPath(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
	selected := filepath.Join(appDir, "models", "second")
	writeMLmodelFile(t, selected)

	t.Setenv(EnvModelPath, filepath.Join("models", "second"))

	got, err := FindLocalModelDir(appDir)
	if err != nil {
		t.Fatalf("FindLocalModelDir() error = %v", err)
	}
	if got != selected {
		t.Fatalf("FindLocalModelDir() = %q, want %q", got, selected)
	}
}

func TestFindLocalModelDir_ReturnsAmbiguousError(t *testing.T) {
	appDir := t.TempDir()
	writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
	writeMLmodelFile(t, filepath.Join(appDir, "models", "second"))

	_, err := FindLocalModelDir(appDir)
	if err == nil {
		t.Fatal("expected error for multiple MLmodel files, got nil")
	}
	if !strings.Contains(err.Error(), EnvModelPath) {
		t.Fatalf("error %q does not mention %s", err, EnvModelPath)
	}
}

func TestFindLocalModelDir_ReturnsNotFound(t *testing.T) {
	appDir := t.TempDir()

	_, err := FindLocalModelDir(appDir)
	if !errors.Is(err, ErrLocalModelNotFound) {
		t.Fatalf("expected ErrLocalModelNotFound, got %v", err)
	}
}

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

	t.Run("fails with no local model and no registry env", func(t *testing.T) {
		appDir := t.TempDir()

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if res.Pass {
			t.Fatal("Detect() = true, want false")
		}
	})

	t.Run("passes with registry env", func(t *testing.T) {
		appDir := t.TempDir()
		t.Setenv("BP_MLFLOW_MODEL_NAME", "wine-model")

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true")
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

	t.Run("models uri in model path env skips local filesystem detection", func(t *testing.T) {
		appDir := t.TempDir()
		writeMLmodelFile(t, filepath.Join(appDir, "models", "first"))
		writeMLmodelFile(t, filepath.Join(appDir, "models", "second"))
		t.Setenv(EnvModelPath, "models://wine-classifier/7")

		res, err := Detect(cnb.DetectContext{AppDir: appDir})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true")
		}
	})
}

func TestDetectFromModelPathEnv(t *testing.T) {
	t.Run("parses models uri with version", func(t *testing.T) {
		t.Setenv(EnvModelPath, "models://wine-model/12")

		name, version, ok, err := DetectFromModelPathEnv()
		if err != nil {
			t.Fatalf("DetectFromModelPathEnv() error = %v", err)
		}
		if !ok {
			t.Fatal("DetectFromModelPathEnv() ok = false, want true")
		}
		if name != "wine-model" {
			t.Fatalf("name = %q, want %q", name, "wine-model")
		}
		if version != "12" {
			t.Fatalf("version = %q, want %q", version, "12")
		}
	})

	t.Run("parses models uri without version as latest", func(t *testing.T) {
		t.Setenv(EnvModelPath, "models://wine-model")

		name, version, ok, err := DetectFromModelPathEnv()
		if err != nil {
			t.Fatalf("DetectFromModelPathEnv() error = %v", err)
		}
		if !ok {
			t.Fatal("DetectFromModelPathEnv() ok = false, want true")
		}
		if name != "wine-model" {
			t.Fatalf("name = %q, want %q", name, "wine-model")
		}
		if version != "latest" {
			t.Fatalf("version = %q, want %q", version, "latest")
		}
	})

	t.Run("returns false when local path is provided", func(t *testing.T) {
		t.Setenv(EnvModelPath, "e2e/models/sklearn")

		name, version, ok, err := DetectFromModelPathEnv()
		if err != nil {
			t.Fatalf("DetectFromModelPathEnv() error = %v", err)
		}
		if ok {
			t.Fatalf("DetectFromModelPathEnv() ok = true, want false (name=%q version=%q)", name, version)
		}
	})

	t.Run("errors on malformed models uri", func(t *testing.T) {
		t.Setenv(EnvModelPath, "models://")

		_, _, _, err := DetectFromModelPathEnv()
		if err == nil {
			t.Fatal("expected error for malformed models uri, got nil")
		}
	})
}

func writeMLmodelFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, MLmodelFile)
	if err := os.WriteFile(path, []byte("flavors:\n  sklearn: {}\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
