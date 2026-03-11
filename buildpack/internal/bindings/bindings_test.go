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

func TestReadMLflowBindingWithFallback_EnvVars(t *testing.T) {
	// Setup temp dir for bindings (empty)
	tempDir := t.TempDir()
	reader := NewReader(tempDir)

	// Set env vars
	t.Setenv("MLFLOW_TRACKING_URI", "https://mlflow.example.com")
	t.Setenv("MLFLOW_TRACKING_USERNAME", "testuser")
	t.Setenv("MLFLOW_TRACKING_PASSWORD", "testpass")

	binding, err := reader.ReadMLflowBindingWithFallback()
	if err != nil {
		t.Fatalf("ReadMLflowBindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadMLflowBindingWithFallback() returned nil")
	}
	if binding.TrackingURI != "https://mlflow.example.com" {
		t.Errorf("TrackingURI = %q, want %q", binding.TrackingURI, "https://mlflow.example.com")
	}
	if binding.Username != "testuser" {
		t.Errorf("Username = %q, want %q", binding.Username, "testuser")
	}
	if binding.Password != "testpass" {
		t.Errorf("Password = %q, want %q", binding.Password, "testpass")
	}
}

func TestReadMLflowBindingWithFallback_BindingsPriority(t *testing.T) {
	// Setup temp dir with binding
	tempDir := t.TempDir()
	mlflowDir := filepath.Join(tempDir, "mlflow")
	if err := os.MkdirAll(mlflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlflowDir, "type"), []byte("mlflow"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mlflowDir, "tracking_uri"), []byte("https://binding.example.com"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set env vars (should be ignored)
	t.Setenv("MLFLOW_TRACKING_URI", "https://env.example.com")

	reader := NewReader(tempDir)
	binding, err := reader.ReadMLflowBindingWithFallback()
	if err != nil {
		t.Fatalf("ReadMLflowBindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadMLflowBindingWithFallback() returned nil")
	}
	// Bindings should take priority
	if binding.TrackingURI != "https://binding.example.com" {
		t.Errorf("TrackingURI = %q, want %q (from bindings)", binding.TrackingURI, "https://binding.example.com")
	}
}

func TestReadS3BindingWithFallback_EnvVars(t *testing.T) {
	tempDir := t.TempDir()
	reader := NewReader(tempDir)

	// Set env vars
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com")

	binding, err := reader.ReadS3BindingWithFallback()
	if err != nil {
		t.Fatalf("ReadS3BindingWithFallback() error = %v", err)
	}
	if binding == nil {
		t.Fatal("ReadS3BindingWithFallback() returned nil")
	}
	if binding.AccessKey != "test-access-key" {
		t.Errorf("AccessKey = %q, want %q", binding.AccessKey, "test-access-key")
	}
	if binding.SecretKey != "test-secret-key" {
		t.Errorf("SecretKey = %q, want %q", binding.SecretKey, "test-secret-key")
	}
	if binding.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", binding.Region, "us-west-2")
	}
	if binding.Endpoint != "https://s3.example.com" {
		t.Errorf("Endpoint = %q, want %q", binding.Endpoint, "https://s3.example.com")
	}
}
