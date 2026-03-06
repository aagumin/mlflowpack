package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amazme/aipack/buildpack/internal/mlflow"
)

func TestResolveDependencies_FallbackToRequirementsWhenNoConda(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	requirementsPath := filepath.Join(dir, mlflow.RequirementsFile)
	if err := os.WriteFile(requirementsPath, []byte("pandas==2.2.0\n"), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}

	model := mlflow.NewLocalModel(dir)

	deps, err := resolveDependencies(model, "mlserver-sklearn")
	if err != nil {
		t.Fatalf("resolveDependencies returned error: %v", err)
	}

	if deps.requirementsPath != requirementsPath {
		t.Fatalf("requirementsPath = %q, want %q", deps.requirementsPath, requirementsPath)
	}

	pipDeps := deps.conda.PipDependencies()
	for _, pkg := range []string{"mlserver", "mlserver-mlflow", "mlserver-sklearn"} {
		if !containsString(pipDeps, pkg) {
			t.Fatalf("missing required package %q in %v", pkg, pipDeps)
		}
	}
}

func TestResolveDependencies_CondaTakesPrecedenceOverRequirements(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	condaPath := filepath.Join(dir, mlflow.CondaFile)
	condaYAML := []byte("dependencies:\n  - python=3.11\n  - pip:\n    - numpy==2.0.0\n")
	if err := os.WriteFile(condaPath, condaYAML, 0644); err != nil {
		t.Fatalf("write conda.yaml: %v", err)
	}

	requirementsPath := filepath.Join(dir, mlflow.RequirementsFile)
	if err := os.WriteFile(requirementsPath, []byte("pandas==2.2.0\n"), 0644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}

	model := mlflow.NewLocalModel(dir)

	deps, err := resolveDependencies(model, "mlserver-sklearn")
	if err != nil {
		t.Fatalf("resolveDependencies returned error: %v", err)
	}

	if deps.requirementsPath != "" {
		t.Fatalf("requirementsPath = %q, want empty (conda should take precedence)", deps.requirementsPath)
	}

	if got := deps.conda.PythonVersion(); got != "3.11" {
		t.Fatalf("python version = %q, want 3.11", got)
	}

	pipDeps := deps.conda.PipDependencies()
	for _, pkg := range []string{"numpy==2.0.0", "mlserver", "mlserver-mlflow", "mlserver-sklearn"} {
		if !containsString(pipDeps, pkg) {
			t.Fatalf("missing required package %q in %v", pkg, pipDeps)
		}
	}
}

func TestResolveDependencies_ErrorsOnInvalidConda(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	condaPath := filepath.Join(dir, mlflow.CondaFile)
	if err := os.WriteFile(condaPath, []byte("dependencies: ["), 0644); err != nil {
		t.Fatalf("write invalid conda.yaml: %v", err)
	}

	model := mlflow.NewLocalModel(dir)

	_, err := resolveDependencies(model, "mlserver-sklearn")
	if err == nil {
		t.Fatal("expected error for invalid conda.yaml, got nil")
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
