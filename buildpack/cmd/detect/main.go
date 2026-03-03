package main

import (
	"github.com/amazme/aipack/buildpack/internal/detect"
	"github.com/buildpacks/libcnb/v2"
)

func main() {
	libcnb.Detect(detect.Detect, libcnb.NewConfig())
}
