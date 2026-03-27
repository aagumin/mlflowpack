// Package deps provides dependency hash computation for layer caching.
package deps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ComputeHash computes a hash of dependency files in the given directory.
// It hashes conda.yaml and requirements.txt if they exist.
func ComputeHash(dir string) (string, error) {
	h := sha256.New()

	// Files to hash, in sorted order for determinism
	files := []string{"conda.yaml", "requirements.txt"}
	sort.Strings(files)

	hasFiles := false
	for _, file := range files {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("reading %s: %w", file, err)
		}

		hasFiles = true
		h.Write([]byte(file + ":"))
		h.Write(data)
		h.Write([]byte("\n"))
	}

	// If no files, hash a constant to represent default dependencies
	if !hasFiles {
		h.Write([]byte("default-deps"))
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
