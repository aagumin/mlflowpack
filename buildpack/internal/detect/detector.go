// Package detect implements the CNB detect phase for MLflow models.
package detect

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

const (
	// MLmodelFile is the name of the MLflow model descriptor file.
	MLmodelFile = "MLmodel"

	// EnvModelPath points to a local directory containing MLmodel.
	EnvModelPath = "BP_MLFLOW_MODEL_PATH"

	// ModelRegistryPrefix marks BP_MLFLOW_MODEL_PATH as a registry model URI.
	// Changed from "models://" to "models:/" for modctl compatibility.
	ModelRegistryPrefix = "models:/"
)

// Detect checks if an MLmodel file exists in the application directory.
// If found, it returns a build plan that provides "mlflow-model".
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	if _, _, ok, err := DetectFromModelPathEnv(); err != nil {
		return cnb.DetectResult{}, err
	} else if ok {
		return writePlanAndPass(ctx)
	}

	if _, err := FindLocalModelDir(ctx.AppDir); err != nil {
		// Not a local model build; registry mode may still apply.
		if errors.Is(err, ErrLocalModelNotFound) {
			if _, _, ok, err := DetectFromEnv(); err != nil {
				return cnb.DetectResult{}, err
			} else if !ok {
				return cnb.DetectResult{Pass: false}, nil
			}
		} else {
			return cnb.DetectResult{}, err
		}
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

// DetectFromEnv checks if model parameters are provided via environment variables.
// This allows building without an MLmodel file in the application path.
func DetectFromEnv() (modelName, modelVersion string, ok bool, err error) {
	if modelName, modelVersion, ok, err = DetectFromModelPathEnv(); ok || err != nil {
		return modelName, modelVersion, ok, err
	}

	modelName = os.Getenv("BP_MLFLOW_MODEL_NAME")
	if modelName == "" {
		return "", "", false, nil
	}

	modelVersion = os.Getenv("BP_MLFLOW_MODEL_VERSION")
	if modelVersion == "" {
		modelVersion = os.Getenv("BP_MLFLOW_MODEL_STAGE")
	}

	if modelVersion == "" {
		modelVersion = "latest"
	}

	return modelName, modelVersion, true, nil
}

// DetectFromModelPathEnv detects a registry model reference from BP_MLFLOW_MODEL_PATH.
// Supported format:
//
//	models:/<model-name>
//	models:/<model-name>/<version-or-stage>
//
// Deprecated: Use DetectStoragePath for s3:// and local paths.
func DetectFromModelPathEnv() (modelName, modelVersion string, ok bool, err error) {
	raw := strings.TrimSpace(os.Getenv(EnvModelPath))
	if raw == "" {
		return "", "", false, nil
	}
	if !strings.HasPrefix(raw, ModelRegistryPrefix) {
		return "", "", false, nil
	}

	ref := strings.TrimPrefix(raw, ModelRegistryPrefix)
	parts := strings.Split(ref, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false, fmt.Errorf("%s=%q is invalid: expected %s<model-name>[/<version-or-stage>]", EnvModelPath, raw, ModelRegistryPrefix)
	}
	if len(parts) > 2 {
		return "", "", false, fmt.Errorf("%s=%q is invalid: expected %s<model-name>[/<version-or-stage>]", EnvModelPath, raw, ModelRegistryPrefix)
	}

	modelName = strings.TrimSpace(parts[0])
	if modelName == "" {
		return "", "", false, fmt.Errorf("%s=%q is invalid: model name is empty", EnvModelPath, raw)
	}

	if len(parts) == 2 {
		modelVersion = strings.TrimSpace(parts[1])
		if modelVersion == "" {
			return "", "", false, fmt.Errorf("%s=%q is invalid: model version/stage is empty", EnvModelPath, raw)
		}
	} else {
		modelVersion = "latest"
	}

	return modelName, modelVersion, true, nil
}

// DetectStoragePath detects a storage path from BP_MLFLOW_MODEL_PATH.
// Returns (storageType, path, ok) where storageType is "s3" or "local".
// Supported formats:
//   - s3://bucket/path/to/model
//   - /local/path/to/model
//   - file:///local/path/to/model
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
	if strings.HasPrefix(raw, ModelRegistryPrefix) {
		// Deprecated - return false to indicate not supported
		return "", "", false
	}

	// Assume local path
	return "local", raw, true
}
