package mlflow

import (
	"context"
	"fmt"

	modctlmlflow "github.com/modelpack/modctl/pkg/modelprovider/mlflow"
)

// Downloader wraps modctl's MLflow client for model downloading.
type Downloader struct {
	client *modctlmlflow.MlFlowClient
}

// NewDownloader creates a new downloader using modctl's MLflow client.
func NewDownloader() (*Downloader, error) {
	client, err := modctlmlflow.NewMlFlowRegistry(nil)
	if err != nil {
		return nil, fmt.Errorf("creating mlflow client: %w", err)
	}
	return &Downloader{client: &client}, nil
}

// DownloadModel downloads a model from MLflow registry to destDir.
// Returns the path to the downloaded model.
func (d *Downloader) DownloadModel(ctx context.Context, name, version, destDir string) (string, error) {
	path, err := d.client.PullModelByName(ctx, name, version, destDir)
	if err != nil {
		return "", fmt.Errorf("downloading model %s:%s: %w", name, version, err)
	}
	return path, nil
}
