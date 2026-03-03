package main

import (
	"github.com/amazme/aipack/buildpack/internal/build"
	"github.com/buildpacks/libcnb/v2"
)

func main() {
	libcnb.Build(build.Build, libcnb.NewConfig())
}
