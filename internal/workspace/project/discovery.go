package project

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
)

// LibraryRootsEnvVar names the directories installed projects are looked for
// under, separated as PATH is, besides the well-known ones beneath each
// enclosing project.
const LibraryRootsEnvVar = "OPENSYSML_LIBRARY_ROOTS"

// wellKnownLibraryDirs is where package managers install the projects a project
// uses, relative to the project's directory: one entry per layout. The layout
// is the only thing provider-specific here; what is installed there is read as
// the project interchange format describes it.
var wellKnownLibraryDirs = []string{
	LibraryDir,                      // sysml -install
	filepath.Join(".sysand", "lib"), // sysand
}

// maxLibraryDirs bounds the directories walked under one library root looking
// for installed projects.
const maxLibraryDirs = 2000

// Discovery locates the projects a project uses among the installed copies
// under its library roots. The zero value consults the well-known layouts only.
type Discovery struct {
	// Roots are the directories installed projects are looked for under,
	// besides the well-known ones beneath each enclosing project.
	Roots []string
	// Guard, when set, is asked before a file found by the dependency search is
	// read; a file it refuses is left out as if it declared nothing.
	Guard func(path string) error
}

// EnvLibraryRoots returns the library roots LibraryRootsEnvVar names, none when
// it is unset.
func EnvLibraryRoots() []string {
	return SplitLibraryRoots(envvar.Lookup(LibraryRootsEnvVar))
}

// SplitLibraryRoots splits a list of library roots written as PATH is, dropping
// empty entries.
func SplitLibraryRoots(list string) []string {
	var out []string
	for _, root := range filepath.SplitList(list) {
		if root = strings.TrimSpace(root); root != "" {
			out = append(out, root)
		}
	}
	return out
}

// Projects is what a discovery found around a set of paths: the project each
// path belongs to and the installed projects those use.
type Projects struct {
	// Enclosing holds the nearest project at or above each path, each once.
	Enclosing []*Interchange
	// Used holds the installed projects the enclosing ones use, directly or
	// through the projects those use in turn; with no enclosing project, every
	// project installed under the configured roots.
	Used []*Interchange
	// Malformed holds the archives found under the roots that could not be
	// read as project interchange files, each reported under its path.
	Malformed []*ArchiveError
}

// Dirs lists the directories of the enclosing and the used projects, in that order.
func (p Projects) Dirs() []string {
	out := make([]string, 0, len(p.Enclosing)+len(p.Used))
	for _, project := range p.Enclosing {
		out = append(out, project.Dir)
	}
	for _, project := range p.Used {
		out = append(out, project.Dir)
	}
	return out
}

// Around finds the project each path belongs to — the nearest directory at or
// above it holding a project interchange file — and the installed projects
// those use. Installed projects are looked for under the configured roots and
// under the well-known library directories of every project at or above the
// paths, so a file inside an installed project finds the projects installed
// beside it. A usage no installed project answers to, the standard library's
// archives among them, selects nothing.
func (d Discovery) Around(paths []string) Projects {
	var nearest, ancestors []string
	seenNearest := map[string]bool{}
	seenAncestor := map[string]bool{}
	for _, path := range paths {
		if IsStdin(path) {
			continue
		}
		dirs := EnclosingProjects(path)
		if len(dirs) == 0 {
			continue
		}
		if !seenNearest[dirs[0]] {
			seenNearest[dirs[0]] = true
			nearest = append(nearest, dirs[0])
		}
		for _, dir := range dirs {
			if !seenAncestor[dir] {
				seenAncestor[dir] = true
				ancestors = append(ancestors, dir)
			}
		}
	}
	var out Projects
	for _, dir := range nearest {
		if IsArchiveFile(dir) {
			if a, err := openCached(dir); err == nil {
				out.Enclosing = append(out.Enclosing, a.Project)
			}
			continue
		}
		if project, err := ReadInterchange(dir); err == nil {
			out.Enclosing = append(out.Enclosing, project)
		}
	}
	locked := lockedPaths(out.Enclosing)
	installed, malformed := installedProjects(d.libraryRoots(ancestors, locked))
	out.Malformed = malformed
	if len(out.Enclosing) == 0 {
		out.Used = installed
		return out
	}
	out.Used = used(out.Enclosing, installed, locked)
	return out
}

// lockedPaths reads the lock of each enclosing project directory and maps
// every usage resource it pins to the path its project is read from, so a
// usage is matched to the project the installer put there; the first
// project to pin a resource wins. A lock that cannot be read pins nothing.
func lockedPaths(enclosing []*Interchange) map[string]string {
	locked := map[string]string{}
	for _, project := range enclosing {
		if project.IsArchived() {
			continue
		}
		lock, err := ReadLock(project.Dir)
		if err != nil {
			continue
		}
		for i := range lock.Entries {
			e := &lock.Entries[i]
			if _, dup := locked[e.Resource]; !dup {
				locked[e.Resource] = lock.AbsPath(e)
			}
		}
	}
	return locked
}

