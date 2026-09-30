package passes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// UndeclaredSignalPass warns when a transition's `when <name>` or a state's
// `defer <name>` names neither a declaration visible where it is written nor a
// signal any `send` of the model sends. Such a name is matched against injected
// signals by name alone (see resolve.Resolver.resolveTrigger), so a misspelling
// would otherwise wait for a signal nothing sends.
type UndeclaredSignalPass struct{}

// Level reports the name-resolution level: what the trigger's name reaches is
// all it reads.
func (UndeclaredSignalPass) Level() PassLevel { return LevelNameResolution }

// Run checks the signal triggers of one workspace document.
func (UndeclaredSignalPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil || ctx.Index.IsLibraryDocument(name) {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &signalLint{ctx: ctx, union: signalUnionOf(ctx)}
	if !ctx.Gathers().Has(name) {
		c.local = map[string]bool{}
		gatherSentSignals(ctx, rootScope, c.local)
	}
	for _, at := range kit.ScopedNodes(ctx, rootScope) {
		switch n := at.Node.(type) {
		case *ast.TransitionMember:
			if ref, ok := n.Trigger.(*ast.FeatureReference); ok && ref.Name != nil {
				c.check(at.Scope, "when", ref.Name)
			}
		case *ast.DeferMember:
			for _, trigger := range n.Triggers {
				if qn, ok := trigger.(*ast.QualifiedName); ok {
					c.check(at.Scope, "defer", qn)
				}
			}
		}
	}
	return c.diags
}

type signalLint struct {
	ctx   *Context
	union *signalUnion
	local map[string]bool
	diags []diag.Diagnostic
}

// check reports the signal name qn written after keyword in scope when it
// reaches no declaration and no send sends it.
func (c *signalLint) check(scope *symbols.Scope, keyword string, qn *ast.QualifiedName) {
	if len(qn.Parts) == 0 || c.ctx.DownstreamOfFailure(qn) {
		return
	}
	if _, ok := c.ctx.Resolver().ReadQualified(scope, qn).Symbol(); ok {
		return
	}
	// The runtime matches a trigger by the last segment of what it names.
	name := qn.Parts[len(qn.Parts)-1].Text
	if c.sent(name) {
		return
	}
	written := qnText(qn)
	msg := fmt.Sprintf(
		"`%s %s` names no declaration visible here and no signal the model sends, so only a signal injected by that name triggers it",
		keyword, written)
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     qn.Span(),
		Message:  c.ctx.Resolver().SuggestName(msg, scope, name, qn, suggest.Nearest(name, c.sentNames())),
		Code:     CodeUndeclaredSignal,
		Source:   lintSource,
	})
}

func (c *signalLint) sent(name string) bool {
	c.ctx.Resolver().ReadName(signalSentName(name))
	return c.union.sent.Has(name) || c.local[name]
}

