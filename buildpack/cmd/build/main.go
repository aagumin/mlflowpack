package main

import (
	"fmt"
	"os"

	"github.com/amazme/aipack/buildpack/internal/build"
	"github.com/amazme/aipack/buildpack/internal/cnb"
)

func main() {
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
		fmt.Fprintf(os.Stderr, "ERROR: getting working directory: %v\n", err)
		os.Exit(1)
	}
	ctx.AppDir = wd

	// Run build
	result, err := build.Build(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Write layer TOML files
	for layerName, metadata := range result.Layers {
		if err := cnb.WriteLayerToml(ctx.LayersDir, layerName, metadata); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: writing layer %s: %v\n", layerName, err)
			os.Exit(1)
		}
	}

	// Write launch.toml
	if err := cnb.WriteLaunchToml(ctx.LayersDir, result.Launch); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: writing launch.toml: %v\n", err)
		os.Exit(1)
	}

	// Success - exit 0
}
