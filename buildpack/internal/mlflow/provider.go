package mlflow

import (
	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/provider"
)

type mlflowProvider struct{}

func (p *mlflowProvider) Name() string { return "mlflow" }

func (p *mlflowProvider) Detect(ctx cnb.DetectContext) (cnb.DetectResult, error) {
	return Detect(ctx)
}

func (p *mlflowProvider) Build(ctx cnb.BuildContext) (cnb.BuildResult, error) {
	return Build(ctx)
}

func init() {
	provider.Register(&mlflowProvider{})
}
