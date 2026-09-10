// Package repl implements an interactive SysML v2 read-eval-print loop as a
// thin frontend over model.Workspace (spec §13).
package repl

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// docName is the in-memory workspace key for the accumulated REPL buffer.
// Text loaded from a .kerml file keeps that file's language: it is masked out
// of docName and analyzed in kermlDocName, whose name carries the KerML kind
// the parser's file-kind gates read. Both documents span the same joined
// buffer byte for byte, so every offset locates the same snippet in either.
const docName = "<repl>"

// kermlDocName is the workspace key for the buffer's KerML text.
const kermlDocName = "<repl>.kerml"

// parseDocName is the document a snippet from origin is parsed and analyzed
// in, which carries the kind of the file it was loaded from.
func parseDocName(origin string) string {
	if source.KindOf(origin) == source.KindKerML {
		return kermlDocName
	}
	return docName
}

// snippet is one accepted submission source, the top-level names it declares,
// and the file it was loaded from, so a finding about it can be reported where
// its reader would look for it. The key is that file itself, identifying it
// across the ways one path can be written.
type snippet struct {
	src   string
	names []string
	// origin is the file this snippet was read from, empty for a submission
	// typed at the prompt, and key identifies that file across the ways its path
	// can be written.
	origin string
	key    string
	// gen is the submission that appended this snippet, which is what scopes a
	// report to the files just loaded rather than the whole buffer.
	gen int
	// prefix is the bytes of earlier comment lines folded into src, which are
	// not part of what was submitted.
	prefix int
	// own locates, within src, the text a merge added to a namespace already in
	// the buffer. It is empty for a snippet that is wholly its submission's.
	own []source.Span
	// open marks a submission that left a brace, comment or quoted name open, so
	// it absorbs the text after it. Such a snippet is kept for %list and %save but
	// masked out of the analyzed buffer, and diags carries what its own parse
	// found, mapped as the workspace maps a document's.
	open  bool
	diags []passes.Diagnostic
}

// Session accumulates submissions into a single implicit <repl> document.
type Session struct {
	// mu serializes the session's exported entry points, which a frontend may
	// call from more than one goroutine: readline answers Tab from its own input
	// goroutine while the loop is still evaluating the previous line. Exported
	// methods take it; lower-case helpers assume the caller holds it.
	mu sync.Mutex

	ws       *model.Workspace
	snippets []snippet
	version  int

	// Runtime execution context
	rtCtx *runtime.Context
	// exhibits indexes the documents' exhibited-state declarations for rtCtx;
	// a different context invalidates it (see exhibitEntries).
	exhibits *exhibitIndex
	// replaced is a context a debugging session still runs against, whose identity
	// sequence the context built next takes over.
	replaced   *runtime.Context
	idx        *symbols.Index               // index over the session document, shared by lookup and runtime
	libSource  libs.Source                  // the library files idx holds, for their spans' text
	idxVersion int                          // document version idx holds, 0 when it holds none
	names      *nameTable                   // simple names of the documents, rebuilt when their scope trees change
	instances  map[string]*runtime.Instance // FQN -> instance for %instantiate tracking
	unnamed    []unnamedObject              // objects a later %instantiate of their name displaced, still addressed by id

	// argMemo and nameMemo hold what command text parsed to, so a repeated
	// invocation is evaluated without being parsed again.
	argMemo  parseMemo[parsedArgs]
	nameMemo parseMemo[parsedName]

	// Active executor sessions for debugging
	actionExec *actionSession
	stateExec  *stateSession

	// Why the debugging sessions and the instances are gone, when a submission
	// ended them. A command that finds nothing active reports this rather than
	// leaving the user to guess.
	endedAction *endedSession
	endedState  *endedSession
	// lost records the objects the session no longer holds, and what took them.
	lost instanceLoss

	// materializeFailures are the feature values a command of this session reported it
	// could not materialize, which a non-interactive run exits on.
	materializeFailures []error

	// notedBlocker identifies the unresolved error the session has already
	// reported as blocking the deeper checks, so it is named once rather than on
	// every submission after it.
	notedBlocker blockerNote

	// trace records execution steps while tracing is on, nil otherwise.
	trace *runtime.TraceRecorder

	// budgets bounds every runtime context this session creates.
	budgets runtime.Budgets

	// schedule is the policy runs started from here on resolve choice points under.
	schedule runtime.SchedulePolicy

	// engines answers every check, run, exploration, sweep and solve the session
	// makes, dispatching each to the engine that covers it.
	engines *analysis.Registry
	// engine is the selection every question is put to the engines under.
	engine analysis.Selection

	verbosity Verbosity

	// renderWidth is the width a text rendering's table is written to fit, 0 for
	// as wide as its widest cell.
	renderWidth int
}

// unnamedObject is an object a later %instantiate of its name displaced.
type unnamedObject struct {
	fqn string
	obj *runtime.Instance
}

