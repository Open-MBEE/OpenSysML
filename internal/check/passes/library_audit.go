package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// libraryAudit is what the passes auditing a model against a bundled vocabulary
// library share: the library definitions they bind to and how a model's elements are read.
type libraryAudit struct {
	ctx   *Context
	model *semantics.Model
	// source is the diagnostic source the audit reports under.
	source string
	// defs holds the library definitions by qualified name.
	defs map[string]*symbols.Symbol
	// conforms memoizes conformance of a type to a library definition.
	conforms map[[2]*symbols.Symbol]bool
	diags    []diag.Diagnostic
}

// newLibraryAudit returns nil unless every name is an element of a bundled library:
// a workspace package of the same name is not the library's vocabulary.
func newLibraryAudit(ctx *Context, source string, fqns ...string) *libraryAudit {
	a := &libraryAudit{
		ctx:      ctx,
		model:    ctx.Model(),
		source:   source,
		defs:     map[string]*symbols.Symbol{},
		conforms: map[[2]*symbols.Symbol]bool{},
	}
	for _, fqn := range fqns {
		def := libraryElement(ctx, fqn)
		if def == nil {
			return nil
		}
		a.defs[fqn] = def
	}
	return a
}

// libraryElement returns the bundled library element registered under fqn, or nil.
func libraryElement(ctx *Context, fqn string) *symbols.Symbol {
	for _, def := range ctx.Index.LookupQualified(fqn) {
		if def != nil && ctx.Index.Library(def) {
			return def
		}
	}
	return nil
}

// enumLiteral returns the literal name of an enumeration value of enumFQN, or "".
func enumLiteral(v symbols.FilterValue, enumFQN string) string {
	if v.Kind != symbols.FilterValueRef || !strings.HasPrefix(v.RefFQN, enumFQN+"::") {
		return ""
	}
	return strings.TrimPrefix(v.RefFQN, enumFQN+"::")
}

// annotationIs reports whether an annotation's type is, or conforms to, the library metadata definition.
func (a *libraryAudit) annotationIs(facts symbols.AnnotationFacts, fqn string) bool {
	def := a.defs[fqn]
	for _, t := range a.ctx.Index.LookupQualified(facts.TypeFQN) {
		if t == def || a.model.Conforms(t, def) {
			return true
		}
	}
	return false
}

// conformsTo reports whether a type is, or specializes, the library definition fqn.
func (a *libraryAudit) conformsTo(t *symbols.Symbol, fqn string) bool {
	def := a.defs[fqn]
	key := [2]*symbols.Symbol{t, def}
	if ok, seen := a.conforms[key]; seen {
		return ok
	}
	ok := t == def || a.model.Conforms(t, def)
	a.conforms[key] = ok
	return ok
}

// isA reports whether a feature is typed, directly or through a keyword, by a
// type conforming to the library definition fqn.
func (a *libraryAudit) isA(sym *symbols.Symbol, fqn string) bool {
	if sym == nil || !sym.IsFeature() {
		return false
	}
	return a.anyConformsTo(a.model.FeatureTypeSet(sym), fqn)
}

// anyConformsTo reports whether one of types conforms to the library definition fqn.
func (a *libraryAudit) anyConformsTo(types []*symbols.Symbol, fqn string) bool {
	for _, t := range types {
		if a.conformsTo(t, fqn) {
			return true
		}
	}
	return false
}

// features returns the features sym states: those of its own body and those it
// inherits from the types and generals the model, not the library, declares.
func (a *libraryAudit) features(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, member := range a.model.MembersOf(sym) {
		if member.Kind != symbols.SymbolAlias && member.IsFeature() && !seen[member] && !a.ctx.Index.Library(member) {
			seen[member] = true
			out = append(out, member)
		}
	}
	return out
}

// literalOf reads an enumeration-valued feature of sym as its literal name, or "".
func (a *libraryAudit) literalOf(sym *symbols.Symbol, feature, enumFQN string) string {
	values, present, err := a.model.DeclaredFeatureValues(sym, feature)
	if err != nil || !present {
		return ""
	}
	for _, v := range values {
		if lit := enumLiteral(v, enumFQN); lit != "" {
			return lit
		}
	}
	return ""
}

// report records a warning on sym's declaration.
func (a *libraryAudit) report(sym *symbols.Symbol, code, message string) {
	if sym == nil || sym.Decl == nil {
		return
	}
	a.diags = append(a.diags, diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     sym.Decl.Span(),
		Message:  message,
		Code:     code,
		Source:   a.source,
	})
}