// sentNames lists every signal name the model sends, for suggestions.
func (c *signalLint) sentNames() []string {
	c.ctx.Resolver().ReadName(signalAnySentName)
	names := make([]string, 0, len(c.union.sent)+len(c.local))
	for name := range c.union.sent {
		names = append(names, name)
	}
	for name := range c.local {
		if !c.union.sent.Has(name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// qnText spells a qualified name as written.
func qnText(qn *ast.QualifiedName) string {
	parts := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		parts[i] = part.Text
	}
	return strings.Join(parts, "::")
}

// gatherSentSignals records in sent the names of the signals every send of the
// document under root sends: the name its payload is written as — the type a
// `new T()` or `T()` payload constructs, the last segment of a name or feature
// chain — and the types of the feature such a name reaches, which is what the
// runtime tells the sent message's signal by.
func gatherSentSignals(ctx *Context, root *symbols.Scope, sent map[string]bool) {
	for _, at := range kit.ScopedNodes(ctx, root) {
		send, ok := at.Node.(*ast.SendStatement)
		if !ok {
			continue
		}
		if send.Message != nil {
			noteSentSignal(ctx, at.Scope, send.Message, sent)
			continue
		}
		noteSentSignal(ctx, kit.BodyScope(at.Scope, send), lower.SendPayload(send), sent)
		if param := lower.SendPayloadParameter(send); param != nil {
			for _, rel := range param.Relationships {
				if rel != nil && rel.Kind == ast.RelTyping {
					noteSentName(rel.Target, sent)
				}
			}
		}
	}
}

// noteSentSignal records the signal names a send payload expression sends.
func noteSentSignal(ctx *Context, scope *symbols.Scope, expr ast.Node, sent map[string]bool) {
	switch e := expr.(type) {
	case nil:
		return
	case *ast.ConstructorExpr:
		noteSentDefinition(ctx, scope, e.Type, sent)
	case *ast.InvocationExpr:
		if e.Operand != nil || e.Type == nil {
			return
		}
		model := ctx.Model()
		sel := model.SelectCall(scope, e, semantics.PerformsBehavior)
		if !model.CallsCalc(sel) {
			noteSentDefinition(ctx, scope, e.Type, sent)
			return
		}
		// The calculation's value is sent, typed by its result.
		for _, called := range selectedCalcs(sel) {
			for _, typ := range model.DeclaredTypes(model.ResultParameterOf(called)) {
				if typ != nil && typ.Name != "" {
					sent[typ.Name] = true
				}
			}
		}
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		sym, ok := ctx.Resolver().ResolveTarget(scope, e)
		if !ok || sym == nil {
			noteSentName(e, sent)
			return
		}
		if sym = ctx.Resolver().AliasedElement(sym); isDefinition(sym) {
			sent[sym.Name] = true
			return
		}
		// A feature sends its value: a scalar by its scalar type, else by its type.
		if scalars := ctx.Model().SentScalarTypes(scope, e); len(scalars) > 0 {
			for _, name := range scalars {
				sent[name] = true
			}
			return
		}
		for _, typ := range ctx.Model().DeclaredTypes(sym) {
			if typ != nil && typ.Name != "" {
				sent[typ.Name] = true
			}
		}
	default:
		for _, name := range ctx.Model().SentScalarTypes(scope, e) {
			sent[name] = true
		}
	}
}

// noteSentDefinition records the definition a constructed or invoked message
// names, through any alias, or the name as written where it names none.
func noteSentDefinition(ctx *Context, scope *symbols.Scope, name *ast.QualifiedName, sent map[string]bool) {
	if name == nil {
		return
	}
	if sym, ok := ctx.Resolver().ResolveTarget(scope, name); ok && sym != nil {
		if sym = ctx.Resolver().AliasedElement(sym); isDefinition(sym) {
			sent[sym.Name] = true
			return
		}
	}
	noteSentName(name, sent)
}

// isDefinition reports whether sym is a named definition, which a message is typed by.
func isDefinition(sym *symbols.Symbol) bool {
	if sym == nil || sym.Name == "" {
		return false
	}
	_, ok := sym.DefinitionKind()
	return ok
}

// noteSentName records the last segment of a name written in a send.
// selectedCalcs is what a call of a calculation may run: the one selected, or
// every tied candidate when the arguments' values choose.
func selectedCalcs(sel *semantics.InvocationSelection) []*symbols.Symbol {
	if sel.Ambiguous {
		return sel.Tied
	}
	return []*symbols.Symbol{sel.Called()}
}

func noteSentName(node ast.Node, sent map[string]bool) {
	var qn *ast.QualifiedName
	switch n := node.(type) {
	case *ast.QualifiedName:
		qn = n
	case *ast.FeatureReference:
		qn = n.Name
	case *ast.FeatureChainExpr:
		qn = n.Member
	}
	if qn != nil && len(qn.Parts) > 0 {
		if name := qn.Parts[len(qn.Parts)-1].Text; name != "" {
			sent[name] = true
		}
	}
}
