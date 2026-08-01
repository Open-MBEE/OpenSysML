// Package libs loads SysML/KerML standard-library files (bundled via embed.FS
// or overridden on disk) and maintains a persistent cache of their indexed
// symbols. See spec section 10.
package libs

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed stdlib
var stdlibFS embed.FS

// Source yields standard-library file contents by relative path
// (e.g. "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml").
type Source interface {
	// List returns the relative paths of available library files, sorted.
	List() []string
	// Read returns the bytes of the named library file, or an error.
	Read(name string) ([]byte, error)
}

// DefaultSource returns a dirSource rooted at SYSML_LIBRARY_PATH when that
// environment variable is set and non-empty, otherwise the embedded source.
func DefaultSource() Source {
	if dir := os.Getenv("SYSML_LIBRARY_PATH"); dir != "" {
		return &dirSource{dir: dir}
	}
	return &embedSource{}
}

// NewDirSource returns a Source that reads .kerml/.sysml files from dir.
func NewDirSource(dir string) Source {
	return &dirSource{dir: dir}
}

type embedSource struct{}

func (s *embedSource) List() []string {
	var out []string
	fs.WalkDir(stdlibFS, "stdlib", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Only include .kerml and .sysml files, skip LICENSE/NOTICE
		if strings.HasSuffix(path, ".kerml") || strings.HasSuffix(path, ".sysml") {
			// Strip "stdlib/" prefix to get relative path
			relPath := strings.TrimPrefix(path, "stdlib/")
			out = append(out, relPath)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (s *embedSource) Read(name string) ([]byte, error) {
	return stdlibFS.ReadFile("stdlib/" + name)
}

type dirSource struct{ dir string }

func (s *dirSource) List() []string {
	var out []string
	filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Only include .kerml and .sysml files
		if strings.HasSuffix(path, ".kerml") || strings.HasSuffix(path, ".sysml") {
			// Get relative path from base dir
			relPath, err := filepath.Rel(s.dir, path)
			if err == nil {
				out = append(out, relPath)
			}
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func (s *dirSource) Read(name string) ([]byte, error) {
	// Allow subdirectories now
	path := filepath.Join(s.dir, name)
	// Security check: ensure path doesn't escape base dir
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.dir)) {
		return nil, fmt.Errorf("libs: invalid library file path %q", name)
	}
	return os.ReadFile(path)
}
