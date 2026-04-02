package cnb

import (
	"testing"
)

func TestBuildPlanEntry_Metadata(t *testing.T) {
	entry := BuildPlanEntry{
		Name:     "model",
		Metadata: map[string]interface{}{"provider": "mlflow"},
	}

	meta, ok := entry.Metadata["provider"].(string)
	if !ok {
		t.Fatalf("Metadata[provider] type = %T, want string", entry.Metadata["provider"])
	}

	if meta != "mlflow" {
		t.Fatalf("Metadata[provider] = %v, want mlflow", meta)
	}
}
