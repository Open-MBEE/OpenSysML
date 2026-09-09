package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const analysisUsage = "usage: %analysis <name>[(<args>)] [<object>]"

// analysisInvocation is `%analysis`'s tail taken apart: the case's name, the
// argument list written in parentheses after it, and the object named after
// that as the case's subject.
type analysisInvocation struct {
	name    string
	argText string
	object  string
}

// splitAnalysisArgs takes apart `%analysis`'s tail: `Case`, `Case ship`,
// `Case(3.0, limit = 4.0)` or `Case(3.0) ship`. Arguments are written as an
// invocation so a bare word after the name is unambiguously the subject.
func splitAnalysisArgs(tail string) (analysisInvocation, error) {
	tail = strings.TrimSpace(tail)
	cut := indexOutsideName(tail, " \t(")
	if cut < 0 {
		return analysisInvocation{name: tail}, nil
	}
	inv := analysisInvocation{name: tail[:cut]}
	rest := strings.TrimSpace(tail[cut:])
	if strings.HasPrefix(rest, "(") {
		end := closingParen(rest)
		if end < 0 {
			return analysisInvocation{}, fmt.Errorf("argument list %q is not closed", rest)
		}
		inv.argText = rest[1:end]
		rest = strings.TrimSpace(rest[end+1:])
	}
	if rest == "" {
		return inv, nil
	}
	if strings.ContainsAny(rest, " \t") {
		return analysisInvocation{}, fmt.Errorf("%q does not name one object; arguments are written in parentheses after the case's name", rest)
	}
	inv.object = rest
	return inv, nil
}

// analysisArgs are an invocation's arguments parsed and not yet evaluated:
// positional ones in order, and the ones written as `<parameter> = <expr>`.
type analysisArgs struct {
	positional []argExpr
	named      []argument
}

// parseAnalysisArgs parses an argument list in which a positional argument and
// one binding a parameter by name may be mixed, as a case's inputs allow.
func parseAnalysisArgs(text string) (analysisArgs, error) {
	var args analysisArgs
	for _, arg := range splitArgs(text) {
		if isNamedArgument(arg) {
			named, err := parseArguments([]string{arg})
			if err != nil {
				return analysisArgs{}, err
			}
			args.named = append(args.named, named...)
			continue
		}
		expr, err := parseWholeExpr(arg)
		if err != nil {
			return analysisArgs{}, err
		}
		args.positional = append(args.positional, argExpr{expr: expr, text: arg})
	}
	return args, nil
}

// doAnalysis carries out %analysis at the prompt.
func (s *Session) doAnalysis(tail string) ([]string, bool, error) {
	inv, err := splitAnalysisArgs(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), analysisUsage}, false, nil
	}
	if inv.name == "" {
		return []string{analysisUsage}, false, nil
	}
	return s.withTrace(s.analysisVerdict(inv)).Lines, false, nil
}

// analysisVerdict runs an analysis case and reports what it computed and
// decided. A run that could not be made is unresolved; one whose objective or
// assertion did not hold fails; one a check left undecided is unresolved too,
// since it decided nothing about the model. A verification case answers with
// the verdict of its body as well, which its status reports. A run that failed
// after evaluating some of what it declares reports those evaluations and the
// verdicts left undecided beneath the error.
func (s *Session) analysisVerdict(inv analysisInvocation) Verdict {
	label := inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	run, err := s.runAnalysis(inv)
	if err != nil {
		verdict := unresolvedVerdict(label, err.Error())
		s.reportCaseRun(&verdict, run.result)
		return verdict
	}
	result, subject, subjectLabel := run.result, run.subject, run.label

	status := VerdictHolds
	for _, v := range result.Verdicts {
		switch v.Status {
		case runtime.VerdictNotSatisfied:
			if status == VerdictHolds {
				status = VerdictFails
			}
		case runtime.VerdictUndecided:
			status = VerdictUnresolved
		}
	}
	// The case's own body decides its status; a subcase answers for itself,
	// since the library states no roll-up into the case performing it.
	for _, v := range run.verdicts {
		if s := verificationStatus(v.Kind); !v.Subcase && s > status {
			status = s
		}
	}
	mark := "✓"
	switch status {
	case VerdictFails:
		mark = "✗"
	case VerdictUnresolved:
		mark = "?"
	}
	on := ""
	if subject != nil {
		on = " on " + objectMention(subject, subjectLabel)
	}
	verdict := Verdict{Subject: label, Status: status, Lines: []string{fmt.Sprintf("%s %s%s", mark, label, on)}}
	s.reportCaseRun(&verdict, result)
	for _, v := range run.verdicts {
		verdict.Verifications = append(verdict.Verifications, VerificationVerdict{
			Case: v.Case, Kind: string(v.Kind), Detail: v.Detail, Subcase: v.Subcase,
		})
		verdict.Lines = append(verdict.Lines, "  "+verificationLine(v))
	}
	return verdict
}

// reportCaseRun adds to verdict what a case run produced: each output, each
// objective and assertion verdict, then each evaluation the run made of one of
// the case's calcs — a trade study's alternatives in subject order, the
// selected one marked, and those evaluating alike marked tied.
func (s *Session) reportCaseRun(verdict *Verdict, result runtime.AnalysisResult) {
	ctx := s.rtCtx
	for _, out := range result.Outputs {
		text := objectText(ctx, out.Value)
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  %s = %s", out.Name, text))
		verdict.Values = append(verdict.Values, NamedValue{Name: out.Name, Value: text})
	}
	for _, v := range result.Verdicts {
		name := v.Kind + " " + v.Name
		text := v.Status.String()
		if v.Detail != "" {
			text += ": " + v.Detail
		}
		verdict.Lines = append(verdict.Lines, fmt.Sprintf("  %s: %s", name, text))
		verdict.Values = append(verdict.Values, NamedValue{Name: name, Value: text})
	}
	for _, e := range result.Evaluations {
		evaluation, text := evaluationOf(ctx, result.Case, e)
		verdict.Lines = append(verdict.Lines, "  "+text)
		verdict.Evaluations = append(verdict.Evaluations, evaluation)
	}
}

