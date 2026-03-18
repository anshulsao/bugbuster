package workspace

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Dir returns the workspace directory path: ~/.bugbuster/workspace
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".bugbuster", "workspace"), nil
}

// Extract writes all embedded assets to the workspace directory.
// It overwrites existing files so the workspace always matches the binary version.
func Extract(assets embed.FS, version string) (string, error) {
	wsDir, err := Dir()
	if err != nil {
		return "", err
	}

	// Check if already extracted for this version
	versionFile := filepath.Join(wsDir, ".bugbuster-version")
	if data, err := os.ReadFile(versionFile); err == nil {
		if string(data) == version {
			return wsDir, nil // already up to date
		}
	}

	// Extract all embedded files
	err = fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(wsDir, path)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := assets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		// Preserve executable bit for .sh files and Dockerfiles
		perm := os.FileMode(0644)
		if filepath.Ext(path) == ".sh" {
			perm = 0755
		}

		return os.WriteFile(target, data, perm)
	})
	if err != nil {
		return "", fmt.Errorf("extracting workspace: %w", err)
	}

	// Write version marker
	_ = os.WriteFile(versionFile, []byte(version), 0644)

	return wsDir, nil
}
