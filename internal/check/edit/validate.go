package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// checkValue refuses a new value that is not one expression. Whether it names
// anything is answered by analyzing the edited model, where it has a scope.
func (m Model) checkValue(i int, op Operation) error {
	return m.checkExpression(i, "value", op.Target, op.Value)
}

func (m Model) checkExpression(i int, label, target, value string) error {
	text := strings.TrimSpace(value)
	sf := source.New("<value>", []byte(text))
	refuse := func(reason string, diags []diag.Diagnostic) error {
		return &Error{
			Failure:        FailureInvalidValue,
			OperationIndex: i,
			Diagnostics:    diags,
			Diagnosed:      sf,
			Message:        fmt.Sprintf("%s %q for %s %s", label, value, target, reason),
		}
	}
	if text == "" {
		return refuse("is empty", nil)
	}
	p := parser.New(sf)
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 {
		return refuse("does not parse as an expression", parseDiagnostics(p.Diagnostics))
	}
	if expr == nil {
		return refuse("does not parse as an expression", nil)
	}
	if end := expr.Span().End(); end != len(text) {
		return refuse(fmt.Sprintf("is not one expression: %q is left over", text[end:]), nil)
	}
	return nil
}

// checkName refuses a new name that would not lex as the identifier it replaces.
func checkName(i int, name string) error {
	refuse := func(reason string) error {
		return &Error{
			Failure:        FailureInvalidName,
			OperationIndex: i,
			Message:        fmt.Sprintf("new name %q %s", name, reason),
		}
	}
	if name == "" {
		return refuse("is empty")
	}
	if source.IsKeyword(name) {
		return refuse("is a keyword")
	}
	lx := lexer.New(source.New("<name>", []byte(name)))
	tok := lx.Next()
	switch {
	case tok.Kind != lexer.Identifier && tok.Kind != lexer.UnrestrictedName:
		return refuse("is not an identifier")
	case tok.Unterminated:
		return refuse("is an unterminated quoted name")
	case tok.Span.Len != len(name):
		return refuse("is not a single name")
	}
	return nil
}

// validate re-reads every rewritten document the way the original was read and
// refuses the edit if any carries errors its original did not: an edit never
// hands back a model that cannot be read again. The documents are judged
// together, each analyzed in an index holding the others as rewritten.
func (m Model) validate(edited rewrites) error {
	own := m.Source.Name()
	type reread struct {
		Model
		sf          *source.SourceFile
		root        *ast.RootNamespace
		editedParse []diag.Diagnostic
		before      []diag.Diagnostic
	}
	rereads := make([]*reread, 0, len(edited))
	for _, name := range edited.names(own) {
		doc, _ := m.inDocument(name)
		sf := source.NewWithKind(name, edited[name].content, doc.Source.Kind())
		p := parser.New(sf)
		rr := &reread{Model: doc, sf: sf, root: p.ParseFile(), editedParse: parseDiagnostics(p.Diagnostics)}
		originalParse := parseDiagnostics(doc.ParseDiags)
		if introduced := introduced(originalParse, rr.editedParse); len(introduced) > 0 {
			return invalidResult(own, sf, introduced, "does not parse")
		}
		rr.before = errorsOnly(originalParse)
		rereads = append(rereads, rr)
	}
	if m.NewIndex == nil {
		return nil
	}
	if m.reindex == nil { // validated outside an Apply call
		m.reindex = newReindexer(m)
	}
	// The parse diagnostics are handed to the analysis, so a model that already
	// had syntax errors is not judged by tiers its own parse never reached. The
	// baselines are taken first: they and the edited notation share one index, in
	// which each is in turn the document under its name.
	for _, rr := range rereads {
		rr.reindex = m.reindex
		rr.before = append(errorsOnly(rr.baseline(rr.editedParse)), rr.before...)
	}
	var idx *symbols.Index
	for _, rr := range rereads {
		idx = m.reindex.analyzedIn(rr.sf, rr.root)
	}
	for _, rr := range rereads {
		after := errorsOnly(passes.AnalyzeWithOptions(rr.sf.Name(), rr.sf.Kind(), rr.root, rr.editedParse, idx, m.Analysis))
		if introduced := introduced(rr.before, after); len(introduced) > 0 {
			return invalidResult(own, rr.sf, introduced, "is not valid")
		}
	}
	return nil
}

// invalidResult refuses the edit for the errors introduced into sf, named when
// it is not the edited document own.
func invalidResult(own string, sf *source.SourceFile, introduced []diag.Diagnostic, what string) error {
	where := ""
	if sf.Name() != own {
		where = " in " + sf.Name()
	}
	return &Error{
		Failure:        FailureResultInvalid,
		OperationIndex: -1,
		Diagnostics:    introduced,
		Diagnosed:      sf,
		Message:        "the edited model " + what + where + ": " + introduced[0].Message,
	}
}

// baseline is what the original was already wrong about, judged at the tiers the
// edited notation is judged at. A model that did not parse was never analyzed —
// the service analyzes a clean parse only — so its stored diagnostics say
// nothing about the tiers an edit that repairs the syntax reaches for the first
// time; the original is analyzed here instead, under the edited model's gate, so
// that both are compared at one tier.
func (m Model) baseline(gate []diag.Diagnostic) []diag.Diagnostic {
	if len(m.ParseDiags) == 0 {
		return m.SemDiags
	}
	p := parser.New(m.Source)
	root := p.ParseFile()
	idx := m.reindex.analyzedIn(m.Source, root)
	return passes.AnalyzeWithOptions(m.Source.Name(), m.Source.Kind(), root, gate, idx, m.Analysis)
}

// parseDiagnostics presents parse diagnostics as pass diagnostics, which is how
// every consumer of them reports them: as syntax errors.
func parseDiagnostics(diags []parser.Diagnostic) []diag.Diagnostic {
	out := make([]diag.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     "syntax",
			Source:   "syntax",
		})
	}
	return out
}

// errorsOnly keeps the diagnostics that say the model is wrong.
func errorsOnly(diags []diag.Diagnostic) []diag.Diagnostic {
	out := make([]diag.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// introduced returns the diagnostics of the edited notation that the original
// did not already have. Spans move when bytes are spliced, so a diagnostic is
// identified by what it says rather than by where it says it.
func introduced(before, after []diag.Diagnostic) []diag.Diagnostic {
	counts := map[string]int{}
	for _, d := range before {
		counts[diagKey(d)]++
	}
	var out []diag.Diagnostic
	for _, d := range after {
		key := diagKey(d)
		if counts[key] > 0 {
			counts[key]--
			continue
		}
		out = append(out, d)
	}
	return out
}

func diagKey(d diag.Diagnostic) string {
	return d.Source + "\x00" + d.Code + "\x00" + d.Message
}
