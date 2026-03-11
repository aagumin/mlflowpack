// Package storage provides storage backends for downloading MLflow artifacts.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Backend implements the storage backend for S3-compatible storage.
type S3Backend struct {
	client *s3.Client
}

// S3Config contains S3 configuration.
type S3Config struct {
	Endpoint   string
	AccessKey  string
	SecretKey  string
	Region     string
	DisableSSL bool
}

// NewS3Backend creates a new S3 backend with the given configuration.
func NewS3Backend(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	var opts []func(*config.LoadOptions) error

	// Custom credentials
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	// Region
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// Custom endpoint (for MinIO, etc.)
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &S3Backend{client: client}, nil
}

// Supports returns true if this backend supports the given URI.
func (b *S3Backend) Supports(uri string) bool {
	return strings.HasPrefix(uri, "s3://")
}

// Download downloads artifacts from S3 to the destination directory.
func (b *S3Backend) Download(ctx context.Context, uri, destDir string) error {
	bucket, key, err := parseS3URI(uri)
	if err != nil {
		return err
	}

	// Ensure destination exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// List and download objects
	prefix := strings.TrimSuffix(key, "/")
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	paginator := s3.NewListObjectsV2Paginator(b.client, listInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			if err := b.downloadObject(ctx, bucket, *obj.Key, prefix, destDir); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *S3Backend) downloadObject(ctx context.Context, bucket, key, prefix, destDir string) error {
	// Calculate relative path
	relPath := strings.TrimPrefix(key, prefix)
	relPath = strings.TrimPrefix(relPath, "/")

	if relPath == "" {
		return nil // Skip the prefix itself
	}

	destPath, err := secureJoin(destDir, relPath)
	if err != nil {
		return err
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Download file
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	output, err := b.client.GetObject(ctx, input)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", key, err)
	}
	defer func() {
		if closeErr := output.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing S3 object body %q: %w", key, closeErr))
		}
	}()

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", destPath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing file %q: %w", destPath, closeErr))
		}
	}()

	if _, err := io.Copy(file, output.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", destPath, err)
	}

	return nil
}

func parseS3URI(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parsing S3 URI: %w", err)
	}

	if u.Scheme != "s3" {
		return "", "", fmt.Errorf("not an S3 URI: %s", uri)
	}

	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")
	return bucket, key, nil
}

// FileBackend implements the storage backend for local files.
type FileBackend struct{}

// NewFileBackend creates a new file backend.
func NewFileBackend() *FileBackend {
	return &FileBackend{}
}

// Supports returns true if this backend supports the given URI.
func (b *FileBackend) Supports(uri string) bool {
	return strings.HasPrefix(uri, "file://") || !strings.Contains(uri, "://")
}

// Download copies files from source to destination.
func (b *FileBackend) Download(ctx context.Context, uri, destDir string) error {
	var srcPath string
	if strings.HasPrefix(uri, "file://") {
		srcPath = strings.TrimPrefix(uri, "file://")
	} else {
		srcPath = uri
	}

	// Ensure destination exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Walk and copy
	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		destPath, err := secureJoin(destDir, relPath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing source file %q: %w", src, closeErr))
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// #nosec G703 -- dst is validated by secureJoin before passing to copyFile.
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing destination file %q: %w", dst, closeErr))
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

func secureJoin(baseDir, relPath string) (string, error) {
	cleanRelPath := filepath.Clean(relPath)
	if cleanRelPath == "." {
		return filepath.Clean(baseDir), nil
	}
	if filepath.IsAbs(cleanRelPath) {
		return "", fmt.Errorf("absolute path %q is not allowed", relPath)
	}

	baseDir = filepath.Clean(baseDir)
	destPath := filepath.Join(baseDir, cleanRelPath)

	relativeToBase, err := filepath.Rel(baseDir, destPath)
	if err != nil {
		return "", fmt.Errorf("building destination path: %w", err)
	}
	if relativeToBase == ".." || strings.HasPrefix(relativeToBase, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal detected for %q", relPath)
	}

	return destPath, nil
}

// MultiBackend supports multiple storage backends.
type MultiBackend struct {
	backends []Backend
}

// Backend is the interface for storage backends.
type Backend interface {
	Supports(uri string) bool
	Download(ctx context.Context, uri, destDir string) error
}

// NewMultiBackend creates a multi-backend with the given backends.
func NewMultiBackend(backends ...Backend) *MultiBackend {
	return &MultiBackend{backends: backends}
}

// DefaultBackends creates the default set of backends.
func DefaultBackends(ctx context.Context, s3Config S3Config) (*MultiBackend, error) {
	s3, err := NewS3Backend(ctx, s3Config)
	if err != nil {
		return nil, err
	}

	return NewMultiBackend(s3, NewFileBackend()), nil
}

// Download selects the appropriate backend and downloads.
func (m *MultiBackend) Download(ctx context.Context, uri, destDir string) error {
	for _, backend := range m.backends {
		if backend.Supports(uri) {
			return backend.Download(ctx, uri, destDir)
		}
	}

	return fmt.Errorf("no backend supports URI: %s", uri)
}