// evaluationOf reports one evaluation a run of caseName made, as data and as the
// line a report prints: the call relative to the case, its result or error, and
// whether it was selected or tied.
func evaluationOf(ctx *runtime.Context, caseName string, e runtime.AnalysisEvaluation) (Evaluation, string) {
	evaluation := Evaluation{Function: e.Function, Selected: e.Selected, Tied: e.Tied}
	for _, arg := range e.Arguments {
		evaluation.Arguments = append(evaluation.Arguments, objectText(ctx, arg))
	}
	text := fmt.Sprintf("%s(%s)", strings.TrimPrefix(e.Function, caseName+"::"), strings.Join(evaluation.Arguments, ", "))
	if e.Error != nil {
		evaluation.Error = e.Error.Error()
		text += ": error: " + evaluation.Error
	} else {
		evaluation.Result = formatValue(ctx, e.Result)
		text += " = " + evaluation.Result
	}
	switch {
	case e.Selected:
		text += " [selected]"
	case e.Tied:
		text += " [tied]"
	}
	return evaluation, text
}

// objectText spells a value as a report names it: an object that is the
// occurrence of a usage by that usage, anything else as %eval prints it.
func objectText(ctx *runtime.Context, val runtime.Value) string {
	if val.Kind == runtime.ValInstance {
		if inst, ok := ctx.Instance(val.Instance); ok {
			if usage := ctx.OccurrenceUsage(inst); usage != "" {
				return fmt.Sprintf("%s (object #%d)", usage, inst.ID)
			}
		}
	}
	return formatValue(ctx, val)
}

// caseRun is what one run of a case produced: what it computed and decided, the
// object it ran on, and, for a verification case, the verdict of its body and of
// every subcase it performed.
type caseRun struct {
	result   runtime.AnalysisResult
	subject  *runtime.Instance
	label    string
	verdicts []runtime.VerificationVerdict
}

// runAnalysis resolves the case an invocation names, evaluates its arguments
// and the object named as its subject, and runs it in the session's context. A
// usage nested in a type is run as a feature of the object the session holds for
// that type, as a constraint is checked on the object carrying it.
func (s *Session) runAnalysis(inv analysisInvocation) (caseRun, error) {
	sym, fqn, err := s.analysisSymbol(inv)
	if err != nil {
		return caseRun{}, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return caseRun{}, err
	}
	return s.runAnalysisIn(ctx, inv, sym, fqn, heldObjects{s})
}

// analysisSymbol resolves the case an invocation names. It is resolved before the
// runtime is built, so a misspelling is reported as one whatever the session holds.
func (s *Session) analysisSymbol(inv analysisInvocation) (*symbols.Symbol, string, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return nil, "", errors.New("no declarations loaded")
	}
	return s.lookupSymbolOfKinds(inv.name,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage)
}

// runAnalysisIn runs a case in ctx, finding the object named as its subject, and
// the one owning a usage nested in a type, where objects finds them.
func (s *Session) runAnalysisIn(ctx *runtime.Context, inv analysisInvocation, sym *symbols.Symbol, fqn string, objects runObjects) (caseRun, error) {
	if err := ctx.RequireAnalysisCase(sym); err != nil {
		return caseRun{}, err
	}

	parsed, err := parseAnalysisArgs(inv.argText)
	if err != nil {
		return caseRun{}, err
	}
	var args runtime.AnalysisArgs
	scope := s.promptScope()
	for _, arg := range parsed.positional {
		val, err := ctx.EvalWithScope(arg.expr, scope)
		if err != nil {
			return caseRun{}, fmt.Errorf("evaluation of argument %q failed: %w", arg.text, err)
		}
		args.Positional = append(args.Positional, val)
	}
	if args.Named, err = s.evalArguments(ctx, parsed.named); err != nil {
		return caseRun{}, err
	}

	run := caseRun{}
	if inv.object != "" {
		if run.subject, run.label, err = objects.object(inv.object); err != nil {
			return caseRun{}, err
		}
		args.Subject = run.subject
	}

	// A usage owned by a type is a feature of an object of that type, which the
	// session holds when one was created; a package-level case has no such owner.
	self := nestedCaseOwner(sym, fqn, objects)
	runScope := declaringScope(sym, s.ws.Document(docName).Scope)

	// A verification case runs the same body; asking the run for its verdict too
	// reports it beside what the run computed.
	if runtime.IsVerificationCaseSymbol(sym) {
		verified, err := ctx.RunVerification(sym, args, runScope, self)
		if err != nil {
			return caseRun{}, err
		}
		run.result = verified.Run
		run.verdicts = append([]runtime.VerificationVerdict{verified.Verdict}, verified.Subcases...)
		return run, nil
	}
	result, err := ctx.RunAnalysis(sym, args, runScope, self)
	run.result = result
	if err != nil {
		if errors.Is(err, runtime.ErrNotAnAnalysis) {
			return caseRun{}, err
		}
		return run, fmt.Errorf("analysis run failed: %w", err)
	}
	return run, nil
}
