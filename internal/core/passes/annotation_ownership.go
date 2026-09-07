package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// KerMLValidator's validateAnnotationAnnotatedElementOwnership message.
const msgAnnotationOwnsAnnotating = "Must own its annotating element"

// AnnotationOwnershipPass checks that an annotation owned by its annotated element
// owns its annotating element (KerML 8.3.2.3.3): textually, no `about` names itself.
type AnnotationOwnershipPass struct{}

func (AnnotationOwnershipPass) Level() PassLevel { return LevelConstraint }

// ElementScoped: each `about` reference gates on its own resolution.
func (AnnotationOwnershipPass) ElementScoped() { /* marker: per-element gating */ }

func (AnnotationOwnershipPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &annotationOwnershipChecker{ctx: ctx, resolver: ctx.Resolver()}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, c.check)
	return c.diags
}

type annotationOwnershipChecker struct {
	ctx      *Context
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (c *annotationOwnershipChecker) check(sym *symbols.Symbol) {
	if sym == nil || sym.OwnerScope == nil {
		return
	}
	for _, about := range annotatedElementRefs(sym.Decl) {
		if c.ctx.DownstreamOfFailure(about) {
			continue
		}
		target, ok := c.resolver.ResolveQualified(sym.OwnerScope, about)
		if !ok || target == nil {
			continue
		}
		if alias, aliasOK := c.resolver.ResolveAliasTarget(target); aliasOK {
			target = alias
		}
		if target != sym {
			continue
		}
		c.diags = append(c.diags, Diagnostic{
			Severity: SeverityError,
			Span:     about.Span(),
			Message:  msgAnnotationOwnsAnnotating,
			Code:     "annotation-annotated-element-ownership",
			Source:   "constraint",
		})
	}
}

// annotatedElementRefs lists the names an annotating element's `about` clause states.
func annotatedElementRefs(decl ast.Node) []*ast.QualifiedName {
	switch d := decl.(type) {
	case *ast.Comment:
		return d.About
	case *ast.PrefixMetadata:
		return d.About
	case *ast.Usage:
		if d.Kind != ast.UsageMetadata {
			return nil
		}
		var out []*ast.QualifiedName
		for _, rel := range d.Relationships {
			if rel == nil || rel.Kind != ast.RelAnnotates {
				continue
			}
			if qn, ok := rel.Target.(*ast.QualifiedName); ok {
				out = append(out, qn)
			}
		}
		return out
	}
	return nil
}
