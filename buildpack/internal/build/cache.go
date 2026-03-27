// Package build implements the CNB build phase for MLflow models.
package build

import (
	"os"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/layer"
)

const (
	// EnvPrevDepsHash is the environment variable for previous dependency hash.
	// Used for cache optimization in orchestrators like kpack.
	EnvPrevDepsHash = "BP_MLFLOW_PREV_DEPS_HASH"
)

// CachedDepsHash returns the dependency hash from cached venv layer metadata.
func CachedDepsHash(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.VenvLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	metadata, ok := meta.Metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	hash, _ := metadata["deps_hash"].(string)
	return hash
}

// CachedPythonVersion returns the Python version from cached python layer metadata.
func CachedPythonVersion(layersDir string) string {
	meta, err := cnb.ReadLayerToml(layersDir, layer.PythonLayerName)
	if err != nil {
		return ""
	}
	if meta.Metadata == nil {
		return ""
	}
	metadata, ok := meta.Metadata.(map[string]interface{})
	if !ok {
		return ""
	}
	version, _ := metadata["python_version"].(string)
	return version
}

// PrevDepsHashFromEnv returns the previous deps hash from environment variable.
// This allows orchestrators to pass the hash from previous build for comparison.
func PrevDepsHashFromEnv() string {
	return os.Getenv(EnvPrevDepsHash)
}
