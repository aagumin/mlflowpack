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
	os.Exit(run())
}

func run() int {
	ctx := cnb.DetectContext{
		PlatformDir:   os.Getenv("CNB_PLATFORM_DIR"),
		BuildPlanPath: os.Getenv("CNB_BUILD_PLAN_PATH"),
		BuildpackDir:  os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:       os.Getenv("CNB_EXEC_ENV"),
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: getting working directory: %v\n", err)
		return cnb.ExitCodeErr
	}
	ctx.AppDir = wd

	p, result, err := provider.DetectFirst(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return cnb.ExitCodeErr
	}

	if !result.Pass {
		return cnb.ExitCodeFail
	}

	// Write build plan with provider name in metadata
	plan := cnb.BuildPlan{
		Provides: []cnb.BuildPlanEntry{
			{Name: "model"},
		},
		Requires: []cnb.BuildPlanEntry{
			{Name: "model", Metadata: map[string]interface{}{"provider": p.Name()}},
		},
	}

	if err := cnb.WriteBuildPlan(ctx.BuildPlanPath, plan); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: writing build plan: %v\n", err)
		return cnb.ExitCodeErr
	}

	return cnb.ExitCodePass
}
