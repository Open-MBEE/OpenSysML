package libs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// archiveSource reads the standard library from project interchange archives
// (.kpar), as the OMG publishes it: each archive's model members are listed
// under the archive's file name, so two archives' members never collide.
type archiveSource struct {
	archives []*project.Archive
	err      error
}

// NewArchiveSource returns a Source over the archives named, read into memory
// and validated when the source is made. An archive that is not a project
// interchange file makes every read fail with its reason rather than serve a
// library with a piece missing.
func NewArchiveSource(paths ...string) Source {
	s := &archiveSource{}
	for _, path := range paths {
		a, err := project.OpenArchive(path)
		if err != nil {
			s.err = err
			return s
		}
		s.archives = append(s.archives, a)
	}
	return s
}

// libraryPathSource returns the source LibraryPathEnvVar names: an archive,
// a directory of archives (read with any .kerml/.sysml files beside them), or
// a directory of library files.
func libraryPathSource(path string) Source {
	if project.IsArchiveFile(path) {
		return NewArchiveSource(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return &dirSource{dir: path}
	}
	var archives []string
	for _, e := range entries {
		if e.Type().IsRegular() && project.IsArchive(e.Name()) {
			archives = append(archives, filepath.Join(path, e.Name()))
		}
	}
	if len(archives) == 0 {
		return &dirSource{dir: path}
	}
	sort.Strings(archives)
	return &unionSource{sources: []Source{NewArchiveSource(archives...), &dirSource{dir: path}}}
}

func (s *archiveSource) List() []string {
	var out []string
	for _, a := range s.archives {
		base := filepath.Base(a.Path)
		for _, name := range a.Names() {
			if strings.HasSuffix(name, ".kerml") || strings.HasSuffix(name, ".sysml") {
				out = append(out, filepath.Join(base, filepath.FromSlash(name)))
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *archiveSource) Read(name string) ([]byte, error) {
	if s.err != nil {
		return nil, fmt.Errorf("libs: %s: %w", name, s.err)
	}
	base, member, found := strings.Cut(filepath.ToSlash(name), "/")
	if !found {
		return nil, fmt.Errorf("libs: %q names no archive member", name)
	}
	for _, a := range s.archives {
		if filepath.Base(a.Path) == base {
			return a.Read(member)
		}
	}
	return nil, fmt.Errorf("libs: no library archive %q", base)
}

// unionSource lists the files of several sources and reads each from the
// first that lists it.
type unionSource struct{ sources []Source }

func (s *unionSource) List() []string {
	seen := map[string]bool{}
	var out []string
	for _, src := range s.sources {
		for _, name := range src.List() {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *unionSource) Read(name string) ([]byte, error) {
	for _, src := range s.sources {
		for _, listed := range src.List() {
			if listed == name {
				return src.Read(name)
			}
		}
	}
	return nil, fmt.Errorf("libs: no library file %q", name)
}
