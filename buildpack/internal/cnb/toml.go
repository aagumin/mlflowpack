package cnb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// WriteLayerToml writes a layer's metadata to <layersDir>/<layerName>.toml.
func WriteLayerToml(layersDir, layerName string, metadata LayerMetadata) (err error) {
	path := filepath.Join(layersDir, layerName+".toml")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing %q: %w", path, closeErr))
		}
	}()

	return toml.NewEncoder(f).Encode(metadata)
}

// WriteLaunchToml writes the launch.toml file to <layersDir>/launch.toml.
func WriteLaunchToml(layersDir string, launch LaunchTOML) (err error) {
	// Only write if there's something to write
	if len(launch.Processes) == 0 && len(launch.Labels) == 0 {
		return nil
	}

	path := filepath.Join(layersDir, "launch.toml")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing %q: %w", path, closeErr))
		}
	}()

	return toml.NewEncoder(f).Encode(launch)
}

// ReadLayerToml reads a layer's metadata from <layersDir>/<layerName>.toml.
// Returns empty LayerMetadata if file doesn't exist.
func ReadLayerToml(layersDir, layerName string) (LayerMetadata, error) {
	path := filepath.Join(layersDir, layerName+".toml")

	var metadata LayerMetadata
	_, err := toml.DecodeFile(path, &metadata)
	if os.IsNotExist(err) {
		return LayerMetadata{}, nil
	}
	if err != nil {
		return LayerMetadata{}, err
	}

	return metadata, nil
}
