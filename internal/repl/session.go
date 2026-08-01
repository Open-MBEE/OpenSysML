// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/model"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// docName is the in-memory workspace key for the accumulated REPL buffer.
const docName = "<repl>"

// snippet is one accepted submission source plus the top-level names it declares.
type snippet struct {
	src   string
	names []string
}

// Session accumulates submissions into a single implicit <repl> document.
type Session struct {
	ws       *model.Workspace
	snippets []snippet
	version  int
	
	// Runtime execution context
	rtCtx     *runtime.Context
	instances map[string]*runtime.Instance // name -> instance for %instantiate tracking
	
	// Active executor sessions for debugging
	actionExec *actionSession
	stateExec  *stateSession
}

// actionSession holds an active action executor debugging session.
type actionSession struct {
	name     string
	symbol   *symbols.Symbol
	executor *runtime.ActionExecutor
}

// stateSession holds an active state machine executor debugging session.
type stateSession struct {
	name     string
	symbol   *symbols.Symbol
	executor *runtime.StateExecutor
}

// NewSession returns a session over a fresh workspace.
func NewSession() *Session {
	return &Session{
		ws:        model.NewWorkspace(),
		instances: make(map[string]*runtime.Instance),
	}
}

// List returns a one-line summary per surviving snippet.
func (s *Session) List() []string {
	out := make([]string, 0, len(s.snippets))
	for _, sn := range s.snippets {
		out = append(out, sn.src)
	}
	return out
}

// accept parses src to compute its declared names, drops any earlier snippet
// whose names intersect, appends the new snippet, and returns the joined
// <repl> content. It does NOT touch the workspace (Task 4 does).
func (s *Session) accept(src string) string {
	root := parser.New(source.New(docName, []byte(src))).ParseFile()
	names := declaredNames(root)
	if len(names) > 0 {
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			if !intersects(sn.names, set) {
				kept = append(kept, sn)
			}
		}
		s.snippets = kept
	}
	s.snippets = append(s.snippets, snippet{src: src, names: names})
	return s.joined()
}

func (s *Session) joined() string {
	parts := make([]string, len(s.snippets))
	for i, sn := range s.snippets {
		parts[i] = sn.src
	}
	return strings.Join(parts, "\n")
}

func intersects(names []string, set map[string]bool) bool {
	for _, n := range names {
		if set[n] {
			return true
		}
	}
	return false
}

// Submit accumulates src into the <repl> document, reindexes and eagerly
// analyzes the whole buffer, and returns a Result. Submissions are always
// accumulated (even with parse errors) so diagnostics are reported against the
// live session context; a later redeclaration of the same name replaces the
// prior snippet (see accept).
func (s *Session) Submit(src string) Result {
	declared := declaredNames(parser.New(source.New(docName, []byte(src))).ParseFile())
	joined := s.accept(src)
	s.version++
	s.ws.Open(docName, []byte(joined), s.version)
	diags := s.ws.Diagnostics(docName)
	var members []ast.Node
	if doc := s.ws.Document(docName); doc != nil && doc.AST != nil {
		members = doc.AST.Members
	}
	return Result{
		Members:     members,
		Declared:    declared,
		Diagnostics: diags,
		Source:      joined,
	}
}

// Clear resets the session, dropping all accumulated declarations.
func (s *Session) Clear() {
	s.ws.Remove(docName)
	s.snippets = nil
	s.version = 0
	s.rtCtx = nil
	s.instances = make(map[string]*runtime.Instance)
	s.actionExec = nil
	s.stateExec = nil
}

// getOrCreateRuntime lazily creates runtime context when first needed.
func (s *Session) getOrCreateRuntime() (*runtime.Context, error) {
	if s.rtCtx != nil {
		return s.rtCtx, nil
	}
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil, fmt.Errorf("no document loaded")
	}
	
	// Build fresh index from current document
	idx := symbols.NewIndex()
	idx.AddDocument(docName, doc.AST)
	
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	s.rtCtx = runtime.NewContext(model, resolver, 100000)
	return s.rtCtx, nil
}
