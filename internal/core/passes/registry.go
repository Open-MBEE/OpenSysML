package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Registry holds an ordered set of validation passes.
type Registry struct {
	passes []Pass
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a pass to the registry.
func (r *Registry) Register(p Pass) {
	r.passes = append(r.passes, p)
}

// Passes returns the registered passes in registration order.
func (r *Registry) Passes() []Pass {
	return append([]Pass(nil), r.passes...)
}

// ElementScoped marks a pass whose subject is a single element rather than the
// document, so a failure on another element does not skip it. Such a pass names
// its subject in code and asks Context.DownstreamOfFailure about the references
// that subject rests on.
type ElementScoped interface{ ElementScoped() }

// Run executes all registered passes in ascending PassLevel order and returns
// their accumulated diagnostics. A pass whose subject is the document is skipped
// once a strictly lower level emitted a blocking diagnostic, avoiding cascade
// noise; an ElementScoped pass runs and gates itself per element. Passes at the
// same level always run.
func (r *Registry) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	ordered := make([]Pass, len(r.passes))
	copy(ordered, r.passes)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Level() < ordered[j].Level()
	})

	var all []diag.Diagnostic
	// failedLevel is the lowest level at which an Error occurred; document-scoped
	// passes at a strictly higher level are skipped.
	failed := false
	var failedLevel PassLevel
	byLevel := map[PassLevel][]source.Span{}
	if ctx != nil {
		byLevel[LevelSyntax] = blockingSpans(ctx.ParseDiagnostics)
	}
	for _, p := range ordered {
		_, elementScoped := p.(ElementScoped)
		if failed && p.Level() > failedLevel && !elementScoped {
			continue
		}
		if ctx != nil {
			ctx.SetFailures(spansBelow(byLevel, p.Level()))
		}
		diags := p.Run(ctx, name, root)
		all = append(all, diags...)
		if spans := blockingSpans(diags); len(spans) > 0 {
			byLevel[p.Level()] = append(byLevel[p.Level()], spans...)
			if !failed || p.Level() < failedLevel {
				failedLevel = p.Level()
			}
			failed = true
		}
	}
	if ctx != nil {
		ctx.SetFailures(nil)
	}
	return all
}

// blockingSpans returns where the diagnostics a level depends on were reported.
func blockingSpans(diags []diag.Diagnostic) []source.Span {
	var out []source.Span
	for _, d := range diags {
		if d.Blocking() {
			out = append(out, d.Span)
		}
	}
	return out
}

// spansBelow collects the blocking spans of every level strictly below level.
func spansBelow(byLevel map[PassLevel][]source.Span, level PassLevel) []source.Span {
	var out []source.Span
	for l, spans := range byLevel {
		if l < level {
			out = append(out, spans...)
		}
	}
	return out
}
