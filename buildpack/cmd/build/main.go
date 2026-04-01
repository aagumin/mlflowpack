package main

import (
	"fmt"
	"os"

	"github.com/aagumin/mlflowpack/internal/build"
	"github.com/aagumin/mlflowpack/internal/cnb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Build context from environment variables
	ctx := cnb.BuildContext{
		LayersDir:    os.Getenv("CNB_LAYERS_DIR"),
		PlatformDir:  os.Getenv("CNB_PLATFORM_DIR"),
		BpPlanPath:   os.Getenv("CNB_BP_PLAN_PATH"),
		BuildpackDir: os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:      os.Getenv("CNB_EXEC_ENV"),
	}

	// Get app directory (current working directory)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	ctx.AppDir = wd

	// Run build
	result, err := build.Build(ctx)
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
