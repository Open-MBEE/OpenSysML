package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// SetWorkspaceRoot names the directory a typed submission's unresolved root
// namespaces are looked for under: when an import or a qualified reference
// begins with a name neither the session nor the library declares, the model
// file under the root declaring it loads as %load would. The empty string
// looks nowhere, which is how a session starts.
func (s *Session) SetWorkspaceRoot(dir string) error {
	root := ""
	if dir != "" {
		abs, err := filepath.Abs(expandHome(dir))
		if err != nil {
			return fmt.Errorf("workspace root %s: %w", dir, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("workspace root %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("workspace root %s is not a directory", dir)
		}
		root = abs
	}
	defer s.enter()()
	s.workspaceRoot, s.workspace = root, nil
	return nil
}

// WorkspaceRoot is the directory unresolved root namespaces are looked for
// under, empty when they are looked for nowhere.
func (s *Session) WorkspaceRoot() string {
	defer s.reading()()
	return s.workspaceRoot
}

// workspaceIndex is the index over the workspace root's model files, nil
// without a root.
func (s *Session) workspaceIndex() *project.WorkspaceIndex {
	if s.workspaceRoot == "" {
		return nil
	}
	if s.workspace == nil {
		s.workspace = s.discovery().NewWorkspaceIndex(s.workspaceRoot)
	}
	return s.workspace
}

// doWorkspace carries out %workspace: it shows the root, or that there is none.
func (s *Session) doWorkspace() []string {
	if s.workspaceRoot == "" {
		return []string{"no workspace root: an unresolved import is looked for nowhere; %load names the files to load"}
	}
	return []string{fmt.Sprintf("workspace root: %s", s.workspaceRoot),
		"  a root namespace a submission leaves unresolved is loaded from the model file under it that declares the name"}
}

// workspaceLoad is what resolving references did: the files it accepted into
// the current generation, if any, their parses, what they declared, what
// accepting them dropped, and the lines to report about it.
type workspaceLoad struct {
	loaded   bool
	files    []string
	parses   []parsed
	declared []string
	drops    []dropReport
	notices  []string
}

// rootSearch is where a submission's unresolved root namespaces are looked
// for, and whether the files found join the session on demand, as typed
// text's do, or for good, as a load's do.
type rootSearch struct {
	find     func(names []string, exclude func(path string) bool) map[string][]string
	where    string
	onDemand bool
}

// searchFor is the search a submission's unresolved root namespaces get: a
// load's, beside and below its files and in the projects around them; typed
// text's, under the workspace root, and none without one.
func (s *Session) searchFor(load bool, files []SourceFile) *rootSearch {
	if load {
		paths := make([]string, 0, len(files))
		for _, f := range files {
			if path := f.Of; path != "" {
				paths = append(paths, path)
			} else if f.Name != "" {
				paths = append(paths, f.Name)
			}
		}
		d := s.discovery()
		find := func(names []string, exclude func(path string) bool) map[string][]string {
			return d.Declaring(paths, names, exclude)
		}
		return &rootSearch{find: find, where: "beside the loaded files"}
	}
	if ix := s.workspaceIndex(); ix != nil {
		return &rootSearch{find: ix.Declaring, where: "under " + s.workspaceRoot, onDemand: true}
	}
	return nil
}

// resolveReferences loads, for a submission whose text parsed to parses and
// analyzed to res, the files declaring the root namespaces its imports and
// qualified references leave unresolved, found where search looks. Imports
// come first: a qualified name an import would answer is not looked for until
// the imports have been (import Lib::*; then Inner::Thing with Lib::Inner).
func (s *Session) resolveReferences(res Result, parses []parsed, search *rootSearch) workspaceLoad {
	if search == nil {
		return workspaceLoad{}
	}
	var imported, referenced []string
	for _, pre := range parses {
		imported = append(imported, project.ImportedRoots(pre.root)...)
		referenced = append(referenced, project.ReferencedRoots(pre.root)...)
	}
	var subjects []string
	for _, d := range res.Diagnostics {
		if d.Code != resolve.CodeUnresolved || !res.mine(d.Span) {
			continue
		}
		if subject, ok := resolve.UnresolvedSubject(d.Message); ok {
			subjects = append(subjects, subject)
		}
	}
	unresolved := func(name string) bool { return referenceBegins(subjects, name) }
	wanted := s.undeclaredRoots(imported, unresolved)
	if len(wanted) == 0 {
		wanted = s.undeclaredRoots(referenced, unresolved)
	}
	return s.loadDeclaring(wanted, search)
}

// pullReferences carries on from what loading ws did for the text parsed to
// parses: the session is rebuilt over the files loaded, and the root
// namespaces the text and those files leave unresolved in turn are loaded the
// same way, round after round, until a round loads nothing. It reports what
// the rounds did together, and what rebuilding did to the session.
func (s *Session) pullReferences(ws workspaceLoad, parses []parsed, search *rootSearch) (all workspaceLoad, whole []string) {
	for ws.loaded {
		all.loaded = true
		all.files = append(all.files, ws.files...)
		all.declared = append(all.declared, ws.declared...)
		all.drops = append(all.drops, ws.drops...)
		s.recordDrops = append(s.recordDrops, ws.drops...)
		s.idxVersion, s.names = 0, nil
		more := s.rebuildOver(ws.drops)
		whole = append(whole, more...)
		all.notices = append(append(append(all.notices, dropNotices(ws.drops)...), more...), ws.notices...)
		parses = append(parses, ws.parses...)
		ws = s.resolveReferences(s.generationResult(), parses, search)
	}
	all.notices = append(all.notices, ws.notices...)
	return all, whole
}

// generationResult is the result of the current generation as the buffer
// stands, for reading what its text leaves unresolved.
func (s *Session) generationResult() Result {
	own := s.ownSpans()
	offset := s.genOffset(s.joined())
	if at, ok := firstText(own); ok {
		offset = at
	}
	return s.submissionResult(nil, offset, own, nil)
}

// loadWorkspaceForText loads the workspace files declaring the root namespaces
// a prompt expression or command argument names and nothing in the session
// declares, after it failed to evaluate. It is a submission of its own, as a
// %load is; loaded reports whether the buffer changed.
func (s *Session) loadWorkspaceForText(text string) (notices []string, loaded bool) {
	if s.workspaceRoot == "" {
		return nil, false
	}
	p := parser.New(source.New("<repl-input>", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil {
		return nil, false
	}
	var roots []string
	ast.Inspect(expr, func(n ast.Node) bool {
		if qn, ok := n.(*ast.QualifiedName); ok && len(qn.Parts) > 1 && !qn.Parts[1].Chained {
			roots = append(roots, qn.Parts[0].Text)
		}
		return true
	})
	wanted := s.undeclaredRoots(roots, nil)
	if len(wanted) == 0 {
		return nil, false
	}
	s.version++
	search := s.searchFor(false, nil)
	ws := s.loadDeclaring(wanted, search)
	if ws.loaded {
		s.recordDrops = nil
	}
	all, _ := s.pullReferences(ws, nil, search)
	return all.notices, all.loaded
}

// undeclaredRoots keeps the names among roots, each once, that neither the
// buffer nor the library declares at the root and that confirm admits, when
// it is given.
func (s *Session) undeclaredRoots(roots []string, confirm func(name string) bool) []string {
	declared := map[string]bool{}
	for _, sn := range s.snippets {
		for _, name := range sn.names {
			declared[name] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range roots {
		if seen[name] || declared[name] || s.ws.IsLibraryRoot(name) {
			continue
		}
		seen[name] = true
		if confirm == nil || confirm(name) {
			out = append(out, name)
		}
	}
	return out
}

// referenceBegins reports whether one of the unresolved references begins with
// name, written bare or quoted, from the global namespace or not.
func referenceBegins(subjects []string, name string) bool {
	for _, subject := range subjects {
		subject = strings.TrimPrefix(subject, "$::")
		for _, written := range []string{name, suggest.Notation(name)} {
			if subject == written || strings.HasPrefix(subject, written+"::") {
				return true
			}
		}
	}
	return false
}

// loadDeclaring accepts, into the current generation, the model files search
// finds declaring the root namespaces named, as %load accepts them: the one
// file declaring each name. A name two files declare is reported and left to
// the user, as is a file that cannot be read; a name no file declares is left
// as it was.
func (s *Session) loadDeclaring(names []string, search *rootSearch) workspaceLoad {
	var out workspaceLoad
	if search == nil || len(names) == 0 {
		return out
	}
	held := map[string]bool{}
	for _, sn := range s.snippets {
		if sn.key != "" {
			held[sn.key] = true
		}
	}
	found := search.find(names, func(path string) bool { return held[fileKeyOf(path)] })
	var paths []string
	for _, name := range names {
		files := found[name]
		switch len(files) {
		case 0:
		case 1:
			paths = append(paths, files[0])
			out.notices = append(out.notices, fmt.Sprintf("note: loaded %s, which declares %s", s.relative(files[0]), name))
		default:
			rel := make([]string, len(files))
			for i, file := range files {
				rel[i] = s.relative(file)
			}
			out.notices = append(out.notices, fmt.Sprintf("note: %s is declared by more than one file %s: %s; load the one meant with %%load", name, search.where, strings.Join(rel, ", ")))
		}
	}
	if len(paths) == 0 {
		return out
	}
	if err := s.guardFiles(paths, true); err != nil {
		out.notices = append(out.notices, errPrefix+err.Error())
		return out
	}
	read, err := s.readSources(paths, loadOptions{})
	if err != nil {
		out.notices = append(out.notices, errPrefix+err.Error())
		return out
	}
	out.parses = make([]parsed, len(read.files))
	model.ParallelFor(s.jobs, len(read.files), func(i int) {
		out.parses[i] = preparseWithKind(read.files[i].Name, read.files[i].Text, read.files[i].Kind)
	})
	seen := map[string]bool{}
	for i, f := range read.files {
		names, dropped := s.acceptParsed(f.Name, f.key(), fileKeyOf(f.Of), f.Text, out.parses[i], f.Kind)
		s.snippets[len(s.snippets)-1].onDemand = search.onDemand
		if f.Of != "" {
			out.files = append(out.files, f.Of)
		} else {
			out.files = append(out.files, f.Name)
		}
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				out.declared = append(out.declared, name)
			}
		}
		out.drops = append(out.drops, dropped...)
	}
	out.notices = append(out.notices, conversionWarnings(read.files, s.verbosity)...)
	out.loaded = true
	return out
}

// relative is path as a notice names it: relative to the workspace root when
// there is one, as given otherwise.
func (s *Session) relative(path string) string {
	if s.workspaceRoot == "" {
		return path
	}
	return project.Relative(s.workspaceRoot, path)
}
