package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Source is a loaded source as the dependency search sees it: the path it was
// read from, which locates the directories searched, and its model text. The
// cells of a notebook are Sources sharing the notebook's path.
type Source struct {
	Path    string
	Content []byte
}

// Dependencies returns the model files, beside or below the named ones, that
// declare a root namespace the named files reference — by an import or by a
// qualified name — and none of them declares, then the files those references
// lead to in turn. known reports the names another source declares — the
// standard library's root packages — so a reference to one is not searched
// for. A named file that cannot be read or is standard input contributes
// nothing; the loader reports the former. The result is ordered as it was
// found and holds no file already named.
func Dependencies(files []string, known func(name string) bool) []string {
	return Discovery{}.Dependencies(files, known)
}

// Dependencies is the package's Dependencies with the discovery's library
// roots: the files are also looked for in the project each named file belongs
// to and in the installed projects that project uses.
func (d Discovery) Dependencies(files []string, known func(name string) bool) []string {
	sources := make([]Source, 0, len(files))
	for _, file := range files {
		if IsStdin(file) {
			continue
		}
		path, content, err := ReadFile(file)
		if err != nil {
			continue
		}
		sources = append(sources, Source{Path: path, Content: content})
	}
	return d.DependenciesOf(sources, known)
}

// DependenciesOf is Dependencies over sources already read, searched from the
// directories their paths lie in — an archive member's, the archive — for the
// model text they load as. Standard input contributes nothing.
func (d Discovery) DependenciesOf(sources []Source, known func(name string) bool) []string {
	loaded := map[string]bool{}
	declared := map[string]bool{}
	wanted := map[string]bool{}
	paths := make([]string, 0, len(sources))
	for _, src := range sources {
		if IsStdin(src.Path) {
			continue
		}
		paths = append(paths, src.Path)
		loaded[absKey(src.Path)] = true
		summarizeContent(src.Path, src.Content).contribute(declared, wanted)
	}
	prune(wanted, declared, known)
	if len(wanted) == 0 {
		return nil
	}

	files, archives := d.searchSpace(d.searchDirs(paths))
	byRoot := rootIndex(files, archives, loaded, d.Guard)
	var out []string
	for len(wanted) > 0 {
		names := make([]string, 0, len(wanted))
		for name := range wanted {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			delete(wanted, name)
			for _, dep := range byRoot[name] {
				if loaded[absKey(dep.path)] {
					continue
				}
				loaded[absKey(dep.path)] = true
				out = append(out, dep.path)
				dep.contribute(declared, wanted)
			}
		}
		prune(wanted, declared, known)
	}
	return out
}

// Declaring returns, for each name, the model files beside and below paths
// and in the projects around them that declare it at their root namespace,
// sorted by path and leaving out paths themselves and the files exclude
// admits — those a session already holds. A name no file declares is absent
// from the result; a file the guard refuses declares nothing.
func (d Discovery) Declaring(paths []string, names []string, exclude func(path string) bool) map[string][]string {
	if len(names) == 0 {
		return nil
	}
	loaded := make(map[string]bool, len(paths))
	for _, path := range paths {
		loaded[absKey(path)] = true
	}
	files, archives := d.searchSpace(d.searchDirs(paths))
	byRoot := rootIndex(files, archives, loaded, d.Guard)
	found := map[string][]string{}
	for _, name := range names {
		if _, done := found[name]; done {
			continue
		}
		for _, dep := range byRoot[name] {
			if exclude == nil || !exclude(dep.path) {
				found[name] = append(found[name], dep.path)
			}
		}
		sort.Strings(found[name])
		if len(found[name]) == 0 {
			delete(found, name)
		}
	}
	return found
}

