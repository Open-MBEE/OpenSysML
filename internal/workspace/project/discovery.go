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
	// installed keys, by absKey, every project installed under the roots,
	// the enclosing projects among them included.
	installed map[string]bool

	// Linking describes the usages that name no installed project by publisher and name —
	// linked by elimination, or answerable by several unclaimed projects — one warning per usage.
	Linking []Warning
}

// Warning is what a load fell short of about the project or archive at Path.
type Warning struct {
	Path    string
	Message string
}

func (w Warning) String() string { return w.Path + ": " + w.Message }

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
	out.installed = make(map[string]bool, len(installed))
	for _, project := range installed {
		out.installed[absKey(project.Dir)] = true
	}
	if len(out.Enclosing) == 0 {
		out.Used = installed
		return out
	}
	out.Used, out.Linking = used(out.Enclosing, installed, out.installed)
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

// used selects the installed projects the enclosing ones use, directly or in turn, each once: by name (Uses), else — a
// resource spelling a location — by elimination (pickUnclaimed) or as learnt; an enclosing project installed under the roots (installedKey) may answer one.
func used(enclosing, installed []*Interchange, installedKey map[string]bool) (out []*Interchange, linking []Warning) {
	l := &linker{installed: installed, selected: map[string]bool{}, claimed: map[string]bool{}, learnt: map[string]string{}}
	for _, project := range enclosing {
		l.selected[absKey(project.Dir)] = true
		if !installedKey[absKey(project.Dir)] {
			l.claimed[project.identity()] = true
		}
	}
	l.queue = append(l.queue, enclosing...)
	var unmatched []pending
	for {
		unmatched = l.drain(unmatched)
		if len(unmatched) == 0 {
			break
		}
		rest, linked := l.retry(unmatched)
		if !linked {
			// Nothing was claimed this pass, so what deferred each usage still stands.
			for _, p := range rest {
				l.linking = append(l.linking, ambiguousUsage(p.from, p.u, p.candidates))
			}
			break
		}
		unmatched = rest
	}
	return l.out, l.linking
}

// linker is the state of one selection by used: the installed projects, what is selected, claimed
// and learnt so far, the projects still to walk, and the selection and warnings so far.
type linker struct {
	installed []*Interchange
	selected  map[string]bool
	claimed   map[string]bool
	learnt    map[string]string
	queue     []*Interchange
	linked    bool // whether this pass claimed anything, enqueued or already selected
	out       []*Interchange
	linking   []Warning
}

// pending is a usage spelling a location that no installed project answered by name yet.
type pending struct {
	from       *Interchange
	u          Usage
	r          versionRange
	candidates []*Interchange // the unclaimed projects that deferred it, last pass
}

func (l *linker) link(pick *Interchange) {
	l.linked = true
	l.claimed[pick.identity()] = true
	if l.selected[absKey(pick.Dir)] {
		return
	}
	l.selected[absKey(pick.Dir)] = true
	l.out = append(l.out, pick)
	l.queue = append(l.queue, pick)
}

// drain walks the usages of every queued project, linking what an installed project answers by
// name and adding to unmatched what spells a location none does.
func (l *linker) drain(unmatched []pending) []pending {
	for len(l.queue) > 0 {
		project := l.queue[0]
		l.queue = l.queue[1:]
		for _, u := range project.Usage {
			if _, err := u.location(); err != nil {
				l.linking = append(l.linking, malformedResource(project, u, err))
				continue
			}
			r, err := u.versionRange()
			if err != nil {
				l.linking = append(l.linking, malformedConstraint(project, u, err))
				continue
			}
			switch pick, named := pickInstalled(l.installed, u, r); {
			case pick != nil:
				l.link(pick)
			case named != nil:
				l.linking = append(l.linking, unsatisfiedUsage(project, u, named))
			case u.spellsLocation() && !u.standardLibrary():
				unmatched = append(unmatched, pending{from: project, u: u, r: r})
			}
		}
	}
	return unmatched
}

// retry links each unmatched usage as learnt or by elimination; it returns the usages several
// unclaimed candidates still defer and whether the pass claimed anything.
func (l *linker) retry(unmatched []pending) (rest []pending, linked bool) {
	l.linked = false
	for _, p := range unmatched {
		if id, ok := l.learnt[p.u.key()]; ok {
			if pick, versions := pickVersion(l.installed, id, p.r); pick != nil {
				l.link(pick)
			} else {
				l.linking = append(l.linking, unsatisfiedUsage(p.from, p.u, versions))
			}
			continue
		}
		pick, candidates := pickUnclaimed(l.installed, l.claimed, p.from, p.u, p.r)
		switch {
		case pick != nil:
			l.learnt[p.u.key()] = pick.identity()
			l.linking = append(l.linking, eliminationUsage(p.from, p.u, pick))
			l.link(pick)
		case len(candidates) > 1:
			p.candidates = candidates
			rest = append(rest, p)
		}
	}
	return rest, l.linked
}

