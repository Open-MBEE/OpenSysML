package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// verdictKindName is the enumeration a verification case's verdict is a literal
// of (VerificationCases::VerdictKind).
const verdictKindName = "VerificationCases::VerdictKind"

// verdictOutputName is the output the library declares a case's verdict as
// (VerificationCases::VerificationCase::verdict).
const verdictOutputName = "verdict"

// VerdictKind is the verdict a run of a verification case's body produced, one
// literal of the library's VerdictKind enumeration.
type VerdictKind string

const (
	// VerdictPass is a body whose verdict value is VerdictKind::pass.
	VerdictPass VerdictKind = "pass"
	// VerdictFail is a body whose verdict value is VerdictKind::fail.
	VerdictFail VerdictKind = "fail"
	// VerdictInconclusive is a body that ran and produced no verdict value.
	VerdictInconclusive VerdictKind = "inconclusive"
	// VerdictError is a body whose run could not be carried out.
	VerdictError VerdictKind = "error"
)

// VerificationVerdict is the verdict one run of a verification case's body
// produced. It is reported beside the requirement-satisfaction verdicts of the
// same case, which it does not replace: the SysML v2 specification leaves the
// evaluation of a verdict unspecified, so this verdict is what running the body
// and the library's own PassIf calculation answer.
type VerificationVerdict struct {
	// Case is the qualified name of the verification case that ran.
	Case string

	// Symbol declares the verification case.
	Symbol *symbols.Symbol

	// Kind is the VerdictKind literal the body produced.
	Kind VerdictKind

	// Detail is the message of an error verdict, or why an inconclusive one
	// decided nothing; empty for pass and fail.
	Detail string

	// Subcase marks the verdict of a case performed by another, which the
	// library states no roll-up for and which is therefore reported on its own.
	Subcase bool
}

// VerificationResult is what one run of a verification case produced: the
// verdict of its body, what the run reported as an analysis case's run does,
// and the verdict of each verification subcase its body performs.
type VerificationResult struct {
	// Verdict is the body's verdict.
	Verdict VerificationVerdict

	// Run is the case's outputs and the verdict of each objective and
	// assertion; zero when the body could not run.
	Run AnalysisResult

	// Subcases are the verdicts of the verification cases this case performs as
	// steps. The library states no roll-up of a subcase's verdict into its
	// parent's (VerificationCases::VerificationCase::subVerificationCases), so
	// each is reported on its own.
	Subcases []VerificationVerdict
}

// IsVerificationCaseSymbol reports whether sym declares a verification case
// definition or usage.
func IsVerificationCaseSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageVerificationCase
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolVerificationCaseDef || sym.Kind == symbols.SymbolVerificationCaseUsage
}

// RequireVerificationCase reports ErrNotAVerification for a symbol that is not a
// verification case definition or usage, describing what it is instead.
func (ctx *Context) RequireVerificationCase(sym *symbols.Symbol) error {
	if sym == nil {
		return fmt.Errorf("%w: invalid symbol", ErrNotAVerification)
	}
	if !IsVerificationCaseSymbol(sym) {
		return fmt.Errorf("%w: %s is %s, not a verification case definition or usage",
			ErrNotAVerification, ctx.qualifiedSymbolName(sym), describeDecl(sym.Decl))
	}
	return nil
}

// RunVerification runs a verification case's body as RunAnalysis runs an
// analysis case's — same lowering, same subject and input binding, same step
// execution — and reports the verdict the body produced together with the
// objective and assertion verdicts of the same run.
//
// A body that cannot run is the case's error verdict carrying the message, not
// an error of the call: a verdict is what a verification case answers. An error
// is returned only for a request that cannot be made at all — a symbol that is
// not a verification case, or arguments the case does not take.
func (ctx *Context) RunVerification(sym *symbols.Symbol, args AnalysisArgs, scope *symbols.Scope, self *Instance) (VerificationResult, error) {
	if err := ctx.RequireVerificationCase(sym); err != nil {
		return VerificationResult{}, err
	}
	defer ctx.beginRun()()

	name := ctx.qualifiedSymbolName(sym)
	caseRun, run, err := ctx.runCase(sym, args, scope, self)
	if err != nil {
		if badRequest(err) {
			return VerificationResult{}, err
		}
		return VerificationResult{
			Verdict: VerificationVerdict{Case: name, Symbol: sym, Kind: VerdictError, Detail: err.Error()},
		}, nil
	}
	result := VerificationResult{Verdict: ctx.bodyVerdict(sym, name, caseRun, run), Run: run}
	result.Subcases = ctx.subcaseVerdicts(caseRun)
	return result, nil
}

