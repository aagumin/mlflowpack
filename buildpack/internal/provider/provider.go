// Package provider defines the interface for model registry providers
// and a registry for runtime provider lookup.
package provider

import (
	"github.com/aagumin/mlflowpack/internal/cnb"
)

// Provider implements detect and build for a specific model registry/format.
type Provider interface {
	// Name returns the provider identifier (e.g. "mlflow", "clearml").
	Name() string

	// Detect checks if this provider can handle the given context.
	Detect(ctx cnb.DetectContext) (cnb.DetectResult, error)

	// Build executes the build phase for this provider's model.
	Build(ctx cnb.BuildContext) (cnb.BuildResult, error)
}

// registry holds all registered providers in registration order.
var registry []Provider

// Register adds a provider to the registry.
func Register(p Provider) {
	registry = append(registry, p)
}

// All returns all registered providers.
func All() []Provider {
	return registry
}

// ByName returns the provider with the given name, or nil if not found.
func ByName(name string) Provider {
	for _, p := range registry {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// DetectFirst iterates providers in registration order and returns
// the first one that passes detect. If none pass, returns (nil, DetectResult{Pass: false}, nil).
func DetectFirst(ctx cnb.DetectContext) (Provider, cnb.DetectResult, error) {
	for _, p := range registry {
		result, err := p.Detect(ctx)
		if err != nil {
			return nil, cnb.DetectResult{}, err
		}
		if result.Pass {
			return p, result, nil
		}
	}
	return nil, cnb.DetectResult{Pass: false}, nil
}
