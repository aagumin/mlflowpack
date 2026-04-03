package mlflow

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// MLmodelFile is the name of the MLmodel descriptor file.
	MLmodelFile = "MLmodel"

	// CondaFile is the name of the conda environment file.
	CondaFile = "conda.yaml"

	// RequirementsFile is the name of the requirements file.
	RequirementsFile = "requirements.txt"

	// PythonEnvFile is the name of the MLflow Python environment file.
	PythonEnvFile = "python_env.yaml"
)

// Model represents an MLflow model.
type Model struct {
	// Path is the local path to the model directory.
	Path string

	// ArtifactPath is the original artifact URI.
	ArtifactPath string

	// ModelName is the registered model name (if from registry).
	ModelName string

	// ModelVersion is the model version (if from registry).
	ModelVersion string

	// MLmodel is the parsed MLmodel file.
	MLmodel *MLmodel
}

// MLmodelPath returns the path to the MLmodel file.
func (m *Model) MLmodelPath() string {
	return filepath.Join(m.Path, MLmodelFile)
}

// CondaPath returns the path to the conda.yaml file.
func (m *Model) CondaPath() string {
	return filepath.Join(m.Path, CondaFile)
}

// RequirementsPath returns the path to the requirements.txt file.
func (m *Model) RequirementsPath() string {
	return filepath.Join(m.Path, RequirementsFile)
}

// HasMLmodel checks if the MLmodel file exists.
func (m *Model) HasMLmodel() bool {
	_, err := os.Stat(m.MLmodelPath())
	return err == nil
}

// HasConda checks if the conda.yaml file exists.
func (m *Model) HasConda() bool {
	_, err := os.Stat(m.CondaPath())
	return err == nil
}

// HasRequirements checks if the requirements.txt file exists.
func (m *Model) HasRequirements() bool {
	_, err := os.Stat(m.RequirementsPath())
	return err == nil
}

// ParseMLmodel parses the MLmodel file.
func (m *Model) ParseMLmodel() error {
	if !m.HasMLmodel() {
		return fmt.Errorf("MLmodel file not found")
	}

	data, err := os.ReadFile(m.MLmodelPath())
	if err != nil {
		return fmt.Errorf("reading MLmodel file: %w", err)
	}

	var mlmodel MLmodel
	if err := yaml.Unmarshal(data, &mlmodel); err != nil {
		return fmt.Errorf("parsing MLmodel file: %w", err)
	}

	m.MLmodel = &mlmodel
	return nil
}

// GetPrimaryFlavor returns the primary flavor of the model.
func (m *Model) GetPrimaryFlavor() string {
	if m.MLmodel == nil {
		return ""
	}
	return m.MLmodel.GetPrimaryFlavor()
}

// GetMLServerExtension returns the MLServer extension for the model.
func (m *Model) GetMLServerExtension() (*MLServerExtension, error) {
	if m.MLmodel == nil {
		return nil, fmt.Errorf("MLmodel not parsed")
	}
	return m.MLmodel.GetMLServerExtension()
}

// UUID returns the model UUID from the MLmodel file.
func (m *Model) UUID() string {
	if m.MLmodel == nil {
		return ""
	}
	return m.MLmodel.ModelUUID
}

// NewLocalModel creates a Model from a local directory.
func NewLocalModel(path string) *Model {
	return &Model{
		Path: path,
	}
}

// NewRegistryModel creates a Model from the registry.
func NewRegistryModel(path, name, version string) *Model {
	return &Model{
		Path:         path,
		ModelName:    name,
		ModelVersion: version,
	}
}
