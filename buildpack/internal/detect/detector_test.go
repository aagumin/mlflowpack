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
		buildPlanPath := filepath.Join(t.TempDir(), "buildplan.toml")

		res, err := Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
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
		buildPlanPath := filepath.Join(t.TempDir(), "buildplan.toml")

		res, err := Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true (S3 path should pass detection)")
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

func TestDetectStoragePath(t *testing.T) {
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
			name:     "local absolute path",
			path:     "/workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantOK:   true,
		},
		{
			name:     "file uri",
			path:     "file:///workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantOK:   true,
		},
		{
			name:     "empty path",
			path:     "",
			wantType: "",
			wantPath: "",
			wantOK:   false,
		},
		{
			name:     "relative path (not a storage path)",
			path:     "models/second",
			wantType: "",
			wantPath: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvModelPath, tt.path)
			gotType, gotPath, ok := DetectStoragePath()
			if ok != tt.wantOK {
				t.Errorf("DetectStoragePath() ok = %v, want %v", ok, tt.wantOK)
			}
			if gotType != tt.wantType {
				t.Errorf("DetectStoragePath() type = %v, want %v", gotType, tt.wantType)
			}
			if gotPath != tt.wantPath {
				t.Errorf("DetectStoragePath() path = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}
