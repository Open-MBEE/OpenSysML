package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// checkValue refuses a new value that is not one expression. Whether it names
// anything is answered by analyzing the edited model, where it has a scope.
func (m Model) checkValue(i int, op Operation) error {
	text := strings.TrimSpace(op.Value)
	sf := source.New("<value>", []byte(text))
	refuse := func(reason string, diags []passes.Diagnostic) error {
		return &Error{
			Failure:        FailureInvalidValue,
			OperationIndex: i,
			Diagnostics:    diags,
			Diagnosed:      sf,
			Message:        fmt.Sprintf("value %q for %s %s", op.Value, op.Target, reason),
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
	if lexer.IsKeyword(name) {
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

// validate re-reads the edited notation the way the original was read and
// refuses it if it carries errors the original did not: an edit never hands back
// a model that cannot be read again.
func (m Model) validate(content []byte) error {
	sf := source.NewWithKind(m.Source.Name(), content, m.Source.Kind())
	p := parser.New(sf)
	root := p.ParseFile()
	editedParse := parseDiagnostics(p.Diagnostics)
	originalParse := parseDiagnostics(m.ParseDiags)
	if introduced := introduced(originalParse, editedParse); len(introduced) > 0 {
		return &Error{
			Failure:        FailureResultInvalid,
			OperationIndex: -1,
			Diagnostics:    introduced,
			Diagnosed:      sf,
			Message:        "the edited model does not parse: " + introduced[0].Message,
		}
	}
	if m.NewIndex == nil {
		return nil
	}
	if m.reindex == nil { // validated outside an Apply call
		m.reindex = &reindexer{newIndex: m.NewIndex}
	}
	// The parse diagnostics are handed to the analysis, so a model that already
	// had syntax errors is not judged by tiers its own parse never reached. The
	// baseline is taken first: it and the edited notation share one index, in
	// which each is in turn the document under the model's name.
	before := errorsOnly(m.baseline(editedParse))
	before = append(before, errorsOnly(originalParse)...)
	idx := m.reindex.analyzedIn(sf.Name(), root, sf.Kind())
	after := errorsOnly(passes.AnalyzeWithKind(sf.Name(), sf.Kind(), root, editedParse, idx))
	if introduced := introduced(before, after); len(introduced) > 0 {
		return &Error{
			Failure:        FailureResultInvalid,
			OperationIndex: -1,
			Diagnostics:    introduced,
			Diagnosed:      sf,
			Message:        "the edited model is not valid: " + introduced[0].Message,
		}
	}
	return nil
}

// baseline is what the original was already wrong about, judged at the tiers the
// edited notation is judged at. A model that did not parse was never analyzed —
// the service analyzes a clean parse only — so its stored diagnostics say
// nothing about the tiers an edit that repairs the syntax reaches for the first
// time; the original is analyzed here instead, under the edited model's gate, so
// that both are compared at one tier.
func (m Model) baseline(gate []passes.Diagnostic) []passes.Diagnostic {
	if len(m.ParseDiags) == 0 {
		return m.SemDiags
	}
	p := parser.New(m.Source)
	root := p.ParseFile()
	idx := m.reindex.analyzedIn(m.Source.Name(), root, m.Source.Kind())
	return passes.AnalyzeWithKind(m.Source.Name(), m.Source.Kind(), root, gate, idx)
}

// parseDiagnostics presents parse diagnostics as pass diagnostics, which is how
// every consumer of them reports them: as syntax errors.
func parseDiagnostics(diags []parser.Diagnostic) []passes.Diagnostic {
	out := make([]passes.Diagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, passes.Diagnostic{
			Severity: passes.SeverityError,
			Span:     d.Span,
			Message:  d.Message,
			Code:     "syntax",
			Source:   "syntax",
		})
	}
	return out
}

// errorsOnly keeps the diagnostics that say the model is wrong.
func errorsOnly(diags []passes.Diagnostic) []passes.Diagnostic {
	out := make([]passes.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// introduced returns the diagnostics of the edited notation that the original
// did not already have. Spans move when bytes are spliced, so a diagnostic is
// identified by what it says rather than by where it says it.
func introduced(before, after []passes.Diagnostic) []passes.Diagnostic {
	counts := map[string]int{}
	for _, d := range before {
		counts[diagKey(d)]++
	}
	var out []passes.Diagnostic
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

func diagKey(d passes.Diagnostic) string {
	return d.Source + "\x00" + d.Code + "\x00" + d.Message
}
