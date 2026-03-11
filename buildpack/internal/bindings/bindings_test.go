package bindings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMLflowBinding_TrimsLineEndings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlflowDir := filepath.Join(dir, "mlflow-main")
	if err := os.MkdirAll(mlflowDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeBindingFile(t, mlflowDir, "type", "mlflow\n")
	writeBindingFile(t, mlflowDir, "tracking_uri", "https://mlflow.example.com\n")
	writeBindingFile(t, mlflowDir, "username", "user\r\n")
	writeBindingFile(t, mlflowDir, "password", "pass\n")

	reader := NewReader(dir)
	binding, err := reader.ReadMLflowBinding()
	if err != nil {
		t.Fatalf("ReadMLflowBinding returned error: %v", err)
	}
	if binding == nil {
		t.Fatal("ReadMLflowBinding returned nil binding")
	}

	if binding.TrackingURI != "https://mlflow.example.com" {
		t.Fatalf("TrackingURI = %q", binding.TrackingURI)
	}
	if binding.Username != "user" {
		t.Fatalf("Username = %q", binding.Username)
	}
	if binding.Password != "pass" {
		t.Fatalf("Password = %q", binding.Password)
	}
}

func TestReadBindingByType_ReturnsErrorOnBrokenBinding(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	brokenDir := filepath.Join(dir, "a-broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir broken dir: %v", err)
	}
	if err := os.Symlink(filepath.Join(brokenDir, "missing"), filepath.Join(brokenDir, "type")); err != nil {
		t.Fatalf("symlink type: %v", err)
	}

	validDir := filepath.Join(dir, "z-valid")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatalf("mkdir valid dir: %v", err)
	}
	writeBindingFile(t, validDir, "type", "mlflow")

	reader := NewReader(dir)
	_, err := reader.ReadBindingByType(MLflowBindingType)
	if err == nil {
		t.Fatal("expected error for broken binding, got nil")
	}
}

func TestReadS3Binding_ReturnsErrorOnBrokenNestedBinding(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mlflowDir := filepath.Join(dir, "mlflow")
	s3Dir := filepath.Join(mlflowDir, "s3")
	if err := os.MkdirAll(s3Dir, 0o755); err != nil {
		t.Fatalf("mkdir s3 dir: %v", err)
	}

	writeBindingFile(t, mlflowDir, "type", "mlflow")
	if err := os.Symlink(filepath.Join(s3Dir, "missing"), filepath.Join(s3Dir, "endpoint")); err != nil {
		t.Fatalf("symlink endpoint: %v", err)
	}

	reader := NewReader(dir)
	_, err := reader.ReadS3Binding()
	if err == nil {
		t.Fatal("expected error for broken nested s3 binding, got nil")
	}
}

func writeBindingFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
