// Package repo locates the repository the tools read: every tool resolves its
// corpora, baselines and generated files against the product module's root.
package repo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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

// Choose is the given repository, or Root when none is given; like every
// other path flag, a relative one counts from Root rather than the tools module.
func Choose(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	root, err := Root()
	if err != nil || path == "" {
		return root, err
	}
	return filepath.Join(root, path), nil
}

// DevelopCommit is the given commit, or the develop commit the checkout at dir is based on.
func DevelopCommit(dir, given string) (string, error) {
	if given != "" {
		return given, nil
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found on PATH: %w; pass -develop", err)
	}
	out, err := exec.Command(git, "-C", dir, "merge-base", "HEAD", "origin/develop").Output() // #nosec G204 -- git is resolved once by LookPath above
	if err != nil {
		return "", fmt.Errorf("git merge-base HEAD origin/develop: %w; pass -develop", err)
	}
	return strings.TrimSpace(string(out)), nil
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

// Resolve anchors a path flag at root: a relative path counts from the
// repository root, since `go run -C tools` runs every tool from the tools
// module; an absolute or empty path is returned as given.
func Resolve(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// NeedsRoot reports whether Resolve depends on the root for any of paths:
// an empty path takes the root's default and a relative one counts from it.
func NeedsRoot(paths ...string) bool {
	for _, p := range paths {
		if p == "" || !filepath.IsAbs(p) {
			return true
		}
	}
	return false
}
