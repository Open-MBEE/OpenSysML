// Package repo locates the repository the tools read: every tool resolves its
// corpora, baselines and generated files against the product module's root.
package repo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Module is the product module path; the go.mod declaring it marks the root.
const Module = "github.com/Open-MBEE/OpenSysML"

// Root walks up from the working directory to the directory whose go.mod
// declares Module. The tools module's own go.mod sits below it and is skipped.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return RootFrom(dir)
}

// RootFrom is Root walking up from dir instead of the working directory.
func RootFrom(dir string) (string, error) {
	for {
		if declares(filepath.Join(dir, "go.mod"), Module) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod declaring %s above the working directory", Module)
		}
		dir = parent
	}
}

// declares reports whether the go.mod at path names module.
func declares(path, module string) bool {
	file, err := os.Open(path) // #nosec G304 -- the path is a go.mod on the way up from the working directory
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`) == module
		}
	}
	return false
}
