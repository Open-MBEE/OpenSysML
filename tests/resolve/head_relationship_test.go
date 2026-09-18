package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The document walk reads `featured by startShot` in the feature's own scope,
// where its type contributes Occurrence::startShot, ahead of the enclosing
// portion that redefines startShot; a collected reference must do the same.
const headRelationshipModel = `package P {
	class Occurrence {
		feature startShot : Occurrence;
		feature snapshots : Occurrence;
	}
	class CC1 :> Occurrence {
		portion :>> startShot {
			feature snaps :>> snapshots featured by startShot;
		}
	}
}`

func TestHeadRelationshipReferenceResolvesWhereTheDocumentDid(t *testing.T) {
	walk, root, rootScope := resolvedDocNamed(t, "app.kerml", headRelationshipModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}

	var featuredBy *resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		ref := ref
		if nameText(ref.QN) == "startShot" && !ref.Redefines && ref.Head != nil {
			featuredBy = &ref
		}
	}
	if featuredBy == nil {
		t.Fatal("`featured by startShot` was not collected as a head relationship")
	}

	walked, ok := walk.PartSymbol(featuredBy.QN, 0)
	if !ok {
		t.Fatal("the document walk did not bind `featured by startShot`")
	}
	if !isFeature(walked.Decl) || isPortion(walked.Decl) {
		t.Fatalf("the document walk bound `featured by startShot` to %T, want the inherited feature", walked.Decl)
	}

	sym, ok := walk.ResolveReference(*featuredBy)
	if !ok {
		t.Fatal("`featured by startShot` does not resolve on its own")
	}
	if sym != walked {
		t.Errorf("`featured by startShot` resolves to a different declaration on its own than in the document walk")
	}

	// Spelled through the class, the same occurrence names the redefining
	// portion, which the header does not hold: the enclosing scope decides.
	qualified := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "CC1"}, {Text: "startShot"}}}
	portion, ok := walk.ProbeReference(featuredBy.Spelled(qualified))
	if !ok {
		t.Fatal("`featured by CC1::startShot` does not resolve")
	}
	if !isPortion(portion.Decl) {
		t.Errorf("`featured by CC1::startShot` resolves to %T, want the portion redefining startShot", portion.Decl)
	}
}

// An end's cross feature has no scope of its own: its head relationships are
// read in the end's scope, as the end's are, where a chain's root may be a
// member of the end's body ahead of the sibling end of the same name.
const crossFeatureHeadModel = `package P {
	class Item;
	class Cart {
		feature items : Item;
	}
	class Product;
	assoc A {
		end feature cart : Cart;
		end items :> cart.items feature product : Product {
			feature cart : Cart;
		}
	}
}`

func TestCrossFeatureHeadReferenceResolvesWhereTheDocumentDid(t *testing.T) {
	walk, root, rootScope := resolvedDocNamed(t, "app.kerml", crossFeatureHeadModel)
	if len(walk.Diagnostics) != 0 {
		t.Fatalf("the document walk must resolve every name: %v", walk.Diagnostics)
	}

	var chainRoot *resolve.Reference
	for _, ref := range resolve.References(root, rootScope) {
		ref := ref
		if nameText(ref.QN) == "cart" {
			chainRoot = &ref
		}
	}
	if chainRoot == nil {
		t.Fatal("the root of `:> cart.items` was not collected")
	}
	if _, ok := chainRoot.Member.(*ast.CrossFeatureMember); !ok || chainRoot.Head == nil {
		t.Fatalf("the chain root should be collected as a head relationship of the cross feature, got member %T, head %v", chainRoot.Member, chainRoot.Head)
	}

	walked, ok := walk.PartSymbol(chainRoot.QN, 0)
	if !ok {
		t.Fatal("the document walk did not bind the root of `:> cart.items`")
	}
	if !declaredInEnd(walked, "product") {
		t.Fatalf("the document walk bound `cart` outside the end's body: %v", walked.Decl)
	}

	sym, ok := walk.ResolveReference(*chainRoot)
	if !ok {
		t.Fatal("the root of `:> cart.items` does not resolve on its own")
	}
	if sym != walked {
		t.Errorf("the root of `:> cart.items` resolves to a different declaration on its own than in the document walk")
	}
}

// declaredInEnd reports whether sym is a body member of the end usage named end.
func declaredInEnd(sym *symbols.Symbol, end string) bool {
	if sym.OwnerScope == nil {
		return false
	}
	u, ok := sym.OwnerScope.Node().(*ast.Usage)
	return ok && u.IsEnd && u.Ident.Name == end
}

func isFeature(n ast.Node) bool {
	_, ok := n.(*ast.Usage)
	return ok
}

func isPortion(n ast.Node) bool {
	u, ok := n.(*ast.Usage)
	return ok && u.IsPortion
}
