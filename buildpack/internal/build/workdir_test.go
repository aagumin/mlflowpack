package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestWorkDir_UsesExplicitOverride(t *testing.T) {
	root := filepath.Join(t.TempDir(), "custom-root")
	t.Setenv("BP_MLFLOW_WORK_DIR", root)

	got, err := WorkDir(cnb.BuildContext{LayersDir: filepath.Join(t.TempDir(), "layers")})
	if err != nil {
		t.Fatalf("WorkDir() error = %v", err)
	}
	if got != root {
		t.Fatalf("WorkDir() = %q, want %q", got, root)
	}

	if info, err := os.Stat(root); err != nil {
		t.Fatalf("stat work dir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("work dir %q is not a directory", root)
	}
}

func TestWorkDir_FallsBackToLayersWorkDir(t *testing.T) {
	t.Parallel()

	layersDir := filepath.Join(t.TempDir(), "layers")

	got, err := WorkDir(cnb.BuildContext{LayersDir: layersDir})
	if err != nil {
		t.Fatalf("WorkDir() error = %v", err)
	}

	want := filepath.Join(layersDir, "work")
	if got != want {
		t.Fatalf("WorkDir() = %q, want %q", got, want)
	}

	if info, err := os.Stat(want); err != nil {
		t.Fatalf("stat work dir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("work dir %q is not a directory", want)
	}
}

func TestWorkDir_ErrorsWithoutLayersDirOrOverride(t *testing.T) {
	t.Parallel()

	_, err := WorkDir(cnb.BuildContext{})
	if err == nil {
		t.Fatal("WorkDir() error = nil, want non-nil")
	}
}

func TestTempDir_CreatesTempUnderWorkRoot(t *testing.T) {
	workRoot := filepath.Join(t.TempDir(), "work-root")
	t.Setenv("BP_MLFLOW_WORK_DIR", workRoot)

	got, err := TempDir(cnb.BuildContext{}, "mlflow-model-")
	if err != nil {
		t.Fatalf("TempDir() error = %v", err)
	}

	wantPrefix := filepath.Join(workRoot, "tmp") + string(os.PathSeparator)
	if !strings.HasPrefix(got+string(os.PathSeparator), wantPrefix) {
		t.Fatalf("TempDir() = %q, want prefix %q", got, wantPrefix)
	}

	if info, err := os.Stat(got); err != nil {
		t.Fatalf("stat temp dir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("temp dir %q is not a directory", got)
	}
}