// badRequest reports whether an error faults the request rather than the run:
// asking a case that does not run, or binding arguments it does not declare.
// Every other failure is a run that could not be carried out.
func badRequest(err error) bool {
	return errors.Is(err, ErrNotAnAnalysis) ||
		errors.Is(err, ErrNotAVerification) ||
		errors.Is(err, ErrCalcArity) ||
		errors.Is(err, ErrUnknownParameter)
}

// bodyVerdict reads the verdict the case returned: the VerdictKind literal its
// result parameter holds, bound from the library's PassIf calculation or from a
// literal of its own. An `out` parameter of the same enumeration is not a
// verdict, so it does not answer for the case.
func (ctx *Context) bodyVerdict(sym *symbols.Symbol, name string, run *calcRun, result AnalysisResult) VerificationVerdict {
	verdict := VerificationVerdict{Case: name, Symbol: sym}
	values := make(map[string]Value, len(result.Outputs))
	for _, out := range result.Outputs {
		values[out.Name] = out.Value
	}
	for _, output := range verdictOutputNames(run.resultName()) {
		value, held := values[output]
		if !held {
			continue
		}
		if kind, ok := ctx.verdictOf(value); ok {
			verdict.Kind = kind
			return verdict
		}
	}
	verdict.Kind, verdict.Detail = VerdictInconclusive, "the case body bound no VerdictKind value"
	return verdict
}

// verdictOf reads a value as a verdict: a literal of the library's VerdictKind
// enumeration, or of an enumeration specializing it.
func (ctx *Context) verdictOf(value Value) (VerdictKind, bool) {
	lit := value.Literal()
	if lit == nil {
		return "", false
	}
	if !ctx.isVerdictKind(semantics.EnumerationDefinitionOwning(lit)) {
		return "", false
	}
	switch kind := VerdictKind(lit.Name); kind {
	case VerdictPass, VerdictFail, VerdictInconclusive, VerdictError:
		return kind, true
	}
	return "", false
}

// isVerdictKind reports whether an enumeration definition is the library's
// VerdictKind, or one specializing it.
func (ctx *Context) isVerdictKind(enum *symbols.Symbol) bool {
	if enum == nil {
		return false
	}
	if ctx.qualifiedSymbolName(enum) == verdictKindName {
		return true
	}
	for _, super := range ctx.model.semantics.MemberSources(enum) {
		if super != nil && ctx.qualifiedSymbolName(super) == verdictKindName {
			return true
		}
	}
	return false
}

// subcaseVerdicts are the verdicts of the verification cases the run performed as
// steps, in declaration order; a step the run did not take answered no verdict.
func (ctx *Context) subcaseVerdicts(run *calcRun) []VerificationVerdict {
	if run == nil || run.perf == nil {
		return nil
	}
	var out []VerificationVerdict
	for _, node := range run.shape.Nodes {
		sym := memberSymbol(run.shape.bodyScope(), node)
		if !IsVerificationCaseSymbol(sym) {
			continue
		}
		if verdict, performed := ctx.subcaseVerdict(run, node, sym); performed {
			out = append(out, verdict)
		}
	}
	return out
}

// subcaseVerdict reads the verdict of one performed subcase from the outputs its
// performance holds; performed reports whether the run took the step at all.
func (ctx *Context) subcaseVerdict(run *calcRun, node ast.Node, sym *symbols.Symbol) (VerificationVerdict, bool) {
	verdict := VerificationVerdict{Case: ctx.qualifiedSymbolName(sym), Symbol: sym, Subcase: true}
	perf, _, err := run.perf.subaction(ActionNodeName(node), node)
	if err != nil || perf == nil {
		return verdict, false
	}
	for _, name := range verdictOutputNames(perf.result) {
		value, held := perf.data[perf.key(name)]
		if !held {
			continue
		}
		if kind, ok := ctx.verdictOf(value); ok {
			verdict.Kind = kind
			return verdict, true
		}
	}
	verdict.Kind, verdict.Detail = VerdictInconclusive, "the subcase bound no VerdictKind value"
	return verdict, true
}

// verdictOutputNames are the outputs of a case a verdict is read from: the result
// the case names, then the library's `verdict`, then an unnamed result.
func verdictOutputNames(result string) []string {
	names := make([]string, 0, 3)
	if result != "" {
		names = append(names, result)
	}
	return append(names, verdictOutputName, resultOutputName)
}
