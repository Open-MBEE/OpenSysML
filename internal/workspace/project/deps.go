package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Dependencies returns the model files, beside or below the named ones, that
// declare a root namespace an import of the named files begins with and none of
// them declares, then the files those imports lead to in turn. known reports
// the names another source declares — the standard library's root packages —
// so an import of one is not searched for. A named file that cannot be read or
// is standard input contributes nothing; the loader reports the former. The
// result is ordered as it was found and holds no file already named.
func Dependencies(files []string, known func(name string) bool) []string {
	loaded := map[string]bool{}
	declared := map[string]bool{}
	wanted := map[string]bool{}
	var dirs []string
	seenDir := map[string]bool{}
	for _, file := range files {
		if IsStdin(file) {
			continue
		}
		loaded[absKey(file)] = true
		dir := filepath.Dir(file)
		if key := absKey(dir); !seenDir[key] {
			seenDir[key] = true
			dirs = append(dirs, dir)
		}
		if summary, ok := summarize(file); ok {
			summary.contribute(declared, wanted)
		}
	}
	prune(wanted, declared, known)
	if len(wanted) == 0 {
		return nil
	}

	byRoot := rootIndex(dirs, loaded)
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

// prune drops from wanted every name a loaded file or another source declares.
func prune(wanted, declared map[string]bool, known func(string) bool) {
	for name := range wanted {
		if declared[name] || (known != nil && known(name)) {
			delete(wanted, name)
		}
	}
}

// fileSummary is what a model file declares and what its imports begin with.
type fileSummary struct {
	path        string
	roots       []string // names declared at the root namespace
	declared    []string // names declared at any depth
	importRoots []string // first segment of every import
}

// contribute records the file's names as declared and its import roots as wanted.
func (f *fileSummary) contribute(declared, wanted map[string]bool) {
	for _, name := range f.declared {
		declared[name] = true
	}
	for _, name := range f.importRoots {
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
	root := parser.New(source.New(path, content)).ParseFile()
	f := &fileSummary{path: path}
	for _, m := range root.Members {
		if id, ok := declIdent(unwrapMember(m)); ok {
			f.roots = append(f.roots, namesOf(id)...)
		}
	}
	f.collect(root.Members)
	return f, true
}

// collect walks members at every depth for declared names and import roots.
func (f *fileSummary) collect(members []ast.Node) {
	for _, m := range members {
		decl := unwrapMember(m)
		if imp, ok := decl.(*ast.Import); ok {
			if imp.Imported != nil && len(imp.Imported.Parts) > 0 {
				f.importRoots = append(f.importRoots, imp.Imported.Parts[0].Text)
			}
			f.collect(imp.Body)
			continue
		}
		if id, ok := declIdent(decl); ok {
			f.declared = append(f.declared, namesOf(id)...)
		}
		f.collect(membersOf(decl))
	}
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

// rootIndex maps each root-declared name to the model files under dirs that
// declare it, sorted by path, files already loaded left out. Hidden directories
// and entries that cannot be read are passed over, and the search stops once it
// has visited maxDependencyDirs directories.
func rootIndex(dirs []string, loaded map[string]bool) map[string][]*fileSummary {
	var candidates []string
	budget := maxDependencyDirs
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if path != dir && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				if budget == 0 {
					return fs.SkipAll
				}
				budget--
				return nil
			}
			if IsModelFile(path) {
				candidates = append(candidates, path)
			}
			return nil
		})
	}
	sort.Strings(candidates)
	byRoot := map[string][]*fileSummary{}
	seen := map[string]bool{}
	for _, path := range candidates {
		key := absKey(path)
		if loaded[key] || seen[key] {
			continue
		}
		seen[key] = true
		summary, ok := summarize(path)
		if !ok {
			continue
		}
		for _, name := range summary.roots {
			byRoot[name] = append(byRoot[name], summary)
		}
	}
	return byRoot
}
