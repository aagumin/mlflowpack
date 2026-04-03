package mlflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PythonEnv represents a parsed python_env.yaml file (MLflow virtualenv format).
type PythonEnv struct {
	Python            string   `yaml:"python"`
	BuildDependencies []string `yaml:"build_dependencies,omitempty"`
	Dependencies      []string `yaml:"dependencies,omitempty"`
}

// ParsePythonEnvFile parses a python_env.yaml file.
func ParsePythonEnvFile(path string) (*PythonEnv, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading python_env.yaml: %w", err)
	}

	var env PythonEnv
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing python_env.yaml: %w", err)
	}

	return &env, nil
}

// RequirementsFile returns the path to requirements.txt referenced in dependencies,
// or empty string if not found.
func (e *PythonEnv) RequirementsFile(modelDir string) string {
	for _, dep := range e.Dependencies {
		if dep == "-r requirements.txt" {
			return "requirements.txt"
		}
		// Handle other -r references
		if len(dep) > 3 && dep[:3] == "-r " {
			return dep[3:]
		}
	}
	return ""
}
