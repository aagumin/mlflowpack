package mlflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aagumin/mlflowpack/internal/python"
)

func TestResolveDependencies_FromMLmodel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create MLmodel with python_version and cloudpickle_version
	mlmodelContent := []byte(`artifact_path: model
flavors:
  python_function:
    cloudpickle_version: 2.2.1
    code: code
    env:
      virtualenv: python_env.yaml
    loader_module: mlflow.pyfunc.model
    python_model: python_model.pkl
    python_version: 3.10.16
    streamable: false
mlflow_version: 2.15.0
model_uuid: test-uuid
`)
	if err := os.WriteFile(filepath.Join(dir, MLmodelFile), mlmodelContent, 0o644); err != nil {
		t.Fatalf("write MLmodel: %v", err)
	}

	// Create python_env.yaml with -r requirements.txt
	pythonEnvContent := []byte(`python: 3.10.16
build_dependencies:
- pip
- setuptools
- wheel
dependencies:
- -r requirements.txt
`)
	if err := os.WriteFile(filepath.Join(dir, "python_env.yaml"), pythonEnvContent, 0o644); err != nil {
		t.Fatalf("write python_env.yaml: %v", err)
	}

	// Create requirements.txt with flags
	reqContent := []byte(`pandas==2.2.0
--extra-index-url https://example.com/pypi/simple
`)
	if err := os.WriteFile(filepath.Join(dir, RequirementsFile), reqContent, 0o644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}

	model := NewLocalModel(dir)
	if err := model.ParseMLmodel(); err != nil {
		t.Fatalf("ParseMLmodel: %v", err)
	}

	deps, err := resolveDependencies(model)
	if err != nil {
		t.Fatalf("resolveDependencies: %v", err)
	}

	if deps.pythonVersion != "3.10.16" {
		t.Fatalf("pythonVersion = %q, want 3.10.16", deps.pythonVersion)
	}

	if deps.cloudpicklePkg != "cloudpickle==2.2.1" {
		t.Fatalf("cloudpicklePkg = %q, want cloudpickle==2.2.1", deps.cloudpicklePkg)
	}

	wantReqPath := filepath.Join(dir, RequirementsFile)
	if deps.requirementsPath != wantReqPath {
		t.Fatalf("requirementsPath = %q, want %q", deps.requirementsPath, wantReqPath)
	}
}

func TestResolveDependencies_DefaultPythonVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// MLmodel without python_version
	mlmodelContent := []byte(`artifact_path: model
flavors:
  python_function:
    cloudpickle_version: ""
    code: code
    loader_module: mlflow.pyfunc.model
    python_model: python_model.pkl
    python_version: ""
mlflow_version: 2.15.0
model_uuid: test-uuid
`)
	if err := os.WriteFile(filepath.Join(dir, MLmodelFile), mlmodelContent, 0o644); err != nil {
		t.Fatalf("write MLmodel: %v", err)
	}

	model := NewLocalModel(dir)
	if err := model.ParseMLmodel(); err != nil {
		t.Fatalf("ParseMLmodel: %v", err)
	}

	deps, err := resolveDependencies(model)
	if err != nil {
		t.Fatalf("resolveDependencies: %v", err)
	}

	if deps.pythonVersion != python.DefaultPythonVersion {
		t.Fatalf("pythonVersion = %q, want %q", deps.pythonVersion, python.DefaultPythonVersion)
	}

	if deps.cloudpicklePkg != "" {
		t.Fatalf("cloudpicklePkg = %q, want empty", deps.cloudpicklePkg)
	}
}

func TestResolveDependencies_RequirementsWithoutPythonEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// MLmodel with python_version but no virtualenv reference
	mlmodelContent := []byte(`artifact_path: model
flavors:
  python_function:
    code: code
    loader_module: mlflow.pyfunc.model
    python_model: python_model.pkl
    python_version: 3.11.0
mlflow_version: 2.15.0
model_uuid: test-uuid
`)
	if err := os.WriteFile(filepath.Join(dir, MLmodelFile), mlmodelContent, 0o644); err != nil {
		t.Fatalf("write MLmodel: %v", err)
	}

	// Just a requirements.txt without python_env.yaml
	reqContent := []byte(`numpy==1.24.0
`)
	if err := os.WriteFile(filepath.Join(dir, RequirementsFile), reqContent, 0o644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}

	model := NewLocalModel(dir)
	if err := model.ParseMLmodel(); err != nil {
		t.Fatalf("ParseMLmodel: %v", err)
	}

	deps, err := resolveDependencies(model)
	if err != nil {
		t.Fatalf("resolveDependencies: %v", err)
	}

	if deps.pythonVersion != "3.11.0" {
		t.Fatalf("pythonVersion = %q, want 3.11.0", deps.pythonVersion)
	}

	wantReqPath := filepath.Join(dir, RequirementsFile)
	if deps.requirementsPath != wantReqPath {
		t.Fatalf("requirementsPath = %q, want %q (fallback to direct requirements.txt)", deps.requirementsPath, wantReqPath)
	}
}
