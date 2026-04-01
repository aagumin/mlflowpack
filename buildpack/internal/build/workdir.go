package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

const envWorkDir = "BP_MLFLOW_WORK_DIR"

// WorkDir returns the writable root used for build-time temporary data.
func WorkDir(ctx cnb.BuildContext) (string, error) {
	workDir := os.Getenv(envWorkDir)
	if workDir == "" {
		if ctx.LayersDir == "" {
			return "", fmt.Errorf("no work directory configured: set %s or CNB_LAYERS_DIR", envWorkDir)
		}
		workDir = filepath.Join(ctx.LayersDir, "work")
	}

	// #nosec G703 -- workDir comes from build system environment, not user input
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", fmt.Errorf("creating work directory %q: %w", workDir, err)
	}

	return workDir, nil
}

// TempDir creates a temporary directory under the build-time work root.
func TempDir(ctx cnb.BuildContext, pattern string) (string, error) {
	workDir, err := WorkDir(ctx)
	if err != nil {
		return "", err
	}

	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("creating temp root %q: %w", tmpDir, err)
	}

	dir, err := os.MkdirTemp(tmpDir, pattern)
	if err != nil {
		return "", fmt.Errorf("creating temp dir in %q: %w", tmpDir, err)
	}

	return dir, nil
}
