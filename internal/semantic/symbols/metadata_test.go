package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestDeclaredMetadataUsageIsIndexed(t *testing.T) {
	root := build(t, `metadata def A;
	item p {
		@ m1 : A;
		@ m2 typed by A { }
		@ : A;
		@A;
	}
`)
	p, ok := root.LookupLocal("p")
	if !ok {
		t.Fatal("item p not found")
	}
	for _, name := range []string{"m1", "m2"} {
		sym, ok := p.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("declared metadata usage %s not indexed", name)
		}
		if sym.Kind != SymbolMetadataUsage {
			t.Fatalf("%s indexed as %v, want a metadata usage", name, sym.Kind)
		}
	}
	// The two usages without an identification name nothing.
	if names := p.Scope.MemberNames(); len(names) != 2 {
		t.Fatalf("members %v indexed, want only the two named usages", names)
	}
}

func TestMetadataBodyHasPrivateNestedScope(t *testing.T) {
	root := build(t, `metadata def A { attribute x; }
	item p {
		@A {
			x = 1;
			private attribute local;
			@A {
				attribute nested;
			}
		}
	}
`)
	p, ok := root.LookupLocal("p")
	if !ok {
		t.Fatal("item p not found")
	}
	usage, ok := p.Decl.(*ast.Usage)
	if !ok || len(usage.Members) != 1 {
		t.Fatal("annotation member not found")
	}
	member, ok := usage.Members[0].(*ast.Membership)
	if !ok {
		t.Fatal("annotation wrapper not found")
	}
	prefix, ok := member.Member.(*ast.PrefixMetadata)
	if !ok {
		t.Fatal("annotation prefix not found")
	}
	body := p.Scope.ChildFor(prefix)
	if body == nil || !body.BodyLocal() {
		t.Fatal("annotation body scope is missing or not body-local")
	}
	if _, ok := body.LookupLocal("x"); !ok {
		t.Fatal("annotation body member x not indexed")
	}
	if _, ok := p.Scope.LookupLocal("local"); ok {
		t.Fatal("annotation body member leaked into enclosing scope")
	}
	var nestedPrefix *ast.PrefixMetadata
	for _, member := range prefix.Body {
		if membership, ok := member.(*ast.Membership); ok {
			nestedPrefix, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if nestedPrefix == nil {
		t.Fatal("nested annotation prefix not found")
	}
	nestedBody := body.ChildFor(nestedPrefix)
	if nestedBody == nil || !nestedBody.BodyLocal() {
		t.Fatal("nested annotation body scope is missing or not body-local")
	}
	if _, ok := nestedBody.LookupLocal("nested"); !ok {
		t.Fatal("nested annotation body member not indexed")
	}
	attached := &ast.Usage{
		Ident: ast.Identification{Name: "attached"},
	}
	prefixUsage := &ast.Usage{
		Ident: ast.Identification{Name: "q"},
		Prefixes: []*ast.PrefixMetadata{{
			Body: []ast.Node{attached},
		}},
	}
	prefixRoot := Build(&ast.RootNamespace{Members: []ast.Node{prefixUsage}})
	_, ok = prefixRoot.LookupLocal("q")
	if !ok {
		t.Fatal("prefixed item not indexed")
	}
	qBody := prefixRoot.ChildFor(prefixUsage.Prefixes[0])
	if qBody == nil || !qBody.BodyLocal() {
		t.Fatal("attached annotation body scope is missing or not body-local")
	}
	if _, ok := qBody.LookupLocal("attached"); !ok {
		t.Fatal("attached annotation body member not indexed")
	}
}