// pickVersion is the highest installed version of the project identity names that
// lies in r, nil when none does, with every installed version of that project.
func pickVersion(installed []*Interchange, identity string, r versionRange) (*Interchange, []*Interchange) {
	var pick *Interchange
	var versions []*Interchange
	for _, candidate := range installed {
		if candidate.identity() != identity {
			continue
		}
		versions = append(versions, candidate)
		if candidate.satisfies(r) && (pick == nil || newerVersion(candidate.Version, pick.Version)) {
			pick = candidate
		}
	}
	return pick, versions
}

// pickUnclaimed links a usage of from by elimination: the highest unclaimed version in r when the
// candidates are all one project (never from itself); several are returned unpicked.
func pickUnclaimed(installed []*Interchange, claimed map[string]bool, from *Interchange, u Usage, r versionRange) (*Interchange, []*Interchange) {
	var candidates []*Interchange
	var pick *Interchange
	one := true
	for _, candidate := range installed {
		if claimed[candidate.identity()] || candidate.identity() == from.identity() || !candidate.satisfies(r) {
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

// eliminationUsage describes a usage of from linked to pick by elimination,
// naming the project the load took and how the resource would name it.
func eliminationUsage(from *Interchange, u Usage, pick *Interchange) Warning {
	return Warning{
		Path: filepath.Join(from.Dir, ProjectFile),
		Message: fmt.Sprintf("usage %s names no installed project by publisher and name; linked by elimination to %s %s (%s), the one unclaimed installed project that satisfies it; end the resource with the project's publisher and name (…/%s) to name it",
			u.Resource, pick.identity(), pick.Version, pick.Dir, pick.identity()),
	}
}

// ambiguousUsage describes a usage of from that several unclaimed installed
// projects could answer, naming each, and how the resource selects one.
func ambiguousUsage(from *Interchange, u Usage, candidates []*Interchange) Warning {
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, fmt.Sprintf("%s %s (%s)", c.identity(), c.Version, c.Dir))
	}
	return Warning{
		Path: filepath.Join(from.Dir, ProjectFile),
		Message: fmt.Sprintf("usage %s names no installed project by publisher and name, and %d unclaimed ones could answer it: %s; end the resource with the project's publisher and name (…/%s) to select one",
			u.Resource, len(candidates), strings.Join(names, ", "), candidates[0].identity()),
	}
}

// pickInstalled chooses, of the installed projects a usage's resource names (Uses), the
// highest version in r; nil with the projects named when none of them lies in r.
func pickInstalled(installed []*Interchange, u Usage, r versionRange) (pick *Interchange, named []*Interchange) {
	for _, candidate := range installed {
		if !candidate.Uses(u) {
			continue
		}
		named = append(named, candidate)
		if candidate.satisfies(r) && (pick == nil || newerVersion(candidate.Version, pick.Version)) {
			pick = candidate
		}
	}
	if pick != nil {
		return pick, nil
	}
	return nil, named
}

// unsatisfiedUsage describes a usage whose project is installed at versions
// none of which its constraint admits.
func unsatisfiedUsage(from *Interchange, u Usage, installed []*Interchange) Warning {
	versions := make([]string, 0, len(installed))
	for _, p := range installed {
		versions = append(versions, p.Version+" ("+p.Dir+")")
	}
	return Warning{
		Path: filepath.Join(from.Dir, ProjectFile),
		Message: fmt.Sprintf("usage %s asks for version %q of %s, which none of the installed versions satisfies: %s",
			u.Resource, u.constraint(), installed[0].identity(), strings.Join(versions, ", ")),
	}
}

// malformedResource describes a usage whose resource spells a location the grammar does not read.
func malformedResource(from *Interchange, u Usage, err error) Warning {
	return Warning{
		Path:    filepath.Join(from.Dir, ProjectFile),
		Message: fmt.Sprintf("usage %s: %v (a package URL of type generic names a repository as pkg:generic/<name>?vcs_url=<vcs>%%2B<transport>:%%2F%%2F<host>%%2F<path>[%%40<version>][%%23<subpath>], the qualifier percent-encoded)", u.Resource, err),
	}
}

// malformedConstraint describes a usage whose constraint the grammar does not read.
func malformedConstraint(from *Interchange, u Usage, err error) Warning {
	return Warning{
		Path:    filepath.Join(from.Dir, ProjectFile),
		Message: fmt.Sprintf("usage %s: %v (an npm-style semver range is expected: 1.2.3, ^1.2, ~1.2.3, >=1.0.0 <2.0.0, 1.x, 1.0.0 - 2.0.0, a || b)", u.Resource, err),
	}
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
