package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
)

// inlineKeyword is the %run-query argument that introduces a query expression
// in place of a query's name.
const inlineKeyword = "inline"

// inlineQueryHeading is what an inline query's rows are reported under.
const inlineQueryHeading = "Inline query"

// RunInlineDocumentQuery compiles text as the result expression of an anonymous
// document query, executes it and reports as RunDocumentQuery does.
func (s *Session) RunInlineDocumentQuery(text string) Verdict {
	defer s.enter()()
	return s.withTrace(s.inlineQueryVerdict(text))
}

func (s *Session) inlineQueryVerdict(text string) Verdict {
	lines, values, table, err := s.runInlineQuery(text)
	if err != nil {
		verdict := unresolvedVerdict(text, err.Error())
		verdict.Lines = append(verdict.Lines, queryexec.OutcomeLines(err)...)
		verdict.Outcome = queryOutcome(err)
		return verdict
	}
	return Verdict{Subject: text, Status: VerdictHolds, Lines: lines, Values: values, Table: table}
}

// inlineInvocation reads `inline <expression>` out of a %run-query invocation,
// false when the invocation names a query instead.
func inlineInvocation(invocation string) (string, bool) {
	rest, ok := strings.CutPrefix(invocation, inlineKeyword)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// runInlineQuery plans text over the session's declarations, in an index of its
// own beside them, and executes it against the session's model and objects.
func (s *Session) runInlineQuery(text string) ([]string, []NamedValue, *VerdictTable, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("runtime init: %w", err)
	}
	inline, err := queryplan.ParseInline(s.browseIndex(), text)
	if err != nil {
		return nil, nil, nil, inlineQueryError(err)
	}
	resolver := resolve.New(inline.Index)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(s.sessionSourceText())
	sem.SetSourceFile(s.sessionSourceFile)
	sem.ShareAbout(s.about)
	program, err := queryplan.Compile(inline.Index, sem, resolver, inline.Entry)
	if err != nil {
		return nil, nil, nil, inlineQueryError(queryexec.LocateInline(inline, err))
	}
	context := s.queryContext(ctx)
	context.Index, context.Resolver, context.Model = inline.Index, resolver, sem
	result, err := queryexec.Execute(program, context, nil, queryexec.Options{})
	if err != nil {
		return nil, nil, nil, inlineQueryError(queryexec.LocateInline(inline, err))
	}
	lines, values, table := renderRows(inlineQueryHeading, result, inline.Index, context.ElementID)
	return lines, values, table, nil
}

// inlineQueryError reports a fault located in the inline text with the line it
// is on and a caret under it, as a binding expression's fault is reported.
func inlineQueryError(err error) error {
	var located *queryplan.InlineError
	if !errors.As(err, &located) {
		return err
	}
	line, col := located.Line()
	return fmt.Errorf("%w\n%s\n%s", err, line, caretLine(line, col, located.Span.Len))
}
