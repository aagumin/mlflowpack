// Package storage provides model storage backends for buildpack.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage for S3 backend.
type S3Storage struct {
	client    *s3.Client
	bucket    string
	keyPrefix string
}

// NewS3Storage creates a new S3Storage from bucket/key format.
func NewS3Storage(bucketKey string) (*S3Storage, error) {
	parts := strings.SplitN(bucketKey, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid s3 path format: %s", bucketKey)
	}
	return &S3Storage{
		bucket:    parts[0],
		keyPrefix: parts[1],
	}, nil
}

// NewS3StorageFromPath creates a new S3Storage from full s3:// URL.
func NewS3StorageFromPath(url string) (*S3Storage, error) {
	path := strings.TrimPrefix(url, "s3://")
	return NewS3Storage(path)
}

// InitClient initializes the S3 client. Must be called before Download methods.
func (s *S3Storage) InitClient(ctx context.Context) error {
	if s.client != nil {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}
	s.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.DisableLogOutputChecksumValidationSkipped = true
	})
	return nil
}

func (s *S3Storage) String() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.keyPrefix)
}

func (s *S3Storage) Exists(ctx context.Context) (bool, error) {
	if err := s.InitClient(ctx); err != nil {
		return false, err
	}

	// Check if MLmodel exists
	key := s.keyPrefix + "/MLmodel"
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check for NotFound error
		return false, nil
	}
	return true, nil
}

func (s *S3Storage) DownloadMetadata(ctx context.Context, destDir string) error {
	if err := s.InitClient(ctx); err != nil {
		return err
	}

	for _, file := range metadataFiles {
		key := s.keyPrefix + "/" + file
		if err := s.downloadFile(ctx, key, destDir, file); err != nil {
			// File doesn't exist, skip
			continue
		}
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, destDir string) error {
	if err := s.InitClient(ctx); err != nil {
		return err
	}

	// First pass: collect all objects and calculate total size
	type objectInfo struct {
		key     string
		relPath string
		size    int64
	}
	var objects []objectInfo
	var totalSize int64

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.keyPrefix + "/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing s3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			relPath := strings.TrimPrefix(*obj.Key, s.keyPrefix+"/")
			if relPath == "" {
				continue
			}
			objects = append(objects, objectInfo{
				key:     *obj.Key,
				relPath: relPath,
				size:    *obj.Size,
			})
			totalSize += *obj.Size
		}
	}

	fmt.Printf("  Model: s3://%s/%s\n", s.bucket, s.keyPrefix)
	fmt.Printf("  Files: %d objects, total size: %s\n", len(objects), formatSize(totalSize))
	fmt.Println("  Downloading:")

	// Second pass: download with progress
	var downloadedSize int64
	for i, obj := range objects {
		if err := s.downloadFileWithProgress(ctx, obj.key, destDir, obj.relPath, obj.size, i+1, len(objects)); err != nil {
			return fmt.Errorf("downloading %s: %w", obj.key, err)
		}
		downloadedSize += obj.size
	}

	fmt.Printf("  Download complete: %s total\n", formatSize(downloadedSize))
	return nil
}

func (s *S3Storage) downloadFile(ctx context.Context, key, destDir, relPath string) error {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dstPath := filepath.Join(destDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, resp.Body)
	return err
}

func (s *S3Storage) downloadFileWithProgress(ctx context.Context, key, destDir, relPath string, size int64, fileNum, totalFiles int) error {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dstPath := filepath.Join(destDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Show file info with progress
	fmt.Printf("    [%d/%d] %s (%s)\n", fileNum, totalFiles, relPath, formatSize(size))

	_, err = io.Copy(dstFile, resp.Body)
	return err
}

// formatSize converts bytes to human-readable format
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
