// Package layer provides utilities for managing CNB layers.
package layer

import (
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb/v2"
)

const (
	// PythonLayerName is the name of the Python installation layer.
	PythonLayerName = "python"

	// VenvLayerName is the name of the virtual environment layer.
	VenvLayerName = "venv"

	// ModelLayerName is the name of the model artifacts layer.
	ModelLayerName = "model"
)

// DefaultPythonLayerTypes returns the layer types for the Python layer.
func DefaultPythonLayerTypes() libcnb.LayerTypes {
	return libcnb.LayerTypes{
		Build:  true,
		Launch: true,
		Cache:  true,
	}
}

// DefaultVenvLayerTypes returns the layer types for the venv layer.
func DefaultVenvLayerTypes() libcnb.LayerTypes {
	return libcnb.LayerTypes{
		Build:  false,
		Launch: true,
		Cache:  true,
	}
}

// DefaultModelLayerTypes returns the layer types for the model layer.
func DefaultModelLayerTypes() libcnb.LayerTypes {
	return libcnb.LayerTypes{
		Build:  false,
		Launch: true,
		Cache:  true,
	}
}

// Manager handles layer operations.
type Manager struct {
	layersDir string
}

// NewManager creates a new layer manager.
func NewManager(layersDir string) *Manager {
	return &Manager{
		layersDir: layersDir,
	}
}

// GetLayerPath returns the full path for a named layer.
func (m *Manager) GetLayerPath(name string) string {
	return filepath.Join(m.layersDir, name)
}

// EnsureLayer creates the layer directory if it doesn't exist.
func (m *Manager) EnsureLayer(name string) (string, error) {
	path := m.GetLayerPath(name)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}
	return path, nil
}

// AppendToPath appends a path to the PATH environment variable.
func AppendToPath(layer *libcnb.Layer, path string) {
	layer.SharedEnvironment.Append("PATH", string(os.PathListSeparator), path)
}

// PrependToPath prepends a path to the PATH environment variable.
func PrependToPath(layer *libcnb.Layer, path string) {
	layer.SharedEnvironment.Prepend("PATH", string(os.PathListSeparator), path)
}

// SetPythonPath sets the PYTHONPATH environment variable.
func SetPythonPath(layer *libcnb.Layer, path string) {
	layer.SharedEnvironment.Default("PYTHONPATH", path)
}

// SetLayerEnv sets an environment variable in the layer.
func SetLayerEnv(layer *libcnb.Layer, name, value string) {
	layer.SharedEnvironment.Default(name, value)
}
