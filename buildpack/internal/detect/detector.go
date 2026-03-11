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
	ModelRegistryPrefix = "models://"
)

// Detect checks if an MLmodel file exists in the application directory.
// If found, it returns a build plan that provides "mlflow-model".
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	if _, _, ok, err := DetectFromModelPathEnv(); err != nil {
		return cnb.DetectResult{}, err
	} else if ok {
		return cnb.DetectResult{Pass: true}, nil
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
//	models://<model-name>
//	models://<model-name>/<version-or-stage>
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
