package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const toolUsage = "usage: %tool <case|action>[(<args>)] [<object>]"

// doTool carries out %tool at the prompt.
func (s *Session) doTool(tail string) ([]string, bool, error) {
	inv, err := splitAnalysisArgs(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), toolUsage}, false, nil
	}
	if inv.name == "" {
		return []string{toolUsage}, false, nil
	}
	return s.withTrace(s.toolDryRunInv(inv)).Lines, false, nil
}

// ToolDryRun previews the first tool call the case or action target names would
// make, without starting the tool's process.
func (s *Session) ToolDryRun(target string) Verdict {
	defer s.enter()()
	inv, err := splitAnalysisArgs(target)
	if err != nil {
		return s.withTrace(unresolvedVerdict(target, err.Error()))
	}
	if inv.name == "" {
		return s.withTrace(unresolvedVerdict(target, toolUsage))
	}
	return s.withTrace(s.toolDryRunInv(inv))
}

// toolDryRunInv resolves and runs the invocation under the dry runner, reporting
// the preview of the tool call it reached.
func (s *Session) toolDryRunInv(inv analysisInvocation) Verdict {
	label := inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return unresolvedVerdict(label, "no declarations loaded")
	}
	sym, fqn, err := s.lookupSymbolOfKinds(inv.name,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolActionDef, symbols.SymbolActionUsage)
	if err != nil {
		return unresolvedVerdict(label, err.Error())
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return unresolvedVerdict(label, fmt.Errorf("%w: %w", errRuntimeInit, err).Error())
	}
	ctx.SetToolRunner(s.engines.DryRunner())
	defer s.attachTools(ctx)

	isAction := sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage
	if isAction {
		err = s.runActionToCompletion(ctx, inv, sym)
	} else {
		_, err = s.runAnalysisIn(s.direct(), ctx, inv, sym, fqn, heldObjects{s})
	}
	return toolDryVerdict(label, err)
}

// runActionToCompletion runs the action to completion as %action + %continue do,
// without leaving a debugging session active.
func (s *Session) runActionToCompletion(ctx *runtime.Context, inv analysisInvocation, sym *symbols.Symbol) error {
	var performer []string
	if inv.object != "" {
		performer = []string{inv.object}
	}
	self, _, err := s.performingObject(performer)
	if err != nil {
		return err
	}
	parsed, err := parseAnalysisArgs(inv.argText)
	if err != nil {
		return err
	}
	scope := s.promptScope()
	var positional []runtime.Value
	for _, arg := range parsed.positional {
		val, err := ctx.EvalWithScope(arg.expr, scope)
		if err != nil {
			return fmt.Errorf("evaluation of argument %q failed: %w", arg.text, err)
		}
		positional = append(positional, val)
	}
	named, err := s.evalArguments(ctx, parsed.named)
	if err != nil {
		return err
	}
	inputs, err := ctx.ActionInputs(sym, positional, named)
	if err != nil {
		return err
	}
	exec, err := ctx.CreateActionExecutorWithInputs(sym, self, inputs)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}
	defer exec.Release()
	exec.SetTrace(s.trace)
	return exec.RunToCompletion()
}

// toolDryVerdict reports a dry run: the preview the typed error carried, the
// refusal of the entry or the manifest as the run's error, or that no tool was
// reached at all.
func toolDryVerdict(label string, err error) Verdict {
	var dry *analysis.ToolDryRunError
	if errors.As(err, &dry) {
		lines := append([]string{fmt.Sprintf("%s %s: dry run of tool '%s' for %s",
			statusMark(VerdictHolds), label, dry.Preview.Tool, dry.Action)},
			indent(dry.Preview.Lines())...)
		return Verdict{Subject: label, Status: VerdictHolds, Lines: lines}
	}
	if err != nil {
		return unresolvedVerdict(label, err.Error())
	}
	return unresolvedVerdict(label, "no ToolExecution-annotated action was reached; nothing to preview")
}
