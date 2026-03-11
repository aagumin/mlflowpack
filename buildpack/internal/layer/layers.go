// Package layer provides utilities for managing CNB layers.
package layer

import (
	"os"
	"path/filepath"

	"github.com/aagumin/mlflowpack/internal/cnb"
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
func DefaultPythonLayerTypes() cnb.LayerTypes {
	return cnb.LayerTypes{
		Build:  true,
		Launch: true,
		Cache:  true,
	}
}

// DefaultVenvLayerTypes returns the layer types for the venv layer.
func DefaultVenvLayerTypes() cnb.LayerTypes {
	return cnb.LayerTypes{
		Build:  false,
		Launch: true,
		Cache:  true,
	}
}

// DefaultModelLayerTypes returns the layer types for the model layer.
func DefaultModelLayerTypes() cnb.LayerTypes {
	return cnb.LayerTypes{
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
	return &Manager{layersDir: layersDir}
}

// GetLayerPath returns the full path for a named layer.
func (m *Manager) GetLayerPath(name string) string {
	return filepath.Join(m.layersDir, name)
}

// EnsureLayer creates the layer directory if it doesn't exist.
func (m *Manager) EnsureLayer(name string) (string, error) {
	path := m.GetLayerPath(name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// SetupLayer creates the layer directory and writes layer.toml.
func SetupLayer(layersDir, layerName string, types cnb.LayerTypes) (string, error) {
	path := filepath.Join(layersDir, layerName)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}

	// Write layer.toml
	if err := cnb.WriteLayerToml(layersDir, layerName, cnb.LayerMetadata{Types: types}); err != nil {
		return "", err
	}

	return path, nil
}
