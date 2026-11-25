package libs

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// archiveSource reads the standard library from project interchange archives
// (.kpar), as the OMG publishes it: each archive's model members are listed
// under the archive's file name, so two archives' members never collide.
type archiveSource struct {
	archives []*project.Archive
	failed   string // the archive that did not open, listed so a read reports err
	err      error
}

// NewArchiveSource returns a Source over the archives named, read into memory
// and validated when the source is made. An archive that is not a project
// interchange file is listed alone and every read fails with its reason, so a
// load reports the archive rather than serve an empty library.
func NewArchiveSource(paths ...string) Source {
	s := &archiveSource{}
	for _, path := range paths {
		a, err := project.OpenArchive(path)
		if err != nil {
			s.failed, s.err = filepath.Base(path), err
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
	if s.err != nil {
		return []string{s.failed}
	}
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
type unionSource struct {
	sources []Source

	mu sync.Mutex
	// owners is the source each file was first listed by, as of the latest
	// listing; nil until one is taken.
	owners map[string]Source
}

// List lists every source afresh, and records which source holds each file
// for the reads and tier lookups that follow.
func (s *unionSource) List() []string {
	owners := map[string]Source{}
	var out []string
	for _, src := range s.sources {
		for _, name := range src.List() {
			if _, seen := owners[name]; !seen {
				owners[name] = src
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	s.mu.Lock()
	s.owners = owners
	s.mu.Unlock()
	return out
}

// owner is the first source listing name. It answers from the latest listing,
// and lists again for a name that one does not hold: a file may have arrived.
func (s *unionSource) owner(name string) (Source, bool) {
	s.mu.Lock()
	src, ok := s.owners[name]
	s.mu.Unlock()
	if ok {
		return src, true
	}
	s.List()
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok = s.owners[name]
	return src, ok
}

func (s *unionSource) Read(name string) ([]byte, error) {
	src, ok := s.owner(name)
	if !ok {
		return nil, fmt.Errorf("libs: no library file %q", name)
	}
	data, err := src.Read(name)
	if err == nil {
		return data, nil
	}
	// The file may have left that source since the listing: ask again.
	s.List()
	if now, ok := s.owner(name); ok && now != src {
		return now.Read(name)
	}
	return nil, err
}

// specLibraries are the libraries the KerML and SysML specifications define,
// named as the archives the OMG publishes name them in .project.json; a SysML
// library the list does not name is a domain library.
var specLibraries = map[string]symbols.LibraryTier{
	"Kernel Semantic Library":  symbols.TierKernelSemantic,
	"Kernel Data Type Library": symbols.TierKernelDataType,
	"Kernel Function Library":  symbols.TierKernelFunction,
	"SysML Systems Library":    symbols.TierSystems,
}

// TierOf is the tier of the archive the file is a member of: the library its
// project is, else a domain library when the project's metamodel is SysML, else
// a library of no stated tier.
func (s *archiveSource) TierOf(name string) symbols.LibraryTier {
	base, _, _ := strings.Cut(filepath.ToSlash(name), "/")
	for _, a := range s.archives {
		if filepath.Base(a.Path) != base {
			continue
		}
		if tier, ok := specLibraries[a.Project.Name]; ok {
			return tier
		}
		if a.Meta.Metamodel == project.MetamodelSysML && strings.HasPrefix(a.Project.Name, "SysML ") {
			return symbols.TierDomain
		}
	}
	return symbols.TierLibrary
}

// TierOf is the tier the source holding the file states for it.
func (s *unionSource) TierOf(name string) symbols.LibraryTier {
	if src, ok := s.owner(name); ok {
		return tierOf(src, name)
	}
	return TierOf(name)
}
