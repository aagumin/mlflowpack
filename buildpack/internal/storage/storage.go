// Package storage provides model storage backends for buildpack.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// _metadataFiles is the list of files to download in metadata-only mode.
var _metadataFiles = []string{"MLmodel", "conda.yaml", "requirements.txt"}

// LocalStorage implements Storage for local filesystem.
type LocalStorage struct {
	path string
}

// NewLocalStorage creates a new LocalStorage.
func NewLocalStorage(path string) *LocalStorage {
	return &LocalStorage{path: path}
}

func (s *LocalStorage) String() string {
	return fmt.Sprintf("local:%s", s.path)
}

func (s *LocalStorage) Exists(ctx context.Context) (bool, error) {
	_, err := os.Stat(s.path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) DownloadMetadata(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	for _, file := range _metadataFiles {
		srcPath := filepath.Join(s.path, file)
		dstPath := filepath.Join(destDir, file)

		if err := copyFile(srcPath, dstPath); err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist in source, skip
				continue
			}
			return fmt.Errorf("copying %s: %w", file, err)
		}
	}

	return nil
}

func (s *LocalStorage) Download(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	return filepath.Walk(s.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.path, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close source file %s: %v\n", src, closeErr)
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close destination file %s: %v\n", dst, closeErr)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// Storage defines the interface for model storage backends.
type Storage interface {
	// Download downloads model files to destDir.
	Download(ctx context.Context, destDir string) error

	// DownloadMetadata downloads only metadata files (MLmodel, conda.yaml, requirements.txt).
	DownloadMetadata(ctx context.Context, destDir string) error

	// Exists checks if the model path exists.
	Exists(ctx context.Context) (bool, error)

	// String returns a human-readable representation of the storage.
	String() string
}

// Verify interface compliance at compile time.
var (
	_ Storage = (*LocalStorage)(nil)
	_ Storage = (*S3Storage)(nil)
)

// ParsePath parses a model path and returns the storage type and normalized path.
// Supported formats:
//   - s3://bucket/path/to/model
//   - /local/path/to/model
//   - file:///local/path/to/model
func ParsePath(path string) (storageType, normalizedPath string, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", fmt.Errorf("model path cannot be empty")
	}

	if strings.HasPrefix(path, "s3://") {
		// Remove s3:// prefix and split into bucket/path
		normalized := strings.TrimPrefix(path, "s3://")
		parts := strings.SplitN(normalized, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			return "", "", fmt.Errorf("invalid s3 path: %s (expected s3://bucket/path)", path)
		}
		return "s3", normalized, nil
	}

	if strings.HasPrefix(path, "file://") {
		return "local", strings.TrimPrefix(path, "file://"), nil
	}

	// Assume local path
	return "local", path, nil
}

// NewStorage creates a Storage instance based on the path scheme.
func NewStorage(path string) (Storage, error) {
	storageType, normalizedPath, err := ParsePath(path)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case "s3":
		return NewS3Storage(normalizedPath)
	case "local":
		return NewLocalStorage(normalizedPath), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}
