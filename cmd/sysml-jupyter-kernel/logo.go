package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

// logos are the kernelspec's icons: the OpenSysML mark at the two sizes
// Jupyter front ends read from a kernelspec directory.
//
//go:embed logo-32x32.png logo-64x64.png
var logos embed.FS

// writeLogos puts the icons beside kernel.json in the kernelspec directory.
func writeLogos(dir string) error {
	entries, err := logos.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		raw, err := logos.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		// #nosec G306 -- read by every user's Jupyter, as kernel.json is.
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), raw, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", entry.Name(), err)
		}
	}
	return nil
}
