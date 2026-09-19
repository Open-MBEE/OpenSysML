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

	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs/errata"
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

// LibraryPathEnvVar names the directory to load the standard library from
// instead of the embedded copy. The legacy SYSML_LIBRARY_PATH name remains
// accepted; the OPENSYSML_ name wins when both are set.
const LibraryPathEnvVar = "OPENSYSML_LIBRARY_PATH"

// DefaultSource returns a dirSource rooted at LibraryPathEnvVar when that
// environment variable is set and non-empty, otherwise BundledSource. A
// directory override is read as it stands: its text is the caller's to correct.
func DefaultSource() Source {
	if dir := envvar.Lookup(LibraryPathEnvVar); dir != "" {
		return &dirSource{dir: dir}
	}
	return BundledSource()
}

// EmbeddedSource returns the standard library exactly as OMG published it,
// whatever LibraryPathEnvVar says. It is the text the declared errata are
// verified against; BundledSource is what a process loads.
func EmbeddedSource() Source {
	return &embedSource{}
}

// BundledSource returns the standard library a process loads when no directory
// overrides it: the published text with the corrections declared in the errata
// package beside it applied on read. The embedded snapshot is generated from it.
// A registry that fails to load makes every read fail rather than serve the
// published text uncorrected.
func BundledSource() Source {
	overlay, err := errata.Library()
	if err != nil {
		return &failingSource{published: EmbeddedSource(), err: err}
	}
	return overlay.LibrarySource(EmbeddedSource())
}

// failingSource lists the published files and fails to read any of them.
type failingSource struct {
	published Source
	err       error
}

func (s *failingSource) List() []string { return s.published.List() }

func (s *failingSource) Read(name string) ([]byte, error) {
	return nil, fmt.Errorf("libs: %s: errata registry: %w", name, s.err)
}

// NewDirSource returns a Source that reads .kerml/.sysml files from dir.
func NewDirSource(dir string) Source {
	return &dirSource{dir: dir}
}

type embedSource struct{}

func (s *embedSource) List() []string {
	var out []string
	// The walk function returns errors only to stop the walk; an unreadable
	// embedded FS is a build-time impossibility.
	_ = fs.WalkDir(stdlibFS, "stdlib", func(path string, d fs.DirEntry, err error) error {
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
	// A directory that cannot be walked yields no library files, which callers
	// handle as an empty source.
	_ = filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
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
	// Subdirectories are allowed; escaping the base directory is not. A prefix
	// test would accept a sibling whose name merely starts with the base ("/libs"
	// against "/libs-evil"), so containment is decided on path elements.
	path := filepath.Join(s.dir, name)
	rel, err := filepath.Rel(s.dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, fmt.Errorf("libs: invalid library file path %q", name)
	}
	// #nosec G304 -- path is confined to s.dir by the check above.
	return os.ReadFile(path)
}
