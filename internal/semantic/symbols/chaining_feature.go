package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// ChainingFeature is the feature a relationship's chain target (`:>> a.b`)
// denotes as a whole: the implicit chaining feature of KerML 8.3.3.3.9, which
// the relationship object targets and owns. The symbol that carries it is
// synthesized by the reflective layer and is never registered in a scope or
// index.
type ChainingFeature struct {
	Relationship *Symbol               // the relationship object whose target it is
	Node         *ast.FeatureChainExpr // the chain written; nil when the owner is recorded
}