// heldObjects counts the objects the session holds, named and displaced alike.
func (s *Session) heldObjects() int {
	return len(s.instances) + len(s.unnamed)
}

// actionSession holds an active action executor debugging session.
type actionSession struct {
	name string
	// fqn is the debugged action's qualified name. Superseding it, or a namespace
	// it lives in, rewrites the graph being run and ends the session.
	fqn string
	// selfFQN names the object performing the behavior, empty when it performs
	// outside any object. Losing that object ends the session with it.
	selfFQN  string
	symbol   *symbols.Symbol
	executor *runtime.ActionExecutor
	// rtCtx is the context the executor runs in. A submission rebuilds the
	// session's own, so results are read against the one that produced them.
	rtCtx *runtime.Context
}

// contextOf returns the context the executor's values belong to, nil for none.
func (a *actionSession) contextOf() *runtime.Context {
	if a == nil {
		return nil
	}
	return a.rtCtx
}

// fqnOf returns the debugged action's qualified name, or "" when no session runs.
func (a *actionSession) fqnOf() string {
	if a == nil {
		return ""
	}
	return a.fqn
}

// selfOf returns the name of the performing object, or "" for none.
func (a *actionSession) selfOf() string {
	if a == nil {
		return ""
	}
	return a.selfFQN
}

// release lets go of the session's run, ending any work a breakpoint left
// paused; a nil session has none.
func (a *actionSession) release() {
	if a != nil {
		a.executor.Release()
	}
}

// stateSession holds an active state machine executor debugging session.
type stateSession struct {
	name string
	// fqn is the debugged state machine's qualified name; see actionSession.fqn.
	fqn string
	// selfFQN names the performing object; see actionSession.selfFQN.
	selfFQN  string
	symbol   *symbols.Symbol
	executor *runtime.StateExecutor
	// machine names the exhibited machine the session is attached to, and
	// machineAt is its position among the object's exhibited machines, so a
	// restart rebinds to the same one when the object exhibits several.
	machine   string
	machineAt int
	// rtCtx is the context the executor runs in; see actionSession.rtCtx. Its
	// clock is the session's time.
	rtCtx *runtime.Context
}

// release lets go of a detached run the session started, so the clock drives it
// no further; a machine an object exhibits runs on, and a nil session has none.
func (s *stateSession) release() {
	if s != nil && s.machine == "" {
		s.executor.Release()
	}
}

// contextOf returns the context the executor's values belong to, nil for none.
func (s *stateSession) contextOf() *runtime.Context {
	if s == nil {
		return nil
	}
	return s.rtCtx
}

// fqnOf returns the debugged state machine's qualified name, or "" when no
// session runs.
func (s *stateSession) fqnOf() string {
	if s == nil {
		return ""
	}
	return s.fqn
}

// selfOf returns the name of the performing object, or "" for none.
func (s *stateSession) selfOf() string {
	if s == nil {
		return ""
	}
	return s.selfFQN
}

// NewSession returns a session over a fresh workspace.
func NewSession() *Session {
	return &Session{
		ws:        model.NewWorkspace(),
		instances: make(map[string]*runtime.Instance),
		budgets:   runtime.DefaultBudgets(),
		engines:   analysis.Default(),
		verbosity: VerbosityNormal,
	}
}

// SetBudgets sets the bounds for runtime contexts created from here on, dropping
// the current one with its objects and the debuggers driving it, which the next
// command reports. It errors on a non-positive bound, which no run could make
// progress under.
func (s *Session) SetBudgets(budgets runtime.Budgets) error {
	if err := budgets.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budgets = budgets
	s.rtCtx, s.replaced = nil, nil
	if n := s.heldObjects(); n > 0 {
		s.lost = lossOnBudgets(n)
	}
	s.instances = make(map[string]*runtime.Instance)
	s.unnamed = nil
	s.endDebugSessions(boundsChanged)
	return nil
}

// endDebugSessions ends both debuggers for a cause outside any submission,
// recording it for the next debugging command.
func (s *Session) endDebugSessions(cause string) {
	if s.actionExec != nil {
		s.endedAction = &endedSession{kind: "action", name: s.actionExec.name, outside: cause}
		s.actionExec.release()
		s.actionExec = nil
	}
	if s.stateExec != nil {
		s.endedState = &endedSession{kind: "state machine", name: s.stateExec.name, outside: cause}
		s.stateExec.release()
		s.stateExec = nil
	}
}

// Budgets returns the bounds this session gives its runtime contexts.
func (s *Session) Budgets() runtime.Budgets {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.budgets
}

// List returns a one-line summary per surviving snippet.
func (s *Session) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list()
}

func (s *Session) list() []string {
	out := make([]string, 0, len(s.snippets))
	for _, sn := range s.snippets {
		out = append(out, sn.src)
	}
	return out
}

// accept accepts src as a submission of its own, from origin when it came from a
// file.
func (s *Session) accept(origin, src string) {
	s.version++
	s.acceptFrom(origin, src)
}

