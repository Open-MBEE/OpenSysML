package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// endKind is the metaclass a KerML relationship end must be an instance of
// (KerML 1.0 §8.3.3, §8.3.4): a Type, a Classifier or a Feature.
type endKind int

const (
	endType endKind = iota
	endClassifier
	endFeature
)

func (e endKind) String() string {
	switch e {
	case endClassifier:
		return "a classifier"
	case endFeature:
		return "a feature"
	}
	return "a type"
}

// admits reports whether a symbol of kind k may stand at an end of this kind.
// An unclassified kind constrains nothing, as on the declaration path.
func (e endKind) admits(k symbols.SymbolKind) bool {
	if k == symbols.SymbolUnknown {
		return true
	}
	switch e {
	case endClassifier:
		return k.IsDefinition()
	case endFeature:
		return k.IsFeature()
	}
	return isTypeKind(k)
}

// relationshipEnds is the pair of end kinds a keyword-first relationship
// relates, its Source first.
type relationshipEnds struct {
	source, target endKind
}

// relationshipMemberEnds maps a keyword-first relationship, by its keyword, to
// the metaclasses its ends must be (KerML 1.0 §8.3.3, §8.3.4).
var relationshipMemberEnds = map[string]relationshipEnds{
	"subtype":       {endType, endType},
	"subclassifier": {endClassifier, endClassifier},
	"typing":        {endFeature, endType},
	"subset":        {endFeature, endFeature},
	"redefinition":  {endFeature, endFeature},
	"conjugate":     {endType, endType},
	"inverse":       {endFeature, endFeature},
	"disjoint":      {endType, endType},
	"featuring":     {endFeature, endType},
}

// checkRelationshipMember checks both named ends of a keyword-first relationship
// against the kinds its metaclass relates; a declaration clause's target is compatMessage's.
func (tc *typeChecker) checkRelationshipMember(scope *symbols.Scope, rel *ast.RelationshipMember) {
	ends, ok := relationshipMemberEnds[rel.Keyword]
	if !ok {
		return
	}
	tc.checkRelationshipEnd(scope, rel, rel.Source, "source", ends.source)
	tc.checkRelationshipEnd(scope, rel, rel.Target, "target", ends.target)
}

func (tc *typeChecker) checkRelationshipEnd(scope *symbols.Scope, rel *ast.RelationshipMember, end ast.Node, role string, want endKind) {
	if end == nil {
		return
	}
	tc.checkChainSegments(scope, end)
	sym, ok := tc.resolver.ResolveTarget(scope, end)
	if !ok || sym == nil {
		return // unresolved: name-resolution tier owns this
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := tc.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			sym = resolved
		}
	}
	if want.admits(sym.Kind) {
		return
	}
	tc.appendUnique(diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     end.Span(),
		Message:  fmt.Sprintf("%s %s must be %s, found %s", rel.Keyword, role, want, sym.Kind),
		Code:     "type",
		Source:   "type",
	})
}
