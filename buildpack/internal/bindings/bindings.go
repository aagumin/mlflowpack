// Package bindings provides utilities for reading CNB service bindings.
package bindings

import (
	"fmt"
	"os"
	"path/filepath"
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
			continue
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
	entries, err := r.readDirectory(s3Path)
	if err != nil {
		return nil, nil
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

		entries[file.Name()] = string(content)
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
