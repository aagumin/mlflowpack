package cnb

import (
	"path/filepath"
	"testing"
)

func TestWriteAndReadBuildPlan(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.toml")

	original := BuildPlan{
		Provides: []BuildPlanEntry{
			{Name: "model"},
		},
		Requires: []BuildPlanEntry{
			{Name: "model", Metadata: map[string]interface{}{"provider": "mlflow"}},
		},
	}

	if err := WriteBuildPlan(planPath, original); err != nil {
		t.Fatalf("WriteBuildPlan() error = %v", err)
	}

	read, err := ReadBuildPlan(planPath)
	if err != nil {
		t.Fatalf("ReadBuildPlan() error = %v", err)
	}

	if len(read.Requires) != 1 {
		t.Fatalf("len(Requires) = %d, want 1", len(read.Requires))
	}

	if read.Requires[0].Name != "model" {
		t.Fatalf("Requires[0].Name = %q, want %q", read.Requires[0].Name, "model")
	}

	meta, ok := read.Requires[0].Metadata["provider"].(string)
	if !ok || meta != "mlflow" {
		t.Fatalf("Requires[0].Metadata[provider] = %v, want %q", read.Requires[0].Metadata["provider"], "mlflow")
	}
}

func TestReadBuildPlan_FileNotFound(t *testing.T) {
	_, err := ReadBuildPlan("/nonexistent/plan.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
