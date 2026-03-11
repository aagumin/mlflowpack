package mlflow

import (
	"testing"
)

func TestNewDownloader(t *testing.T) {
	// This test verifies the downloader can be created
	// Actual download tests require MLflow server
	downloader, err := NewDownloader()
	if err != nil {
		t.Fatalf("NewDownloader() error = %v", err)
	}
	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}
}
