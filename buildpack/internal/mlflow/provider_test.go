package mlflow

import (
	"testing"

	"github.com/aagumin/mlflowpack/internal/cnb"
)

func TestMlflowProvider_Name(t *testing.T) {
	p := &mlflowProvider{}
	if p.Name() != "mlflow" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "mlflow")
	}
}

func TestMlflowProvider_Detect_PassesWithS3Path(t *testing.T) {
	p := &mlflowProvider{}
	appDir := t.TempDir()
	t.Setenv(EnvModelPath, "s3://bucket/models/v1")

	result, err := p.Detect(cnb.DetectContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !result.Pass {
		t.Fatal("Detect() should pass with S3 path")
	}
}

func TestMlflowProvider_Detect_FailsWithoutModel(t *testing.T) {
	p := &mlflowProvider{}
	appDir := t.TempDir()

	result, err := p.Detect(cnb.DetectContext{AppDir: appDir})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Pass {
		t.Fatal("Detect() should fail without model")
	}
}
