// Package detect implements the CNB detect phase for MLflow models.
package detect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

const (
	// MLmodelFile is the name of the MLflow model descriptor file.
	MLmodelFile = "MLmodel"

	// EnvModelPath points to a model location (s3://, file://, or local path).
	EnvModelPath = "BP_MLFLOW_MODEL_PATH"
)

// Detect checks if an MLmodel file exists in the application directory
// or a storage path is provided via BP_MLFLOW_MODEL_PATH.
// If found, it returns a build plan that provides "mlflow-model".
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	// Check for storage path (s3:// or file:// or absolute path)
	if _, _, ok := DetectStoragePath(); ok {
		return writePlanAndPass(ctx)
	}

	// Check for local MLmodel file
	if _, err := FindLocalModelDir(ctx.AppDir); err != nil {
		if errors.Is(err, ErrLocalModelNotFound) {
			return cnb.DetectResult{Pass: false}, nil
		}
		return cnb.DetectResult{}, err
	}

	return writePlanAndPass(ctx)
}

// writePlanAndPass writes the build plan and returns a passing result.
// The buildpack both requires and provides mlflow-model, allowing it to work
// standalone while also enabling other buildpacks to depend on it.
func writePlanAndPass(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	plan := cnb.BuildPlan{
		Provides: []cnb.BuildPlanEntry{
			{Name: "mlflow-model"},
		},
		Requires: []cnb.BuildPlanEntry{
			{Name: "mlflow-model"},
		},
	}

	if err := cnb.WriteBuildPlan(ctx.BuildPlanPath, plan); err != nil {
		return cnb.DetectResult{}, fmt.Errorf("writing build plan: %w", err)
	}

	return cnb.DetectResult{Pass: true}, nil
}

// DetectStoragePath detects a storage path from BP_MLFLOW_MODEL_PATH.
// Returns (storageType, path, ok) where storageType is "s3" or "local".
// Supported formats:
//   - s3://bucket/path/to/model
//   - /local/path/to/model (absolute path only)
//   - file:///local/path/to/model
//
// Relative paths are NOT handled here - they are resolved by FindLocalModelDir.
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

	// Only treat absolute paths as storage paths
	// Relative paths are handled by FindLocalModelDir
	if filepath.IsAbs(raw) {
		return "local", raw, true
	}

	return "", "", false
}