// acceptFrom parses src to compute its declared names, drops any snippet from
// an earlier submission whose names intersect, and appends the new snippet under
// the current submission generation. It does NOT touch the workspace (Submit
// does). It returns the names src declares as written, whether or not the
// snippet recorded for it declares them.
//
// Only earlier submissions are replaced: redeclaring a name supersedes what was
// submitted before it, but two files of one load are both part of the model, so
// a name they share is a conflict for the analysis to report rather than a
// reason to drop one of them.
//
// A submission that declares names takes over the comment lines typed just
// before it. They document what follows, so folding them into the same snippet
// makes a later redeclaration replace the comments along with the declaration
// instead of leaving stale documentation above whatever is current.
//
// A loaded file supersedes only itself and what the prompt said about the same
// names, since several files of one model commonly open the same package.
func (s *Session) acceptFrom(origin, src string) (declared []string, drops []dropReport) {
	p := parser.New(source.New(parseDocName(origin), []byte(src)))
	root := p.ParseFile()
	names := declaredNames(root)
	declared = names
	text := src
	// A submission that does not close its own text is masked out of the buffer
	// rather than left to absorb the submissions after it, and declares nothing:
	// what the parser recovered from it is not what was meant.
	if !closesItsOwnText(parseDocName(origin), src) {
		key := fileKeyOf(origin)
		if key != "" {
			// Re-reading the file supersedes what it declared before, which it no
			// longer does.
			kept := s.snippets[:0]
			for _, sn := range s.snippets {
				if sn.key == key {
					drops = append(drops, dropReport{gone: sn.names})
					continue
				}
				kept = append(kept, sn)
			}
			s.snippets = kept
		}
		s.snippets = append(s.snippets, snippet{
			src:    src,
			origin: origin,
			key:    key,
			gen:    s.version,
			open:   true,
			diags:  parseDiagnostics(p),
		})
		return declared, drops
	}
	var (
		comments  string
		mergedOwn []source.Span
	)
	if origin != "" {
		set := nameSet(names)
		key := fileKeyOf(origin)
		top := topLevelMembers(root)
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			switch {
			case sn.key == key:
				// Re-reading the same file is a refresh the load reports
				// itself, so only what it rewrote is recorded.
				drops = append(drops, dropReport{gone: sn.names})
			case sn.origin == "" && intersects(sn.names, set):
				// The file supersedes what was typed about the same names,
				// which is a loss to report like any other.
				drops = append(drops, replacedReport(sn, set, top))
			default:
				kept = append(kept, sn)
			}
		}
		s.snippets = append(kept, snippet{src: src, names: names, origin: origin, key: key, gen: s.version})
		return declared, append(drops, s.reopenedNamespaces(key, root)...)
	}
	if len(names) > 0 {
		set := nameSet(names)
		comments = s.takeLeadingComments()
		// Re-typing a namespace adds to the one already in the buffer. The
		// merged text stands for both — including any other declaration of the
		// snippet it absorbed, so the names it replaces are its own, not just
		// the submitted ones — and is appended like any other submission so a
		// report still scopes to the tail of the buffer.
		if merged, added, drop, ok := s.mergeSubmission(src, root, comments); ok {
			text, comments, mergedOwn = merged, "", added
			names = declaredNames(parser.New(source.New(docName, []byte(merged))).ParseFile())
			drops = append(drops, drop)
		}
		top := topLevelMembers(root)
		kept := s.snippets[:0]
		for _, sn := range s.snippets {
			if sn.gen == s.version || !intersects(sn.names, set) {
				kept = append(kept, sn)
				continue
			}
			drops = append(drops, replacedReport(sn, set, top))
		}
		s.snippets = kept
	}
	// The prefix marks the comments folded in front of what was submitted, so
	// diagnostics keep the line numbers of the submission. A merge folded the
	// comments into the text it rewrote, where own marks what was added.
	s.snippets = append(s.snippets, snippet{
		src:    comments + text,
		names:  names,
		origin: origin,
		gen:    s.version,
		prefix: len(comments),
		own:    mergedOwn,
	})
	return declared, drops
}