// searchDirs is the directories the files at paths lie in — an archive
// member's, the archive — from which searchSpace reaches the project each
// belongs to and the projects it uses. Standard input lies nowhere.
func (d Discovery) searchDirs(paths []string) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, path := range paths {
		if IsStdin(path) {
			continue
		}
		dir := filepath.Dir(path)
		if archive, _, ok := SplitMemberPath(path); ok {
			dir = archive
		}
		if key := absKey(dir); !seen[key] {
			seen[key] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// prune drops from wanted every name a loaded file or another source declares.
func prune(wanted, declared map[string]bool, known func(string) bool) {
	for name := range wanted {
		if declared[name] || (known != nil && known(name)) {
			delete(wanted, name)
		}
	}
}

// fileSummary is what a model file declares at its root and which root
// namespaces it references without declaring them — the first segment of an
// import or a qualified name that no enclosing namespace declares; only root
// names are visible to another file's references.
type fileSummary struct {
	path       string
	roots      []string // names declared at the root namespace
	referenced []string // root names the file expects to find outside itself
}

// contribute records the file's root names as declared and the roots it references as wanted.
func (f *fileSummary) contribute(declared, wanted map[string]bool) {
	for _, name := range f.roots {
		declared[name] = true
	}
	for _, name := range f.referenced {
		if !declared[name] {
			wanted[name] = true
		}
	}
}

// summarize parses one file into its summary; ok is false when it cannot be read.
func summarize(path string) (*fileSummary, bool) {
	// #nosec G304 -- the file is one the user named, or one found under its directory.
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return summarizeContent(path, content), true
}

// summarizeContent parses model text read from path into its summary.
func summarizeContent(path string, content []byte) *fileSummary {
	root := parser.New(source.New(path, content)).ParseFile()
	return &fileSummary{path: path, roots: declaredNames(root.Members), referenced: ReferencedRoots(root)}
}

// declaredNames lists the names the members of one namespace declare.
func declaredNames(members []ast.Node) []string {
	var out []string
	for _, m := range members {
		if id, ok := declIdent(unwrapMember(m)); ok {
			out = append(out, namesOf(id)...)
		}
	}
	return out
}

// visible reports whether one of the enclosing namespaces declares name.
func visible(name string, enclosing []map[string]bool) bool {
	for _, scope := range enclosing {
		if scope[name] {
			return true
		}
	}
	return false
}

// namesOf lists the names an identification declares, long and short.
func namesOf(id ast.Identification) []string {
	var out []string
	if id.Name != "" {
		out = append(out, id.Name)
	}
	if id.ShortName != "" {
		out = append(out, id.ShortName)
	}
	return out
}

// unwrapMember strips the visibility wrapper a namespace member is written in.
func unwrapMember(m ast.Node) ast.Node {
	if v, ok := m.(*ast.Membership); ok {
		return v.Member
	}
	return m
}

// declIdent is the identification a declaration that owns members carries.
func declIdent(decl ast.Node) (ast.Identification, bool) {
	switch d := decl.(type) {
	case *ast.Package:
		return d.Ident, true
	case *ast.Namespace:
		return d.Ident, true
	case *ast.Definition:
		return d.Ident, true
	case *ast.Usage:
		return d.Ident, true
	case *ast.Alias:
		return d.Ident, true
	default:
		return ast.Identification{}, false
	}
}

// membersOf is the member list of a namespace-bearing node, nil for any other.
func membersOf(node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.Package:
		return n.Members
	case *ast.Namespace:
		return n.Members
	case *ast.Definition:
		return n.Members
	case *ast.Usage:
		return n.Members
	default:
		return nil
	}
}

// maxDependencyDirs bounds the directories searched for dependencies: the named
// file's directory was not chosen as a project, and may be a home directory.
const maxDependencyDirs = 2000

// searchSpace lists what the dependency search reads for sources under dirs:
// the model files under those directories and under the projects enclosing
// them — a project's own directory is where its imports are meant to be found,
// wherever in it the named files lie — then those of the installed projects
// the enclosing ones use. A walk through an installed project, used or
// enclosing a source, stops at a project nested in it, which is read through
// its own installation; a developer's own tree is walked whole. Archives among
// either are listed apart.
func (d Discovery) searchSpace(dirs []string) (files, archives []string) {
	around := d.Around(dirs)
	budget := maxDependencyDirs
	visited := map[string]bool{}
	walk := func(dir string, skip func(dir string) bool) {
		if IsArchiveFile(dir) {
			archives = append(archives, dir)
			return
		}
		walkTolerant(dir, visited, &budget, IsModelFile, skip, &files)
	}
	// nestedIn is what a walk under dir does not enter: nested projects when
	// the project holding dir is an installed one, nothing otherwise.
	nestedIn := func(dir string) func(dir string) bool {
		if enclosing := EnclosingProjects(dir); len(enclosing) > 0 && around.installed[absKey(enclosing[0])] {
			return IsProjectDir
		}
		return nil
	}
	for _, dir := range dirs {
		walk(dir, nestedIn(dir))
	}
	for _, project := range around.Enclosing {
		walk(project.Dir, nestedIn(project.Dir))
	}
	for _, project := range around.Used {
		walk(project.Dir, IsProjectDir)
	}
	return files, archives
}

// rootIndex maps each root-declared name to the unloaded model files among
// candidates and the members of the archives declaring it, sorted by path.
func rootIndex(candidates, archives []string, loaded map[string]bool, guard func(path string) error) map[string][]*fileSummary {
	sort.Strings(candidates)
	byRoot := map[string][]*fileSummary{}
	seen := map[string]bool{}
	add := func(summary *fileSummary) {
		key := absKey(summary.path)
		if loaded[key] || seen[key] {
			return
		}
		seen[key] = true
		for _, name := range summary.roots {
			byRoot[name] = append(byRoot[name], summary)
		}
	}
	for _, path := range candidates {
		if loaded[absKey(path)] || seen[absKey(path)] || (guard != nil && guard(path) != nil) {
			continue
		}
		if summary, ok := summarize(path); ok {
			add(summary)
		}
	}
	sort.Strings(archives)
	for _, path := range archives {
		if guard != nil && guard(path) != nil {
			continue
		}
		for _, summary := range summarizeArchive(path) {
			add(summary)
		}
	}
	return byRoot
}

// summarizeArchive summarizes the model files of an installed archive, read
// from the archive; one that does not open contributes nothing, as a
// directory that cannot be read does not.
func summarizeArchive(path string) []*fileSummary {
	a, err := OpenArchive(path)
	if err != nil {
		return nil
	}
	var out []*fileSummary
	for _, path := range a.Project.Files() {
		_, member, _ := SplitMemberPath(path)
		content, err := a.Read(member)
		if err != nil {
			continue
		}
		out = append(out, summarizeContent(path, content))
	}
	return out
}

// walkTolerant appends the files under dir that accept admits to out as walk
// does, but passes over whatever cannot be read and stops once budget
// directories are visited; a directory below dir that skip admits is not
// entered.
func walkTolerant(dir string, visited map[string]bool, budget *int, accept func(string) bool, skip func(string) bool, out *[]string) {
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
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path) // resolves a link to what it points at
		if err != nil {
			continue
		}
		switch {
		case info.IsDir():
			if !strings.HasPrefix(e.Name(), ".") && !IsLibraryDir(dir, e.Name()) && (skip == nil || !skip(path)) {
				walkTolerant(path, visited, budget, accept, skip, out)
			}
		case accept(path):
			*out = append(*out, path)
		}
	}
}
