package main

import (
	"fmt"
	"os"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/provider"

	// Register providers via init()
	_ "github.com/aagumin/mlflowpack/internal/mlflow"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := cnb.BuildContext{
		LayersDir:    os.Getenv("CNB_LAYERS_DIR"),
		PlatformDir:  os.Getenv("CNB_PLATFORM_DIR"),
		BpPlanPath:   os.Getenv("CNB_BP_PLAN_PATH"),
		BuildpackDir: os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:      os.Getenv("CNB_EXEC_ENV"),
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	ctx.AppDir = wd

	// Read provider name from buildpack plan
	plan, err := cnb.ReadBuildpackPlan(ctx.BpPlanPath)
	if err != nil {
		return fmt.Errorf("reading buildpack plan: %w", err)
	}

	var providerName string
	for _, entry := range plan.Entries {
		if v, ok := entry.Metadata["provider"].(string); ok {
			providerName = v
			break
		}
	}
	if providerName == "" {
		return fmt.Errorf("build plan does not specify a provider")
	}

	p := provider.ByName(providerName)
	if p == nil {
		return fmt.Errorf("unknown provider: %s", providerName)
	}

	result, err := p.Build(ctx)
	if err != nil {
		return err
	}

	// Write layer TOML files
	for layerName, metadata := range result.Layers {
		if err := cnb.WriteLayerToml(ctx.LayersDir, layerName, metadata); err != nil {
			return fmt.Errorf("writing layer %s: %w", layerName, err)
		}
	}

	// Write launch.toml
	if err := cnb.WriteLaunchToml(ctx.LayersDir, result.Launch); err != nil {
		return fmt.Errorf("writing launch.toml: %w", err)
	}

	return nil
}