// origins locates every file of the current submission in the joined buffer, in
// buffer order, so a diagnostic can be reported against the file it came from.
func (s *Session) origins() []Origin {
	var out []Origin
	acc := 0
	for _, sn := range s.snippets {
		if sn.gen == s.version && sn.origin != "" {
			out = append(out, Origin{Name: sn.origin, Offset: acc + sn.prefix})
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return out
}

// ownSpans locates, in the joined buffer, the text this submission added to a
// namespace a merge folded it into, so its report covers what was typed rather
// than the whole snippet the merge absorbed.
func (s *Session) ownSpans() []source.Span {
	var (
		out    []source.Span
		merged bool
		acc    int
	)
	for _, sn := range s.snippets {
		if sn.gen == s.version {
			if len(sn.own) == 0 {
				// Appended whole, so all of it past the folded comments is this
				// submission's.
				out = append(out, source.Span{Offset: acc + sn.prefix, Len: len(sn.src) - sn.prefix})
			} else {
				merged = true
			}
			for _, sp := range sn.own {
				out = append(out, source.Span{Offset: acc + sp.Offset, Len: sp.Len})
			}
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	if !merged {
		// Nothing was merged, so the tail-of-buffer rule already describes the
		// submission and needs no spans.
		return nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	return out
}

// firstText returns the offset of the first span with text in it, which is where
// a submission's own text begins; the zero-length spans only mark a declaration
// a merge folded into.
func firstText(spans []source.Span) (int, bool) {
	for _, sp := range spans {
		if sp.Len > 0 {
			return sp.Offset, true
		}
	}
	return 0, false
}

// genOffset returns the byte offset in the joined buffer where the current
// submission begins: the start of its first surviving snippet, past any comment
// lines folded in front of it, so diagnostics keep the line numbers submitted.
func (s *Session) genOffset(joined string) int {
	acc := 0
	for _, sn := range s.snippets {
		if sn.gen == s.version {
			return acc + sn.prefix
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return len(joined)
}

// takeLeadingComments removes the trailing run of comment-only snippets typed at
// the prompt and returns their text, ready to prefix the declaration they
// document. A comment-only file is a file in its own right, so it stays.
func (s *Session) takeLeadingComments() string {
	cut := len(s.snippets)
	// An open comment is not documentation to fold in front of a declaration: it
	// would swallow the declaration it was folded in front of.
	for cut > 0 && s.snippets[cut-1].origin == "" && !s.snippets[cut-1].open && isCommentOnly(s.snippets[cut-1].src) {
		cut--
	}
	if cut == len(s.snippets) {
		return ""
	}
	var b strings.Builder
	for _, sn := range s.snippets[cut:] {
		b.WriteString(sn.src)
		b.WriteString("\n")
	}
	s.snippets = s.snippets[:cut]
	return b.String()
}

// isCommentOnly reports whether a submission is nothing but comments, and so
// declares nothing of its own to be replaced by name.
func isCommentOnly(src string) bool {
	lx := lexer.New(source.New(docName, []byte(src)))
	comments := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch tok.Kind {
		case lexer.Whitespace:
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			comments++
		default:
			return false
		}
	}
	return comments > 0
}

// sessionOrigin names the accumulated session buffer in diagnostics, which
// belongs to no file on disk.
const sessionOrigin = "<session>"

// joined is the buffer the session analyzes: every accepted submission, with a
// submission that does not close its own text masked out so it cannot change how
// the others parse. Masking is byte for byte, so every offset still locates the
// snippet and line it came from.
func (s *Session) joined() string {
	parts := make([]string, len(s.snippets))
	for i, sn := range s.snippets {
		if sn.open {
			parts[i] = maskedText(sn.src)
			continue
		}
		parts[i] = sn.src
	}
	return strings.Join(parts, "\n")
}

// joinedFor is the buffer one session document analyzes: joined, with the
// snippets of the other language masked out too, so each document parses its
// own snippets as the kind its name carries while keeping every offset. The
// second result reports whether any snippet of that language survives.
func (s *Session) joinedFor(name string) (string, bool) {
	parts := make([]string, len(s.snippets))
	any := false
	for i, sn := range s.snippets {
		if sn.open || parseDocName(sn.origin) != name {
			parts[i] = maskedText(sn.src)
			continue
		}
		any = true
		parts[i] = sn.src
	}
	return strings.Join(parts, "\n"), any
}

// text is the buffer as it was submitted, masking nothing: what %save writes
// back, so work the parser could not read is not lost.
func (s *Session) text() string {
	parts := make([]string, len(s.snippets))
	for i, sn := range s.snippets {
		parts[i] = sn.src
	}
	return strings.Join(parts, "\n")
}

// parseDiagnostics maps a parse of one submission the way the workspace maps a
// document's: its errors carry the syntax code, its warnings their own.
func parseDiagnostics(p *parser.Parser) []passes.Diagnostic {
	out := make([]passes.Diagnostic, 0, len(p.Diagnostics)+len(p.Warnings))
	for _, d := range p.Diagnostics {
		out = append(out, passes.Diagnostic{
			Severity: passes.SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     "syntax",
			Source:   "syntax",
			Fixes:    d.Fixes,
		})
	}
	for _, w := range p.Warnings {
		out = append(out, passes.Diagnostic{
			Severity: passes.SeverityWarning,
			Span:     w.Span,
			Message:  w.Message,
			Code:     w.Code,
			Source:   "syntax",
			Fixes:    w.Fixes,
		})
	}
	return out
}

// maskedSpans locates the masked submissions in the buffer, so a finding of
// theirs is not read as having stopped the deeper checks from running.
func (s *Session) maskedSpans() []source.Span {
	var out []source.Span
	acc := 0
	for _, sn := range s.snippets {
		if sn.open {
			out = append(out, source.Span{Offset: acc, Len: len(sn.src)})
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return out
}

// openDiagnostics reports the findings of the masked submissions, located in the
// session buffer so every surface places them in the file they came from.
func (s *Session) openDiagnostics() []passes.Diagnostic {
	var out []passes.Diagnostic
	acc := 0
	for _, sn := range s.snippets {
		if sn.open {
			for _, d := range sn.diags {
				d.Span.Offset += acc
				out = append(out, d)
			}
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return out
}

// diagnostics reports the analysis of the buffer together with the syntax errors
// of the submissions masked out of it. The masked text is blanked rather than
// removed, so what the analysis finds is about the submissions that did parse
// and is reported as it stands. Both session documents share the buffer's
// coordinates, so their findings interleave by offset.
func (s *Session) diagnostics() []passes.Diagnostic {
	analyzed := append([]passes.Diagnostic{}, s.ws.Diagnostics(docName)...)
	analyzed = append(analyzed, s.ws.Diagnostics(kermlDocName)...)
	open := s.openDiagnostics()
	if len(open) == 0 {
		sort.SliceStable(analyzed, func(i, j int) bool { return analyzed[i].Span.Offset < analyzed[j].Span.Offset })
		return analyzed
	}
	out := make([]passes.Diagnostic, 0, len(analyzed)+len(open))
	out = append(out, analyzed...)
	out = append(out, open...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Span.Offset < out[j].Span.Offset })
	return out
}

// fileKeyOf is fileKey for a path that may be absent, typed input having no file.
func fileKeyOf(path string) string {
	if path == "" {
		return ""
	}
	return fileKey(path)
}

// fileKey identifies the file a path is written for, so the same file loaded
// under another spelling supersedes itself instead of accumulating a copy.
func fileKey(path string) string {
	full := expandHome(path)
	if abs, err := filepath.Abs(full); err == nil {
		full = abs
	}
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		return resolved
	}
	return filepath.Clean(full)
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
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
// analyzes the whole buffer, and returns a Result. A submission with parse errors
// is still accumulated, so its diagnostics are reported against the live session
// context; one that leaves a brace, comment or quoted name open is masked out of
// the buffer instead, since it would otherwise absorb the next submission. A
// later redeclaration of the same name replaces the prior snippet (see accept).
func (s *Session) Submit(src string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitAll([]string{src})
}

// SourceFile is one source of a submission together with the file it was read
// from, which is what diagnostics over a multi-file load are reported against.
type SourceFile struct {
	Name string
	Text string
}

// SubmitAll accumulates every src as one submission, from no file in particular.
func (s *Session) SubmitAll(srcs []string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitAll(srcs)
}

func (s *Session) submitAll(srcs []string) Result {
	files := make([]SourceFile, 0, len(srcs))
	for _, src := range srcs {
		files = append(files, SourceFile{Text: src})
	}
	return s.submitFiles(files)
}

// submit accumulates src as Submit does, recording the file it came from when it
// came from one.
func (s *Session) submit(origin, src string) Result {
	return s.submitFiles([]SourceFile{{Name: origin, Text: src}})
}

// SubmitFiles accumulates every file as one submission: all of them are accepted
// before the buffer is reindexed and analyzed, so a declaration in one resolves
// against the others no matter which order they arrive in. This is what makes
// loading a multi-file project order-independent.
func (s *Session) SubmitFiles(files []SourceFile) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submitFiles(files)
}

func (s *Session) submitFiles(files []SourceFile) Result {
	res, _, _ := s.submitEach(files)
	return res
}

// submitEach is submitFiles with the notices told apart: what accepting each
// file dropped, in file order, and what the submission did to the session as a whole.
func (s *Session) submitEach(files []SourceFile) (res Result, byFile [][]string, whole []string) {
	var (
		declared []string
		drops    []dropReport
	)
	seen := map[string]bool{}
	s.version++
	byFile = make([][]string, len(files))
	for i, f := range files {
		names, dropped := s.acceptFrom(f.Name, f.Text)
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				declared = append(declared, name)
			}
		}
		byFile[i] = dropNotices(dropped)
		drops = append(drops, dropped...)
	}
	joined := s.joined()
	offset := s.genOffset(joined)
	// A merge rewrote a snippet that was already accepted, so only the text the
	// merge added belongs to this submission; anything else is the buffer.
	own := s.ownSpans()
	if at, ok := firstText(own); ok {
		offset = at
	}
	// What the session holds is recorded against the resolution that produced it
	// before the new text replaces that resolution, so what the new document does
	// not change can be told apart from what it does.
	over := s.recordCarryover()
	sysml, _ := s.joinedFor(docName)
	s.ws.Open(docName, []byte(sysml), s.version)
	if kerml, any := s.joinedFor(kermlDocName); any {
		s.ws.Open(kermlDocName, []byte(kerml), s.version)
	} else {
		s.ws.Remove(kermlDocName)
	}
	// The document is a new AST and scope tree, so the context derived from the
	// previous one is replaced; the objects it holds are carried into the new one
	// where the declarations they were materialized against are unchanged. The
	// index is re-used and brought up to date on the next lookup instead, which is
	// why it records the version it holds.
	s.rtCtx = nil
	gone := goneNames(drops)
	whole = s.carryOverObjects(over)
	whole = append(whole, s.dropStaleDebugSessions(gone, over)...)
	notices := append(dropNotices(drops), whole...)
	s.rebindRestartedMachine()
	s.keepIdentitiesOf(over.prev)
	// The diagnostics already carry their own "did you mean" hints.
	diags := s.diagnostics()
	members := s.sessionMembers()
	res = Result{
		Members:     members,
		Declared:    declared,
		Diagnostics: diags,
		// The unmasked buffer: masking is byte for byte, so offsets still land
		// where they did, and a diagnostic echoes the line it is about.
		Source:  s.text(),
		Offset:  offset,
		Origins: s.origins(),
		own:     own,
		masked:  s.maskedSpans(),
		Notices: notices,
	}
	res.Blocked = s.blockedBy(res)
	return res, byFile, whole
}

// fileSpan is where the current submission's text from the named file sits in the
// joined buffer, through the newline closing it, where a parse that ran out of text reports.
func (s *Session) fileSpan(name string) source.Span {
	key := fileKeyOf(name)
	acc := 0
	for _, sn := range s.snippets {
		if sn.gen == s.version && sn.key == key {
			return source.Span{Offset: acc, Len: len(sn.src) + 1}
		}
		acc += len(sn.src) + 1 // the newline joined() writes between snippets
	}
	return source.Span{Offset: acc}
}

// keepIdentitiesOf hands a replaced context's identity sequence to the context
// taking its place while a debugging session still materializes objects through
// it, so the two never name one identity for two objects.
func (s *Session) keepIdentitiesOf(prev *runtime.Context) {
	if prev == nil || (s.actionExec == nil && s.stateExec == nil) {
		s.replaced = nil
		return
	}
	s.replaced = prev
	if s.rtCtx != nil {
		s.rtCtx.AdoptIdentities(prev)
	}
}

// rebindRestartedMachine points a debugging session at the execution the
// carry-over restarted, so %step and %current drive the object's live machine
// rather than the one discarded with the previous analysis.
func (s *Session) rebindRestartedMachine() {
	// Only a session over an object's own exhibited machine follows the restart:
	// a machine the object merely performs is the debugger's own execution, which
	// no restart replaced.
	if s.stateExec == nil || s.stateExec.selfFQN == "" || s.stateExec.fqn != s.stateExec.selfFQN {
		return
	}
	inst, ok := s.heldObject(s.stateExec.selfFQN)
	if !ok {
		return
	}
	behavior, ok := s.stateExec.restartedMachine(inst)
	if !ok || behavior.State == s.stateExec.executor {
		return
	}
	behavior.State.SetTrace(s.trace)
	s.stateExec.symbol = behavior.Symbol
	s.stateExec.executor = behavior.State
	s.stateExec.rtCtx = s.rtCtx
}

// restartedMachine finds, among the machines the rebuilt object exhibits, the
// one this session was attached to: the machine of the same name, or for an
// unnamed one, the machine declared in the same position.
func (st *stateSession) restartedMachine(inst *runtime.Instance) (*runtime.ObjectBehavior, bool) {
	var atPosition *runtime.ObjectBehavior
	position := 0
	for _, b := range inst.Behaviors() {
		if b.Kind != lower.ExhibitedState {
			continue
		}
		if st.machine != "" && b.Name == st.machine {
			return b, true
		}
		if position == st.machineAt {
			atPosition = b
		}
		position++
	}
	if st.machine == "" && atPosition != nil {
		return atPosition, true
	}
	return nil, false
}

// exhibitedPosition is behavior's position among the machines its object
// exhibits, which identifies it when it has no name.
func exhibitedPosition(behavior *runtime.ObjectBehavior) int {
	position := 0
	for _, b := range behavior.Object.Behaviors() {
		if b == behavior {
			return position
		}
		if b.Kind == lower.ExhibitedState {
			position++
		}
	}
	return position
}

// releaseDebuggedName respells the debuggers' object labels rooted at the object
// fqn names by its id, before the name is given to another object, so they keep
// following the object they were started on. It reports each label it respelled.
func (s *Session) releaseDebuggedName(fqn string) []string {
	var notices []string
	if a := s.actionExec; a != nil && a.selfFQN != "" {
		was := a.selfFQN
		a.selfFQN = s.relabelByID(a.selfFQN, fqn)
		if a.selfFQN != was {
			notices = append(notices, debugSessionRelabelled("action", a.name, was, a.selfFQN))
		}
	}
	st := s.stateExec
	if st == nil || st.selfFQN == "" {
		return notices
	}
	// An exhibited machine is held under its object's label as both.
	exhibited := st.fqn == st.selfFQN
	was := st.selfFQN
	st.selfFQN = s.relabelByID(st.selfFQN, fqn)
	if exhibited {
		st.fqn = st.selfFQN
	}
	if st.selfFQN != was {
		notices = append(notices, debugSessionRelabelled("state", st.name, was, st.selfFQN))
	}
	return notices
}

// debugSessionRelabelled reports a debugging session that keeps running over the
// object it was started on, now addressed by the label a displaced name left it.
func debugSessionRelabelled(kind, name, was, now string) string {
	return fmt.Sprintf("note: %s debugging session for %q keeps running over the object %s named, now %s", kind, name, was, now)
}

// dropStaleDebugSessions ends the debugging sessions this submission
// invalidated, and reports each one it ended. A session over a declaration the
// submission left alone survives — including one merged into a namespace that
// only gained an unrelated member: it keeps running against the graph and runtime
// context it started with, so stepping through a behavior does not require
// avoiding the prompt.
func (s *Session) dropStaleDebugSessions(gone []string, over carryover) []string {
	if s.actionExec == nil && s.stateExec == nil {
		return nil
	}
	var notices []string
	if by, objectGone, ok := s.staleDebugState(gone, s.actionExec.fqnOf(), s.actionExec.selfOf(), over.action); ok {
		notices = append(notices, debugSessionEnded("action", s.actionExec.name, by, objectGone))
		s.endedAction = &endedSession{
			kind:       "action",
			name:       s.actionExec.name,
			rootName:   by,
			objectGone: objectGone,
			version:    s.version,
		}
		s.actionExec.release()
		s.actionExec = nil
	}
	if by, objectGone, ok := s.staleDebugState(gone, s.stateExec.fqnOf(), s.stateExec.selfOf(), over.state); ok {
		notices = append(notices, debugSessionEnded("state", s.stateExec.name, by, objectGone))
		s.endedState = &endedSession{
			kind:       "state machine",
			name:       s.stateExec.name,
			rootName:   by,
			objectGone: objectGone,
			version:    s.version,
		}
		s.stateExec.release()
		s.stateExec = nil
	}
	return notices
}

// staleDebugState names the declaration that invalidated a debugging session:
// one this submission superseded, one the session was started against that no
// longer resolves to the same shape, or the one behind an object performing the
// behavior that this submission dropped.
// It reports separately that what went was the performing object, which reads as
// a different loss from a redeclaration.
func (s *Session) staleDebugState(gone []string, fqn, selfFQN string, shapes *runtime.Shapes) (string, bool, bool) {
	if fqn == "" {
		return "", false, false
	}
	if by, ok := supersededBy(gone, fqn); ok {
		return by, false, true
	}
	// The behavior is performed by an object this submission dropped, so what it
	// runs against is no longer part of the session.
	if selfFQN != "" {
		if _, kept := s.heldObject(selfFQN); !kept {
			return selfFQN, true, true
		}
	}
	if shapes == nil {
		return "", false, false
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return "", false, false
	}
	by, changed := ctx.Changed(shapes)
	return by, false, changed
}

// debugSessionResetEnded reports a debugging session a reset ended, which no
// redeclaration accounts for.
func debugSessionResetEnded(kind, name string) string {
	return fmt.Sprintf("note: %s debugging session for %q ended (%s)", kind, name, sessionReset)
}

func debugSessionEnded(kind, name, superseded string, objectGone bool) string {
	if objectGone {
		return fmt.Sprintf("note: %s debugging session for %q ended (the object %s performing it was dropped)", kind, name, superseded)
	}
	return fmt.Sprintf("note: %s debugging session for %q ended (%s was redeclared)", kind, name, superseded)
}

// supersededBy reports which superseded name rewrote the declaration fqn names,
// counting the namespaces it lives in: replacing a package replaces its members.
func supersededBy(gone []string, fqn string) (string, bool) {
	if fqn == "" {
		return "", false
	}
	for _, g := range gone {
		if fqn == g || strings.HasPrefix(fqn, g+"::") {
			return g, true
		}
	}
	return "", false
}

// Clear resets the session, dropping all accumulated declarations. It returns
// the notices for what the reset took with it.
func (s *Session) Clear() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clear()
}

// clear drops the document and everything derived from it. A reset replaces no
// declaration, so nothing materialized from the old one can be rebound: what
// goes is reported and recorded rather than silently emptied.
func (s *Session) clear() []string {
	notices, lost := s.resetLoss()
	s.ws.Remove(docName)
	s.ws.Remove(kermlDocName)
	s.snippets = nil
	s.version = 0
	s.rtCtx, s.replaced = nil, nil
	if s.idx != nil {
		// Drop the documents, keep the library the index was built with.
		s.idx.RemoveDocument(docName)
		s.idx.RemoveDocument(kermlDocName)
		s.idxVersion = 0
	}
	s.instances = make(map[string]*runtime.Instance)
	s.unnamed = nil
	s.lost = lost
	s.endedAction, s.endedState = nil, nil
	s.endDebugSessions(sessionReset)
	s.notedBlocker.record("")
	return notices
}

// resetLoss reports what a reset takes with it and records why, so a later
// %instances, %features or %step explains the loss instead of reading as a session
// that never materialized anything.
func (s *Session) resetLoss() (notices []string, lost instanceLoss) {
	if n := s.heldObjects(); n > 0 {
		notices = append(notices, instancesResetNotice(n))
		lost = lossOnReset(n)
	}
	if s.actionExec != nil {
		notices = append(notices, debugSessionResetEnded("action", s.actionExec.name))
	}
	if s.stateExec != nil {
		notices = append(notices, debugSessionResetEnded("state", s.stateExec.name))
	}
	return notices, lost
}

// getOrCreateRuntime lazily creates runtime context when first needed.
func (s *Session) getOrCreateRuntime() (*runtime.Context, error) {
	if s.rtCtx != nil {
		return s.rtCtx, nil
	}
	ctx, err := s.newRuntime()
	if err != nil {
		return nil, err
	}
	ctx.AdoptIdentities(s.replaced)
	s.rtCtx = ctx
	s.rtCtx.SetTrace(s.trace)
	return s.rtCtx, nil
}

// newRuntime builds a context over the session's declarations, under its budgets
// and the policy its own runs are driven by. Nothing the session holds is in it.
func (s *Session) newRuntime() (*runtime.Context, error) {
	model, resolver, err := s.semanticModel()
	if err != nil {
		return nil, err
	}
	return s.newRuntimeOver(model, resolver)
}

// semanticModel is the model and resolver a context over the session's
// declarations runs on; the model memoizes, so contexts of one exploration share it.
func (s *Session) semanticModel() (*semantics.Model, *resolve.Resolver, error) {
	// Falls back to the library index so a library symbol can be evaluated or
	// instantiated before the session declares anything.
	idx := s.browseIndex()
	if idx == nil {
		return nil, nil, fmt.Errorf("no document loaded")
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	model.SetSourceText(s.sessionSourceText())
	return model, resolver, nil
}

// newRuntimeOver builds a context over model under the session's budgets and the
// policy its own runs are driven by. Nothing the session holds is in it.
func (s *Session) newRuntimeOver(model *semantics.Model, resolver *resolve.Resolver) (*runtime.Context, error) {
	ctx := runtime.NewContext(model, resolver, s.budgets.MaxSteps)
	if err := ctx.SetBudgets(s.budgets); err != nil {
		return nil, err
	}
	// Give the runtime the buffer's text, so an error about a declaration reports
	// the line it was submitted on rather than a byte offset, and the buffer's
	// scope tree, so a carried object is rebound to the symbols the prompt reaches.
	for _, doc := range s.sessionDocs() {
		ctx.RegisterSource(source.New(doc.Name, doc.Content))
		ctx.RegisterScope(doc.Scope)
	}
	if err := ctx.SetSchedule(s.drivenSchedule()); err != nil {
		return nil, err
	}
	return ctx, nil
}

// symbolIndex indexes the session document, returning nil when nothing is
// loaded. Name lookup and the runtime context share it, and it carries the
// standard library too, which the runtime resolves names against — the
// measurement unit of a quantity expression is one.
//
// One index serves the whole session: the library is loaded into it once, and
// re-indexing the document takes back the names the previous submission declared
// and the ones its wildcard imports surfaced, so a submission costs its own
// document rather than a reload of the library.
func (s *Session) symbolIndex() *symbols.Index {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil
	}
	if s.idx == nil {
		s.idx, s.libSource = model.NewIndexWithStdlib()
	} else if s.idxVersion == doc.Version {
		return s.idx
	}
	s.idx.AddDocument(docName, doc.AST)
	if kdoc := s.ws.Document(kermlDocName); kdoc != nil {
		s.idx.AddDocument(kermlDocName, kdoc.AST)
	} else {
		s.idx.RemoveDocument(kermlDocName)
	}
	s.idx.ExpandWildcardImports()
	s.idxVersion = doc.Version
	return s.idx
}

// sessionDocs returns the session's open documents, the SysML buffer first,
// so a caller reading the whole session reads both languages.
func (s *Session) sessionDocs() []*model.Document {
	var out []*model.Document
	for _, name := range []string{docName, kermlDocName} {
		if doc := s.ws.Document(name); doc != nil {
			out = append(out, doc)
		}
	}
	return out
}

// sessionMembers returns the top-level members of both session documents in
// buffer order, which their shared coordinates make the span order.
func (s *Session) sessionMembers() []ast.Node {
	var out []ast.Node
	for _, doc := range s.sessionDocs() {
		if doc.AST != nil {
			out = append(out, doc.AST.Members...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Span().Offset < out[j].Span().Offset })
	return out
}

// qualifiedOr returns the looked-up qualified name, falling back to the name as
// typed when the lookup reported none.
func qualifiedOr(fqn, typed string) string {
	if fqn != "" {
		return fqn
	}
	return typed
}
