package main

import (
	"fmt"
	"os"

	"github.com/aagumin/mlflowpack/internal/cnb"
	"github.com/aagumin/mlflowpack/internal/detect"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Build context from environment variables
	ctx := cnb.DetectContext{
		PlatformDir:   os.Getenv("CNB_PLATFORM_DIR"),
		BuildPlanPath: os.Getenv("CNB_BUILD_PLAN_PATH"),
		BuildpackDir:  os.Getenv("CNB_BUILDPACK_DIR"),
		ExecEnv:       os.Getenv("CNB_EXEC_ENV"),
	}

	// Get app directory (current working directory)
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: getting working directory: %v\n", err)
		return cnb.ExitCodeErr
	}
	ctx.AppDir = wd

	// Run detection
	result, err := detect.Detect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return cnb.ExitCodeErr
	}

	if !result.Pass {
		// Standard "fail" exit code - not an error, just doesn't match
		return cnb.ExitCodeFail
	}

	return cnb.ExitCodePass
}
