// Package storage provides model storage backends for buildpack.
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageInterface(t *testing.T) {
	// Test that LocalStorage implements Storage interface
	var _ Storage = (*LocalStorage)(nil)
	// Test that S3Storage implements Storage interface
	var _ Storage = (*S3Storage)(nil)
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "s3 path",
			path:     "s3://bucket/path/to/model",
			wantType: "s3",
			wantPath: "bucket/path/to/model",
			wantErr:  false,
		},
		{
			name:     "local path",
			path:     "/workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantErr:  false,
		},
		{
			name:     "file uri",
			path:     "file:///workspace/model",
			wantType: "local",
			wantPath: "/workspace/model",
			wantErr:  false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPath, err := ParsePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotType != tt.wantType {
					t.Errorf("ParsePath() gotType = %v, want %v", gotType, tt.wantType)
				}
				if gotPath != tt.wantPath {
					t.Errorf("ParsePath() gotPath = %v, want %v", gotPath, tt.wantPath)
				}
			}
		})
	}
}

func TestLocalStorageExists(t *testing.T) {
	// Create temp dir with model files
	tmpDir := t.TempDir()

	// Create MLmodel file
	if err := os.WriteFile(filepath.Join(tmpDir, "MLmodel"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "existing path",
			path: tmpDir,
			want: true,
		},
		{
			name: "non-existing path",
			path: "/nonexistent/path",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewLocalStorage(tt.path)
			got, err := s.Exists(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalStorage.Exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LocalStorage.Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalStorageDownloadMetadata(t *testing.T) {
	// Create source dir with model files
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create metadata files
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt"} {
		if err := os.WriteFile(filepath.Join(srcDir, file), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Create artifact file (should not be copied)
	if err := os.WriteFile(filepath.Join(srcDir, "model.pkl"), []byte("big data"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewLocalStorage(srcDir)
	if err := s.DownloadMetadata(context.Background(), destDir); err != nil {
		t.Fatalf("DownloadMetadata() error = %v", err)
	}

	// Check metadata files exist in dest
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(destDir, file)); err != nil {
			t.Errorf("expected %s to exist in dest", file)
		}
	}

	// Check artifact file does NOT exist in dest
	if _, err := os.Stat(filepath.Join(destDir, "model.pkl")); err == nil {
		t.Error("model.pkl should not be copied in metadata download")
	}
}

func TestLocalStorageDownload(t *testing.T) {
	// Create source dir with model files
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create all files
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt", "model.pkl"} {
		if err := os.WriteFile(filepath.Join(srcDir, file), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := NewLocalStorage(srcDir)
	if err := s.Download(context.Background(), destDir); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	// Check all files exist in dest
	for _, file := range []string{"MLmodel", "conda.yaml", "requirements.txt", "model.pkl"} {
		if _, err := os.Stat(filepath.Join(destDir, file)); err != nil {
			t.Errorf("expected %s to exist in dest", file)
		}
	}
}
