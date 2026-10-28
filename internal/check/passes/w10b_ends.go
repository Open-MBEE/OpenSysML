package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Pilot SysMLValidator (2026-05) constraints validateInterfaceDefinitionEnd,
// validateInterfaceUsageEnd and validateFlowConnectionDefinitionEnd, and the
// validateConnectionDefinitionEndsAreUsages and validateFlowDefinitionEndsAreUsages
// of its master branch (SysML v2.1 Beta 2 §8.3.13.3, §8.3.16.2).
const (
	msgInterfaceDefEndPort = "An interface definition end must be a port."
	msgInterfaceEndPort    = "An interface end must be a port."
	msgFlowDefTooManyEnds  = "A flow definition can have at most two ends."
)

// SysML v2.1 Beta 2 §8.3.13.3 types `connectionEnd : Usage [0..*] {redefines
// associationEnd}` and §8.3.16.2 `flowEnd : Usage [0..*] {redefines
// associationEnd}`: every end feature of a connection or flow definition, owned
// or inherited, is a Usage. A KerML `end feature` inherited from a kernel
// association (Links::BinaryLink, Transfers::Transfer, ...) violates that.
const (
	msgConnectionDefEndUsage = "A connection definition end must be a usage."
	msgFlowDefEndUsage       = "A flow definition end must be a usage."
)

// W10BEndKindPass checks the kind and the number of end features an interface or
// a flow connection definition declares. It is level-scoped to the type tier
// like the usage-typing rules it accompanies, since the reference reports both
// on the same declaration.
type W10BEndKindPass struct{}

func (W10BEndKindPass) Level() PassLevel { return LevelType }

func (W10BEndKindPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w10bEndChecker{model: ctx.Model(), index: ctx.Index, doc: name, kind: ctx.Kind}
	kit.WalkSymbols(ctx, rootScope, func(sym *symbols.Symbol) {
		switch sym.Kind {
		case symbols.SymbolConnectionDef, symbols.SymbolAllocationDef:
			c.checkEndsAreUsages(sym, msgConnectionDefEndUsage, "connection-def-end-not-usage")
		case symbols.SymbolInterfaceDef:
			c.checkEndsAreUsages(sym, msgConnectionDefEndUsage, "connection-def-end-not-usage")
			c.checkEndsArePorts(sym, msgInterfaceDefEndPort)
		case symbols.SymbolInterfaceUsage:
			c.checkEndsArePorts(sym, msgInterfaceEndPort)
		case symbols.SymbolFlowDef:
			c.checkEndsAreUsages(sym, msgFlowDefEndUsage, "flow-def-end-not-usage")
			c.checkFlowDefEndCount(sym)
		}
	})
	return c.diags
}

type w10bEndChecker struct {
	model *semantics.Model
	index *symbols.Index
	doc   string      // the document under analysis
	kind  source.Kind // and its language
	diags []diag.Diagnostic
}

func (c *w10bEndChecker) report(node ast.Node, msg, code string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     node.Span(),
		Message:  msg,
		Code:     code,
		Source:   "type",
	})
}

// checkEndsArePorts reports each end feature of an interface declared with a
// kind other than `port`; an end written without a kind keyword is a port.
func (c *w10bEndChecker) checkEndsArePorts(sym *symbols.Symbol, msg string) {
	for _, end := range w10bOwnedEnds(sym) {
		if end.Kind == ast.UsagePort || end.Keyword == "" {
			continue
		}
		c.report(end, msg, "interface-end-not-port")
	}
}

// checkFlowDefEndCount reports a flow definition with more than two end
// features: at the extra declared ends, or at the definition itself when the
// excess is inherited.
func (c *w10bEndChecker) checkFlowDefEndCount(sym *symbols.Symbol) {
	owned := w10bOwnedEnds(sym)
	if c.endCount(sym, map[*symbols.Symbol]bool{}) <= 2 {
		return
	}
	if len(owned) <= 2 {
		if decl := sym.Decl; decl != nil {
			c.report(decl, msgFlowDefTooManyEnds, "flow-def-too-many-ends")
		}
		return
	}
	for _, end := range owned[2:] {
		c.report(end, msgFlowDefTooManyEnds, "flow-def-too-many-ends")
	}
}

// endCount is the number of end features sym has. Declared ends redefine the
// inherited ones position by position, so the count is the larger of the two
// rather than their sum.
func (c *w10bEndChecker) endCount(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) int {
	if sym == nil || seen[sym] {
		return 0
	}
	seen[sym] = true
	count := len(w10bOwnedEnds(sym))
	for _, general := range c.model.DirectSupertypes(sym) {
		if inherited := c.endCount(general, seen); inherited > count {
			count = inherited
		}
	}
	return count
}

// w10bOwnedEnds are the end features sym declares in its body, in declaration
// order, anonymous ones included.
func w10bOwnedEnds(sym *symbols.Symbol) []*ast.Usage {
	var members []ast.Node
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		members = decl.Members
	case *ast.Usage:
		members = decl.Members
	default:
		return nil
	}
	var ends []*ast.Usage
	for _, member := range members {
		if m, ok := member.(*ast.Membership); ok {
			member = m.Member
		}
		u, ok := member.(*ast.Usage)
		if ok && u.IsEnd {
			ends = append(ends, u)
		}
	}
	return ends
}

// checkEndsAreUsages reports once at the connection or flow definition sym when
// an end feature of its is no SysML usage. A definition written in SysML can own
// usages only, so the offending end is always an inherited one, and the report
// sits where the reference puts it in that case: at the definition itself.
func (c *w10bEndChecker) checkEndsAreUsages(sym *symbols.Symbol, msg, code string) {
	if c.model == nil || sym.Decl == nil {
		return
	}
	for _, end := range c.model.EndFeatures(sym) {
		if !c.isUsage(end) {
			c.report(sym.Decl, msg, code)
			return
		}
	}
}

// isUsage reports whether end is a SysML Usage: a usage declared outside a KerML
// document. A KerML `end feature`, parsed or restored from a record, is not.
func (c *w10bEndChecker) isUsage(end *symbols.Symbol) bool {
	if end == nil || c.docKind(end.DocName) == source.KindKerML {
		return false
	}
	if end.Recorded() {
		return end.Facts.Node == symbols.NodeUsage
	}
	_, ok := end.Decl.(*ast.Usage)
	return ok
}

// docKind is the language of the document named doc: the analysed document's as
// the context states it, any other's as the index records it, which honours a
// language declared apart from the file name.
func (c *w10bEndChecker) docKind(doc string) source.Kind {
	if doc == c.doc {
		return c.kind
	}
	return c.index.DocumentKind(doc)
}