// libraryRoots lists the roots installed projects are looked for under: the
// configured roots (a directory or an archive), then the well-known library
// directories under each project directory, then the paths the locks pin
// (a local usage read in place lies outside every library directory); those
// that exist, each once.
func (d Discovery) libraryRoots(projectDirs []string, locked map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(root string) {
		key := absKey(root)
		if seen[key] {
			return
		}
		seen[key] = true
		if info, err := os.Stat(root); err == nil && (info.IsDir() || IsArchiveFile(root)) {
			out = append(out, root)
		}
	}
	for _, root := range d.Roots {
		add(root)
	}
	for _, dir := range projectDirs {
		for _, layout := range wellKnownLibraryDirs {
			add(filepath.Join(dir, layout))
		}
	}
	for _, resource := range sortedKeys(locked) {
		add(locked[resource])
	}
	return out
}

// installedProjects lists the projects under the roots: a root that is itself
// a project or an archive, else every directory below it that holds a project
// interchange file and every .kpar file; each project once however many paths
// lead to it, sorted by path. The walk stops at a project, passes over hidden
// directories and whatever cannot be read, and is bounded. An archive that is
// not a project interchange file is returned apart, under its path.
func installedProjects(roots []string) ([]*Interchange, []*ArchiveError) {
	var out []*Interchange
	var malformed []*ArchiveError
	seen := map[string]bool{}
	found := func(path string) {
		key := resolvedKey(path)
		if seen[key] {
			return
		}
		seen[key] = true
		if IsArchive(path) {
			a, err := openCached(path)
			if err != nil {
				var ae *ArchiveError
				if errors.As(err, &ae) {
					malformed = append(malformed, ae)
				}
				return
			}
			out = append(out, a.Project)
			return
		}
		if project, err := ReadInterchange(path); err == nil {
			out = append(out, project)
		}
	}
	for _, root := range roots {
		if IsProjectDir(root) || IsArchiveFile(root) {
			found(root)
			continue
		}
		budget := maxLibraryDirs
		walkInstalled(root, map[string]bool{}, &budget, found)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	sort.Slice(malformed, func(i, j int) bool { return malformed[i].Path < malformed[j].Path })
	return out, malformed
}

// walkInstalled calls found for each project directory and archive under dir, stopping at a
// project, a hidden or seen directory or the budget; those directly under dir come before any descent.
func walkInstalled(dir string, visited map[string]bool, budget *int, found func(path string)) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || visited[resolved] || *budget == 0 {
		return
	}
	visited[resolved] = true
	*budget--
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var below []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path) // resolves a link to what it points at
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if info.Mode().IsRegular() && IsArchive(path) {
				found(path)
			}
			continue
		}
		if IsProjectDir(path) {
			found(path)
			continue
		}
		below = append(below, path)
	}
	for _, path := range below {
		walkInstalled(path, visited, budget, found)
	}
}

// used selects, among the installed projects, those the enclosing projects use,
// directly or through the usages of the projects selected, each once and in the
// order they were reached; an enclosing project is not among what it uses.
func used(enclosing, installed []*Interchange, locked map[string]string) []*Interchange {
	selected := map[string]bool{}
	for _, project := range enclosing {
		selected[absKey(project.Dir)] = true
	}
	var out []*Interchange
	queue := append([]*Interchange(nil), enclosing...)
	for len(queue) > 0 {
		project := queue[0]
		queue = queue[1:]
		for _, u := range project.Usage {
			pick := pickLocked(installed, u, locked)
			if pick == nil {
				pick = pickInstalled(installed, u)
			}
			if pick == nil || selected[absKey(pick.Dir)] {
				continue
			}
			selected[absKey(pick.Dir)] = true
			out = append(out, pick)
			queue = append(queue, pick)
		}
	}
	return out
}

// pickLocked chooses the installed project the lock pins a usage to: the one
// read from the path the lock records for the usage's resource. Nil when the
// lock pins nothing for it or nothing is installed there, in which case the
// usage is matched by name instead.
func pickLocked(installed []*Interchange, u Usage, locked map[string]string) *Interchange {
	path, ok := locked[u.Resource]
	if !ok {
		return nil
	}
	key := resolvedKey(path)
	for _, candidate := range installed {
		if resolvedKey(candidate.Dir) == key {
			return candidate
		}
	}
	return nil
}

// pickInstalled chooses the installed project a usage names: of those its
// resource names (the publisher and name as slugs), the one whose version
// satisfies the constraint, else the latest. Nil when none does.
func pickInstalled(installed []*Interchange, u Usage) *Interchange {
	var pick *Interchange
	for _, candidate := range installed {
		if !candidate.Uses(u) {
			continue
		}
		switch {
		case pick == nil:
			pick = candidate
		case pick.satisfies(u.VersionConstraint):
		case candidate.satisfies(u.VersionConstraint), newerVersion(candidate.Version, pick.Version):
			pick = candidate
		}
	}
	return pick
}

// projectFiles lists the loadable files of each project — model files and
// notebooks, as a directory named on the command line contributes — each walk
// tolerant of what it cannot read and sorted by path, the projects in order.
func projectFiles(projects []*Interchange) []string {
	var out []string
	for _, project := range projects {
		out = append(out, project.Files()...)
	}
	return out
}

// resolvedKey is the identity a directory is counted under: its path with
// every link resolved, so a project reached by two paths is one project.
func resolvedKey(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return absKey(dir)
}
