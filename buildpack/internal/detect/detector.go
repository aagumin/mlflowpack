// Package detect implements the CNB detect phase for MLflow models.
package detect

import (
	"os"
	"path/filepath"

	"github.com/amazme/aipack/buildpack/internal/cnb"
)

const (
	// MLmodelFile is the name of the MLflow model descriptor file.
	MLmodelFile = "MLmodel"
)

// Detect checks if an MLmodel file exists in the application directory.
// If found, it returns a build plan that provides "mlflow-model".
func Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	mlmodelPath := filepath.Join(ctx.AppDir, MLmodelFile)

	if _, err := os.Stat(mlmodelPath); os.IsNotExist(err) {
		// Check if model is specified via environment
		if _, _, ok := DetectFromEnv(); !ok {
			return cnb.DetectResult{Pass: false}, nil
		}
	}

	return cnb.DetectResult{Pass: true}, nil
}

// DetectFromEnv checks if model parameters are provided via environment variables.
// This allows building without an MLmodel file in the application path.
func DetectFromEnv() (modelName, modelVersion string, ok bool) {
	modelName = os.Getenv("BP_MLFLOW_MODEL_NAME")
	if modelName == "" {
		return "", "", false
	}

	modelVersion = os.Getenv("BP_MLFLOW_MODEL_VERSION")
	if modelVersion == "" {
		modelVersion = os.Getenv("BP_MLFLOW_MODEL_STAGE")
	}

	if modelVersion == "" {
		modelVersion = "latest"
	}

	return modelName, modelVersion, true
}
