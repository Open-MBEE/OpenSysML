package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MetadataAnnotationPass checks a metadata annotation: what it may annotate, and
// that its body restates features of its type and binds model-level evaluable
// values (KerML 1.0 §7.4.9, §8.3.4.9).
type MetadataAnnotationPass struct{}

const (
	// msgCannotAnnotate is completed with the metaclass of the element the
	// annotation may not be applied to.
	msgCannotAnnotate = "Cannot annotate "
	// msgOwningTypeFeature is reported on a body feature restating nothing.
	msgOwningTypeFeature = "Must redefine an owning-type feature"
)

func (MetadataAnnotationPass) Level() PassLevel { return LevelType }

func (MetadataAnnotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &metadataAnnotationChecker{model: ctx.Model()}
	if c.model == nil {
		return nil
	}
	w := &w8cWalker{ctx: ctx}
	w.walk(rootScope, c.checkSymbol)
	c.checkAnnotations(rootScope, root, func(typeRef *ast.QualifiedName) (string, bool) {
		return c.model.OwnerAnnotatedElementViolation(rootScope, typeRef)
	})
	return c.diags
}

type metadataAnnotationChecker struct {
	model *semantics.Model
	diags []diag.Diagnostic
}

// checkSymbol checks each annotation of sym's declaration. The annotated element
// owns its annotations, prefix or member, so each names its type in the
// element's own scope.
func (c *metadataAnnotationChecker) checkSymbol(sym *symbols.Symbol) {
	scope := semantics.AnnotationScope(sym)
	c.checkAnnotations(scope, sym.Decl, func(typeRef *ast.QualifiedName) (string, bool) {
		return c.model.AnnotatedElementViolation(sym, scope, typeRef)
	})
	if u, ok := sym.Decl.(*ast.Usage); ok && u.Kind == ast.UsageMetadata {
		c.checkMetadataUsage(sym, u)
	}
}

// checkAnnotations checks the annotations written on decl, read in scope;
// violation judges whether the type an `about`-less one names may annotate decl.
func (c *metadataAnnotationChecker) checkAnnotations(scope *symbols.Scope, decl ast.Node, violation func(*ast.QualifiedName) (string, bool)) {
	if scope == nil {
		return
	}
	for _, a := range semantics.MetadataAnnotationsOf(decl) {
		if metaclass, bad := violation(a.Node.Type); bad {
			c.reportCannotAnnotate(a.Node.Span(), metaclass)
		}
		c.checkBody(scope, a.Node)
		c.checkNested(scope, a.Node)
	}
	for _, a := range semantics.MetadataAnnotationsAboutOthers(decl) {
		for _, metaclass := range c.model.AboutAnnotatedElementViolations(scope, a.Node.Type, a.Node.About) {
			c.reportCannotAnnotate(a.Node.Span(), metaclass)
		}
		c.checkBody(scope, a.Node)
		c.checkNested(scope, a.Node)
	}
}

// checkNested checks what an unnamed annotation's body declares: annotations of
// the annotation itself, and the symbols the walk does not reach.
func (c *metadataAnnotationChecker) checkNested(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	body := unnamedMetadataBody(scope, prefix)
	if body == nil {
		return
	}
	c.checkAnnotations(body, prefix, func(typeRef *ast.QualifiedName) (string, bool) {
		return c.model.OwnerAnnotatedElementViolation(body, typeRef)
	})
	forEachBodySymbol(body, c.checkSymbol)
}

// checkMetadataUsage checks what `metadata m : M about x;` may annotate, or, with
// no `about`, whether M may annotate the element owning the usage.
func (c *metadataAnnotationChecker) checkMetadataUsage(sym *symbols.Symbol, u *ast.Usage) {
	if sym.OwnerScope == nil {
		return
	}
	var typeRef *ast.QualifiedName
	var about []*ast.QualifiedName
	for _, rel := range u.Relationships {
		if rel == nil {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping:
			if typeRef == nil {
				typeRef = qn
			}
		case ast.RelAnnotates:
			about = append(about, qn)
		}
	}
	if typeRef == nil {
		return
	}
	if symbols.UsageAnnotatesOthers(u) {
		for _, metaclass := range c.model.AboutAnnotatedElementViolations(sym.OwnerScope, typeRef, about) {
			c.reportCannotAnnotate(u.Span(), metaclass)
		}
		return
	}
	if metaclass, bad := c.model.OwnerAnnotatedElementViolation(sym.OwnerScope, typeRef); bad {
		c.reportCannotAnnotate(u.Span(), metaclass)
	}
}

func (c *metadataAnnotationChecker) reportCannotAnnotate(span source.Span, metaclass string) {
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     span,
		Message:  msgCannotAnnotate + metaclass,
		Code:     "metadata-annotated-element",
		Source:   "constraint",
	})
}

// checkBody checks that the annotation's body restates features of its type and
// binds model-level evaluable values.
func (c *metadataAnnotationChecker) checkBody(scope *symbols.Scope, prefix *ast.PrefixMetadata) {
	for _, node := range c.model.MetadataBodyViolations(scope, prefix) {
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     node.Span(),
			Message:  msgOwningTypeFeature,
			Code:     "metadata-owning-type-feature",
			Source:   "constraint",
		})
	}
	for _, value := range c.model.MetadataBodyInevaluableValues(scope, prefix) {
		c.diags = append(c.diags, diag.Diagnostic{
			Severity: diag.SeverityError,
			Span:     metadataValueSpan(prefix.Body, value),
			Message:  msgFilterNotEvaluable,
			Code:     "metadata-value-not-evaluable",
			Source:   "constraint",
		})
	}
}

// metadataValueSpan is the span of the binding `= value` in a metadata body, or of the value alone.
func metadataValueSpan(body []ast.Node, value ast.Node) source.Span {
	var found source.Span
	var walk func([]ast.Node)
	walk = func(body []ast.Node) {
		for _, node := range body {
			if mem, ok := node.(*ast.Membership); ok {
				node = mem.Member
			}
			usage, ok := node.(*ast.Usage)
			if !ok {
				continue
			}
			if usage.Value == value && usage.ValueOperatorSpan.Len > 0 {
				end := value.Span().End()
				found = source.Span{
					Offset: usage.ValueOperatorSpan.Offset,
					Len:    end - usage.ValueOperatorSpan.Offset,
				}
				return
			}
			walk(usage.Members)
			if found.Len > 0 {
				return
			}
		}
	}
	walk(body)
	if found.Len > 0 {
		return found
	}
	return value.Span()
}
