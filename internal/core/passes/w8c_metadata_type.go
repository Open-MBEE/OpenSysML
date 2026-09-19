package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const (
	// KerMLValidator's validateMetadataFeatureMetaclassNotAbstract message.
	msgMetadataConcreteType = "Must have a concrete type"
	// KerMLValidator's validateMetadataFeatureMetaclass message.
	msgMetadataMetaclass = "Must have exactly one metaclass"
)

// MetadataTypePass reports an annotation (`@M`, `#M` or `metadata m : M`) whose type is
// not a concrete metaclass or metadata definition (KerML validateMetadataFeatureMetadata,
// validateMetadataFeatureMetadataNotAbstract; SysML validateMetadataUsageType).
type MetadataTypePass struct{}

func (MetadataTypePass) Level() PassLevel { return LevelType }

// Its subject is one annotation, and the reference that subject rests on is the
// metaclass it names: a failure on another element does not hide it.
func (MetadataTypePass) ElementScoped() { /* marker: per-element gating */ }

func (MetadataTypePass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &metadataTypeChecker{ctx: ctx, resolver: ctx.Resolver()}
	w := &kit.Walker{Ctx: ctx}
	w.Walk(rootScope, c.checkSymbol)
	c.checkAnnotations(rootScope, root)
	return c.diags
}

type metadataTypeChecker struct {
	ctx      *Context
	resolver interface {
		ResolveQualified(*symbols.Scope, *ast.QualifiedName) (*symbols.Symbol, bool)
		ResolveAliasTarget(*symbols.Symbol) (*symbols.Symbol, bool)
	}
	diags []diag.Diagnostic
}

func (c *metadataTypeChecker) checkSymbol(sym *symbols.Symbol) {
	c.checkAnnotations(semantics.AnnotationScope(sym), sym.Decl)
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageMetadata && sym.OwnerScope != nil {
		// A SysML usage typed by the wrong kind of element is the usage typing
		// rule's; more than one type is the one-type rule's.
		if typeRef := metadataUsageType(u); typeRef != nil {
			c.check(sym.OwnerScope, typeRef, u.Span(), c.ctx.Kind == source.KindKerML)
		}
	}
}

// checkAnnotations checks the type of each annotation written on decl, read in
// scope, and of everything an unnamed annotation's body declares in turn.
func (c *metadataTypeChecker) checkAnnotations(scope *symbols.Scope, decl ast.Node) {
	if scope == nil {
		return
	}
	for _, a := range semantics.MetadataAnnotationsWritten(decl) {
		c.check(scope, a.Node.Type, a.Node.Span(), true)
		if body := kit.UnnamedMetadataBody(scope, a.Node); body != nil {
			c.checkAnnotations(body, a.Node)
			kit.ForEachBodySymbol(body, c.checkSymbol)
		}
	}
}

// check judges the type an annotation at span names in scope; reportKind says
// whether a type that is no metaclass is this pass's to report.
func (c *metadataTypeChecker) check(scope *symbols.Scope, typeRef *ast.QualifiedName, span source.Span, reportKind bool) {
	if typeRef == nil || c.ctx.DownstreamOfFailure(typeRef) {
		return
	}
	target, ok := c.resolver.ResolveQualified(scope, typeRef)
	if !ok || target == nil {
		return
	}
	if resolved, aliasOK := c.resolver.ResolveAliasTarget(target); aliasOK {
		target = resolved
	} else {
		return
	}
	if !semantics.IsMetadataType(target) {
		if reportKind {
			c.report(span, metadataMetaclassMessage(c.ctx.Kind), "metadata-metaclass")
		}
		return
	}
	if symbols.IsAbstract(target) {
		c.report(span, msgMetadataConcreteType, "metadata-concrete-type")
	}
}

func (c *metadataTypeChecker) report(span source.Span, msg, code string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msg,
		Code:     code,
		Source:   "constraint",
	})
}

// metadataUsageType returns the one type a `metadata m : M` usage declares, or
// nil when it declares none or several.
func metadataUsageType(u *ast.Usage) *ast.QualifiedName {
	var typeRef *ast.QualifiedName
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || typeRef != nil {
			return nil
		}
		typeRef = qn
	}
	return typeRef
}

// metadataMetaclassMessage names the rule in the words of the notation the
// annotation is written in: SysML spells a metaclass `metadata def`.
func metadataMetaclassMessage(kind source.Kind) string {
	if kind == source.KindKerML {
		return msgMetadataMetaclass
	}
	return oneTypeUsageMessages[ast.UsageMetadata]
}
