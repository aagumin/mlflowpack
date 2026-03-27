// Package storage provides model storage backends for buildpack.
package storage

import (
	"testing"
)

func TestS3StorageParsePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantBucket string
		wantKey    string
	}{
		{
			name:       "simple path",
			path:       "s3://mybucket/models/v1",
			wantBucket: "mybucket",
			wantKey:    "models/v1",
		},
		{
			name:       "nested path",
			path:       "s3://mybucket/path/to/model/v2",
			wantBucket: "mybucket",
			wantKey:    "path/to/model/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewS3StorageFromPath(tt.path)
			if err != nil {
				t.Fatalf("NewS3StorageFromPath() error = %v", err)
			}
			if s.bucket != tt.wantBucket {
				t.Errorf("bucket = %v, want %v", s.bucket, tt.wantBucket)
			}
			if s.keyPrefix != tt.wantKey {
				t.Errorf("keyPrefix = %v, want %v", s.keyPrefix, tt.wantKey)
			}
		})
	}
}

func TestS3StorageImplementsInterface(t *testing.T) {
	var _ Storage = (*S3Storage)(nil)
}
