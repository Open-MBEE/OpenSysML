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

// workspaceLoad is what resolving references against the workspace did: the
// files it accepted into the current generation, if any, what they declared,
// what accepting them dropped, and the lines to report about it.
type workspaceLoad struct {
	loaded   bool
	declared []string
	drops    []dropReport
	notices  []string
}

// resolveFromWorkspace loads, for a submission typed at the prompt whose text
// parsed to parses and analyzed to res, the workspace files declaring the root
// namespaces its imports and qualified references leave unresolved.
func (s *Session) resolveFromWorkspace(res Result, parses []parsed) workspaceLoad {
	if s.workspaceRoot == "" {
		return workspaceLoad{}
	}
	var roots []string
	for _, pre := range parses {
		roots = append(roots, project.ReferencedRoots(pre.root)...)
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
	return s.loadWorkspaceFor(s.undeclaredRoots(roots, func(name string) bool { return referenceBegins(subjects, name) }))
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
	ws := s.loadWorkspaceFor(wanted)
	if ws.loaded {
		s.recordDrops = ws.drops
		s.idxVersion, s.names = 0, nil
		notices = append(dropNotices(ws.drops), s.rebuildOver(ws.drops)...)
	}
	return append(notices, ws.notices...), ws.loaded
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
// name, written bare or quoted.
func referenceBegins(subjects []string, name string) bool {
	for _, subject := range subjects {
		for _, written := range []string{name, suggest.Notation(name)} {
			if subject == written || strings.HasPrefix(subject, written+"::") {
				return true
			}
		}
	}
	return false
}

// loadWorkspaceFor accepts, into the current generation, the model files under
// the workspace root declaring the root namespaces named, as %load accepts
// them: the one file declaring each name, with the files its own imports lead
// to. A name two files declare is reported and left to the user, as is a file
// that cannot be read; a name no file declares is left as it was.
func (s *Session) loadWorkspaceFor(names []string) workspaceLoad {
	var out workspaceLoad
	ix := s.workspaceIndex()
	if ix == nil || len(names) == 0 {
		return out
	}
	held := map[string]bool{}
	for _, sn := range s.snippets {
		if sn.key != "" {
			held[sn.key] = true
		}
	}
	found := ix.Declaring(names, func(path string) bool { return held[fileKeyOf(path)] })
	var paths []string
	for _, name := range names {
		files := found[name]
		switch len(files) {
		case 0:
		case 1:
			paths = append(paths, files[0])
			out.notices = append(out.notices, fmt.Sprintf("note: loaded %s, which declares %s", project.Relative(s.workspaceRoot, files[0]), name))
		default:
			rel := make([]string, len(files))
			for i, file := range files {
				rel[i] = project.Relative(s.workspaceRoot, file)
			}
			out.notices = append(out.notices, fmt.Sprintf("note: %s is declared by more than one file under %s: %s; load the one meant with %%load", name, s.workspaceRoot, strings.Join(rel, ", ")))
		}
	}
	if len(paths) == 0 {
		return out
	}
	if err := s.guardFiles(paths, true); err != nil {
		out.notices = append(out.notices, errPrefix+err.Error())
		return out
	}
	// What the buffer declares is known too, so a loaded file's import of a
	// typed package does not pull in a sibling declaring the same name.
	read, err := s.readWithDependencies(paths, loadOptions{}, func(name string) bool {
		return s.ws.IsLibraryRoot(name) || len(s.undeclaredRoots([]string{name}, nil)) == 0
	})
	if err != nil {
		out.notices = append(out.notices, errPrefix+err.Error())
		return out
	}
	if extra := read.files[len(paths):]; len(extra) > 0 {
		rel := make([]string, len(extra))
		for i, file := range extra {
			rel[i] = project.Relative(s.workspaceRoot, file.Name)
		}
		out.notices = append(out.notices, fmt.Sprintf("note: loaded %s, which the loaded files import", strings.Join(rel, ", ")))
	}
	parses := make([]parsed, len(read.files))
	model.ParallelFor(s.jobs, len(read.files), func(i int) {
		parses[i] = preparseWithKind(read.files[i].Name, read.files[i].Text, read.files[i].Kind)
	})
	seen := map[string]bool{}
	for i, f := range read.files {
		names, dropped := s.acceptParsed(f.Name, f.key(), fileKeyOf(f.Of), f.Text, parses[i], f.Kind)
		s.snippets[len(s.snippets)-1].onDemand = true
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
