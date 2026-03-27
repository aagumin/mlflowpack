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
		buildPlanPath := filepath.Join(t.TempDir(), "buildplan.toml")

		res, err := Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
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
		t.Setenv(EnvModelPath, "models:/wine-classifier/7")
		buildPlanPath := filepath.Join(t.TempDir(), "buildplan.toml")

		res, err := Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true")
		}
	})

	t.Run("s3 uri in model path env skips local filesystem detection", func(t *testing.T) {
		appDir := t.TempDir()
		// No local model files - S3 model should be detected via DetectStoragePath in build phase
		t.Setenv(EnvModelPath, "s3://bucket/path/to/model")
		t.Setenv("BP_MLFLOW_MODEL_NAME", "s3-model") // Needed for detect to pass
		buildPlanPath := filepath.Join(t.TempDir(), "buildplan.toml")

		res, err := Detect(cnb.DetectContext{AppDir: appDir, BuildPlanPath: buildPlanPath})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if !res.Pass {
			t.Fatal("Detect() = false, want true (S3 path should be handled like registry path)")
		}
	})
}

func TestDetectFromModelPathEnv(t *testing.T) {
	t.Run("parses models uri with version", func(t *testing.T) {
		t.Setenv(EnvModelPath, "models:/wine-model/12")

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
		t.Setenv(EnvModelPath, "models:/wine-model")

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
		t.Setenv(EnvModelPath, "models:/")

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
			name:     "models:/ path (deprecated, should fail)",
			path:     "models:/my-model/1",
			wantType: "",
			wantPath: "",
			wantOK:   false,
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
