package project

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// WorkspaceIndex maps the names declared at the root namespace of the model
// files under a directory — and of the installed projects the project
// interchange file there, or above it, uses — to the files declaring them. A
// file is parsed once and again only when its size or modification time
// changed, so asking the index after every submission costs a directory walk
// and a stat per file, not a parse.
type WorkspaceIndex struct {
	discovery Discovery
	root      string
	mu        sync.Mutex
	// entries holds what each model file or archive under the root was last
	// read as, by path.
	entries map[string]*indexEntry
}

// indexEntry is one file's or archive's contribution and the stamp it was read at.
type indexEntry struct {
	size    int64
	modTime time.Time
	roots   []rootDeclaration
}

// rootDeclaration is the root names one model file declares: the file itself,
// or one member of an archive.
type rootDeclaration struct {
	path  string
	names []string
}

// NewWorkspaceIndex is an index over the model files under root, found as the
// discovery finds the dependencies of a loaded file.
func (d Discovery) NewWorkspaceIndex(root string) *WorkspaceIndex {
	return &WorkspaceIndex{discovery: d, root: root, entries: map[string]*indexEntry{}}
}

// Root is the directory the index is over.
func (ix *WorkspaceIndex) Root() string { return ix.root }

// Declaring returns, for each name, the model files under the root declaring
// it at their root namespace, sorted by path and leaving out those exclude
// admits — the files a session already holds. A name no file declares is
// absent from the result. Hidden directories are not searched, and a file the
// discovery's guard refuses declares nothing.
func (ix *WorkspaceIndex) Declaring(names []string, exclude func(path string) bool) map[string][]string {
	if len(names) == 0 {
		return nil
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.refresh()
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	found := map[string][]string{}
	for _, entry := range ix.entries {
		for _, decl := range entry.roots {
			if exclude != nil && exclude(decl.path) {
				continue
			}
			for _, name := range decl.names {
				if wanted[name] {
					found[name] = append(found[name], decl.path)
				}
			}
		}
	}
	for name := range found {
		sort.Strings(found[name])
	}
	return found
}

// refresh brings the entries up to date with the files under the root: new
// and changed files are read, and files gone are dropped.
func (ix *WorkspaceIndex) refresh() {
	files, archives := ix.discovery.searchSpace([]string{ix.root})
	live := make(map[string]bool, len(files)+len(archives))
	for _, path := range files {
		live[path] = true
		ix.update(path, func(path string) []rootDeclaration {
			summary, ok := summarize(path)
			if !ok {
				return nil
			}
			return []rootDeclaration{{path: path, names: summary.roots}}
		})
	}
	for _, path := range archives {
		live[path] = true
		ix.update(path, func(path string) []rootDeclaration {
			var out []rootDeclaration
			for _, summary := range summarizeArchive(path) {
				out = append(out, rootDeclaration{path: summary.path, names: summary.roots})
			}
			return out
		})
	}
	for path := range ix.entries {
		if !live[path] {
			delete(ix.entries, path)
		}
	}
}

// update reads path through read unless the entry held for it was read from a
// file of the same size and modification time.
func (ix *WorkspaceIndex) update(path string, read func(path string) []rootDeclaration) {
	if guard := ix.discovery.Guard; guard != nil && guard(path) != nil {
		delete(ix.entries, path)
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		delete(ix.entries, path)
		return
	}
	if entry := ix.entries[path]; entry != nil && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		return
	}
	ix.entries[path] = &indexEntry{size: info.Size(), modTime: info.ModTime(), roots: read(path)}
}

// ReferencedRoots lists the first segment of every import and every qualified
// name written in root that no enclosing namespace declares — the root
// namespaces the text expects to find outside itself — each once, in the
// order first written.
func ReferencedRoots(root *ast.RootNamespace) []string {
	return rootsReferenced(root, false)
}

// ImportedRoots is ReferencedRoots over the imports alone: the root namespaces
// the text's imports expect to find outside itself. An import is resolved
// before the names it makes visible, so these are looked for first.
func ImportedRoots(root *ast.RootNamespace) []string {
	return rootsReferenced(root, true)
}

func rootsReferenced(root *ast.RootNamespace, importsOnly bool) []string {
	if root == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	referencedRoots(root.Members, nil, func(name string, imported bool) {
		if (imported || !importsOnly) && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	})
	return out
}

// referencedRoots walks members as collect does, reporting the first segments
// of imports and of qualified names at every depth that enclosing — the
// namespaces around them, outermost first — does not declare, and which it is.
func referencedRoots(members []ast.Node, enclosing []map[string]bool, report func(name string, imported bool)) {
	scope := map[string]bool{}
	for _, name := range declaredNames(members) {
		scope[name] = true
	}
	enclosing = append(enclosing, scope)
	for _, m := range members {
		decl := unwrapMember(m)
		if imp, ok := decl.(*ast.Import); ok {
			if imp.Imported != nil && len(imp.Imported.Parts) > 0 {
				if name := imp.Imported.Parts[0].Text; imp.Imported.Global || !visible(name, enclosing) {
					report(name, true)
				}
			}
			// The filter expression names what it filters by from outside too.
			if imp.FilterExpr != nil {
				qualifiedRoots(imp.FilterExpr, nil, enclosing, report)
			}
			referencedRoots(imp.Body, enclosing, report)
			continue
		}
		inner := membersOf(decl)
		qualifiedRoots(decl, inner, enclosing, report)
		if len(inner) > 0 {
			referencedRoots(inner, enclosing, report)
		}
	}
}

// qualifiedRoots reports the first segment of every qualified name in node's
// own text — a type, value or expression naming something outside it — that
// enclosing does not declare, not descending into the members excluded.
func qualifiedRoots(node ast.Node, members []ast.Node, enclosing []map[string]bool, report func(name string, imported bool)) {
	excluded := make(map[ast.Node]bool, len(members))
	for _, n := range members {
		excluded[n] = true
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if excluded[n] {
			return false
		}
		if qn, ok := n.(*ast.QualifiedName); ok && len(qn.Parts) > 1 && !qn.Parts[1].Chained {
			if name := qn.Parts[0].Text; qn.Global || !visible(name, enclosing) {
				report(name, false)
			}
		}
		return true
	})
}

// Relative spells path as a reader beside root would: relative to it when it
// lies under it, else as given.
func Relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
		return path
	}
	return rel
}
