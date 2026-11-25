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
	answer, err := s.runInlineQuery(text)
	if err != nil {
		verdict := unresolvedVerdict(text, err.Error())
		verdict.Lines = append(verdict.Lines, queryexec.OutcomeLines(err)...)
		verdict.Outcome = queryOutcome(err)
		verdict.ResultView = answer.view
		return verdict
	}
	return Verdict{Subject: text, Status: VerdictHolds, Lines: answer.lines, Values: answer.values, Table: answer.table, ResultView: answer.view}
}

// inlineInvocation reads `inline <expression>` out of a %run-query invocation;
// false when it names a query, even one named inline followed by its bindings.
func inlineInvocation(invocation string) (string, bool) {
	rest, ok := strings.CutPrefix(invocation, inlineKeyword)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	text := strings.TrimSpace(rest)
	if args := splitQueryArgs(text); len(args) > 0 && isBindingArg(args[0]) {
		return "", false
	}
	return text, true
}

// isBindingArg reports whether a %run-query argument is a `<parameter>=<expression>`
// binding: a parameter name, with no call in it, before a single `=`.
func isBindingArg(arg string) bool {
	name, value, ok := strings.Cut(arg, "=")
	return ok && name != "" && !strings.ContainsAny(name, "(\"'") && !strings.HasPrefix(value, "=")
}

// runInlineQuery plans text over the session's declarations, in an index of its
// own beside them, and executes it against the session's model and objects.
func (s *Session) runInlineQuery(text string) (queryAnswer, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return queryAnswer{}, fmt.Errorf("runtime init: %w", err)
	}
	inline, err := queryplan.ParseInline(s.browseIndex(), text)
	if err != nil {
		return queryAnswer{}, inlineQueryError(err)
	}
	resolver := resolve.New(inline.Index)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(s.sessionSourceText())
	sem.SetSourceFile(s.sessionSourceFile)
	sem.ShareAbout(s.about)
	program, err := queryplan.Compile(inline.Index, sem, resolver, inline.Entry)
	if err != nil {
		return queryAnswer{}, inlineQueryError(queryexec.LocateInline(inline, err))
	}
	context := s.queryContext(ctx)
	context.Index, context.Resolver, context.Model = inline.Index, resolver, sem
	result, err := queryexec.Execute(program, context, nil, queryexec.Options{})
	view := s.resultView(queryexec.DefaultResultViewName, text, context, result, err)
	if err != nil {
		return queryAnswer{view: view}, inlineQueryError(queryexec.LocateInline(inline, err))
	}
	lines, values, table := renderRows(inlineQueryHeading, result, context)
	return queryAnswer{lines: lines, values: values, table: table, view: view}, nil
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
