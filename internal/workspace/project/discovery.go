package project

import (
	"errors"
	"fmt"
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
	"lib",                           // a plain lib/ beside .project.json
	filepath.Join(".sysand", "lib"), // sysand
}

// IsLibraryDir reports whether name, under dir, starts a well-known library
// layout of the project in dir (one holding .project.json): installed, not own, files.
func IsLibraryDir(dir, name string) bool {
	if _, err := os.Stat(filepath.Join(dir, ProjectFile)); err != nil {
		return false
	}
	for _, layout := range wellKnownLibraryDirs {
		if strings.SplitN(layout, string(filepath.Separator), 2)[0] == name {
			return true
		}
	}
	return false
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
	// Unlinked describes the usages that name no installed project and that
	// several unclaimed ones could answer, one line each; see used.
	Unlinked []string
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
	installed, malformed := installedProjects(d.libraryRoots(ancestors))
	out.Malformed = malformed
	if len(out.Enclosing) == 0 {
		out.Used = installed
		return out
	}
	out.Used, out.Unlinked = used(out.Enclosing, installed)
	return out
}

// libraryRoots lists the roots installed projects are looked for under: the
// configured roots (a directory or an archive), then the well-known library
// directories under each project directory; those that exist, each once.
func (d Discovery) libraryRoots(projectDirs []string) []string {
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
// order they were reached; an enclosing project is not among what it uses. A
// usage is answered by the project its resource names (Uses), else, once every
// usage had its say, by elimination: of the installed projects no usage claims —
// a claimed project's other versions count as claimed — those that satisfy its
// constraint, when they are all one project, the highest version of it. A usage
// several unclaimed projects could answer is left unlinked and described, since
// a version-control IRI spells a repository's path, not the project's name; a
// later usage of the same repository takes the project so learnt, at its own
// version. The standard library's usages, which the bundled library answers,
// are not linked to anything installed.
func used(enclosing, installed []*Interchange) (out []*Interchange, unlinked []string) {
	selected := map[string]bool{}
	claimed := map[string]bool{}
	learnt := map[string]string{}
	for _, project := range enclosing {
		selected[absKey(project.Dir)] = true
		claimed[project.identity()] = true
	}
	var queue []*Interchange
	link := func(pick *Interchange) {
		claimed[pick.identity()] = true
		if selected[absKey(pick.Dir)] {
			return
		}
		selected[absKey(pick.Dir)] = true
		out = append(out, pick)
		queue = append(queue, pick)
	}
	type pending struct {
		from *Interchange
		u    Usage
	}
	var unmatched []pending
	queue = append(queue, enclosing...)
	for {
		for len(queue) > 0 {
			project := queue[0]
			queue = queue[1:]
			for _, u := range project.Usage {
				if pick := pickInstalled(installed, u); pick != nil {
					link(pick)
				} else if !u.standardLibrary() {
					unmatched = append(unmatched, pending{project, u})
				}
			}
		}
		if len(unmatched) == 0 {
			break
		}
		var rest []pending
		for _, p := range unmatched {
			if id, ok := learnt[p.u.key()]; ok {
				if pick := pickVersion(installed, id, p.u.constraint()); pick != nil {
					link(pick)
				}
				continue
			}
			pick, candidates := pickUnclaimed(installed, claimed, p.u)
			switch {
			case pick != nil:
				learnt[p.u.key()] = pick.identity()
				link(pick)
			case len(candidates) > 1:
				rest = append(rest, p)
			}
		}
		if len(queue) == 0 {
			for _, p := range rest {
				_, candidates := pickUnclaimed(installed, claimed, p.u)
				unlinked = append(unlinked, ambiguousUsage(p.from, p.u, candidates))
			}
			break
		}
		unmatched = rest
	}
	return out, unlinked
}

// pickVersion chooses, among the installed copies of the project identity
// names, the highest version satisfying constraint (any, when it is empty); nil
// when none does.
func pickVersion(installed []*Interchange, identity, constraint string) *Interchange {
	var pick *Interchange
	for _, candidate := range installed {
		if candidate.identity() != identity || (constraint != "" && !candidate.satisfies(constraint)) {
			continue
		}
		if pick == nil || newerVersion(candidate.Version, pick.Version) {
			pick = candidate
		}
	}
	return pick
}

// pickUnclaimed chooses the installed project a usage is linked to by
// elimination: of the unclaimed ones whose version satisfies the usage's
// constraint (any version, when it has none), the highest version when they are
// all one project. With several projects among them, none is picked and they
// are returned instead; with none, nothing is.
func pickUnclaimed(installed []*Interchange, claimed map[string]bool, u Usage) (*Interchange, []*Interchange) {
	constraint := u.constraint()
	var candidates []*Interchange
	var pick *Interchange
	one := true
	for _, candidate := range installed {
		if claimed[candidate.identity()] || (constraint != "" && !candidate.satisfies(constraint)) {
			continue
		}
		candidates = append(candidates, candidate)
		switch {
		case pick == nil:
			pick = candidate
		case pick.identity() != candidate.identity():
			one = false
		case newerVersion(candidate.Version, pick.Version):
			pick = candidate
		}
	}
	if !one {
		return nil, candidates
	}
	return pick, candidates
}

// ambiguousUsage describes a usage of from that several unclaimed installed
// projects could answer, naming each, and how the resource selects one.
func ambiguousUsage(from *Interchange, u Usage, candidates []*Interchange) string {
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, fmt.Sprintf("%s %s (%s)", c.identity(), c.Version, c.Dir))
	}
	return fmt.Sprintf("%s: usage %s names no installed project by publisher and name, and %d unclaimed ones could answer it: %s; end the resource with the project's publisher and name (…/%s) to select one",
		filepath.Join(from.Dir, ProjectFile), u.Resource, len(candidates), strings.Join(names, ", "), candidates[0].identity())
}

// pickInstalled chooses the installed project a usage names: of those its
// resource names, the one whose version the usage's constraint (its
// versionConstraint, else the @version in its IRI) names exactly, else the
// latest. Nil when none does.
func pickInstalled(installed []*Interchange, u Usage) *Interchange {
	var pick *Interchange
	constraint := u.constraint()
	for _, candidate := range installed {
		if !candidate.Uses(u) {
			continue
		}
		switch {
		case pick == nil:
			pick = candidate
		case pick.satisfies(constraint):
		case candidate.satisfies(constraint), newerVersion(candidate.Version, pick.Version):
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
