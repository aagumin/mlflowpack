// Package bindings provides utilities for reading CNB service bindings.
package bindings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultBindingsDir is the default directory for service bindings.
	DefaultBindingsDir = "/bindings"

	// MLflowBindingType is the type for MLflow bindings.
	MLflowBindingType = "mlflow"

	// S3BindingType is the type for S3 bindings.
	S3BindingType = "s3"
)

// Binding represents a service binding.
type Binding struct {
	Name    string
	Type    string
	Path    string
	Entries map[string]string
}

// MLflowBinding contains MLflow connection details.
type MLflowBinding struct {
	TrackingURI string
	Username    string
	Password    string
}

// S3Binding contains S3 connection details.
type S3Binding struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
}

// Reader reads service bindings from the filesystem.
type Reader struct {
	bindingsDir string
}

// NewReader creates a new bindings reader.
func NewReader(bindingsDir string) *Reader {
	return &Reader{
		bindingsDir: bindingsDir,
	}
}

// ReadBinding reads a binding by name.
func (r *Reader) ReadBinding(name string) (*Binding, error) {
	path := filepath.Join(r.bindingsDir, name)

	entries, err := r.readDirectory(path)
	if err != nil {
		return nil, err
	}

	bindingType := entries["type"]

	return &Binding{
		Name:    name,
		Type:    bindingType,
		Path:    path,
		Entries: entries,
	}, nil
}

// ReadBindingByType reads the first binding of a given type.
func (r *Reader) ReadBindingByType(bindingType string) (*Binding, error) {
	entries, err := os.ReadDir(r.bindingsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading bindings directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		binding, err := r.ReadBinding(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading binding %s: %w", entry.Name(), err)
		}

		if binding.Type == bindingType {
			return binding, nil
		}
	}

	return nil, nil
}

// ReadMLflowBinding reads the MLflow binding.
func (r *Reader) ReadMLflowBinding() (*MLflowBinding, error) {
	binding, err := r.ReadBindingByType(MLflowBindingType)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, nil
	}

	return &MLflowBinding{
		TrackingURI: binding.Entries["tracking_uri"],
		Username:    binding.Entries["username"],
		Password:    binding.Entries["password"],
	}, nil
}

// ReadS3Binding reads the S3 binding from a subdirectory of the MLflow binding.
func (r *Reader) ReadS3Binding() (*S3Binding, error) {
	// First try to find a dedicated S3 binding
	binding, err := r.ReadBindingByType(S3BindingType)
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return &S3Binding{
			Endpoint:  binding.Entries["endpoint"],
			AccessKey: binding.Entries["access_key"],
			SecretKey: binding.Entries["secret_key"],
			Region:    binding.Entries["region"],
		}, nil
	}

	// Try to find S3 binding in MLflow binding subdirectory
	mlflowBinding, err := r.ReadBindingByType(MLflowBindingType)
	if err != nil {
		return nil, err
	}
	if mlflowBinding == nil {
		return nil, nil
	}

	s3Path := filepath.Join(mlflowBinding.Path, "s3")
	if _, err := os.Stat(s3Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking nested s3 binding: %w", err)
	}

	entries, err := r.readDirectory(s3Path)
	if err != nil {
		return nil, fmt.Errorf("reading nested s3 binding: %w", err)
	}

	return &S3Binding{
		Endpoint:  entries["endpoint"],
		AccessKey: entries["access_key"],
		SecretKey: entries["secret_key"],
		Region:    entries["region"],
	}, nil
}

func (r *Reader) readDirectory(path string) (map[string]string, error) {
	entries := make(map[string]string)

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", path, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(path, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", filePath, err)
		}

		entries[file.Name()] = strings.TrimRight(string(content), "\r\n")
	}

	return entries, nil
}

// GetBindingsDir returns the bindings directory from environment or default.
func GetBindingsDir() string {
	if dir := os.Getenv("CNB_BINDINGS_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("SERVICE_BINDING_ROOT"); dir != "" {
		return dir
	}
	return DefaultBindingsDir
}

// ReadMLflowBindingWithFallback reads MLflow binding with environment variable fallback.
// Bindings take priority over environment variables.
func (r *Reader) ReadMLflowBindingWithFallback() (*MLflowBinding, error) {
	// Try bindings first
	binding, err := r.ReadMLflowBinding()
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return binding, nil
	}

	// Fallback to environment variables
	return readMLflowFromEnv(), nil
}

// ReadS3BindingWithFallback reads S3 binding with environment variable fallback.
// Bindings take priority over environment variables.
func (r *Reader) ReadS3BindingWithFallback() (*S3Binding, error) {
	// Try bindings first
	binding, err := r.ReadS3Binding()
	if err != nil {
		return nil, err
	}
	if binding != nil {
		return binding, nil
	}

	// Fallback to environment variables
	return readS3FromEnv(), nil
}

func readMLflowFromEnv() *MLflowBinding {
	uri := os.Getenv("MLFLOW_TRACKING_URI")
	if uri == "" {
		uri = os.Getenv("DATABRICKS_HOST")
	}
	if uri == "" {
		return nil
	}

	return &MLflowBinding{
		TrackingURI: uri,
		Username:    os.Getenv("MLFLOW_TRACKING_USERNAME"),
		Password:    os.Getenv("MLFLOW_TRACKING_PASSWORD"),
	}
}

func readS3FromEnv() *S3Binding {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil
	}

	return &S3Binding{
		Endpoint:  os.Getenv("AWS_ENDPOINT_URL"),
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    os.Getenv("AWS_REGION"),
	}
}
