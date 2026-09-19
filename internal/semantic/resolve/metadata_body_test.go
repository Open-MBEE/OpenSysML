package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestMetadataBodyMissingFeatureDoesNotBecomeUnresolved(t *testing.T) {
	r := resolveDoc(t, "metadata.sysml", `metadata def A;
	item p {
		@A {
			missing = 1;
		}
	}
`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("metadata body declaration produced diagnostics: %v", r.Diagnostics)
	}
}

func TestMetadataBodyDeclarationImplicitlyRedefinesMetadataFeature(t *testing.T) {
	r := resolveDocWithModel(t, "metadata.sysml", `metadata def A {
		attribute x;
	}
	item p {
		attribute outer;
		@A {
			x = outer;
		}
	}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("metadata body resolution diagnostics: %v", r.Diagnostics)
	}
	p, ok := r.idx.DocumentRoot("metadata.sysml").LookupLocal("p")
	if !ok {
		t.Fatal("item p not indexed")
	}
	usage, ok := p.Decl.(*ast.Usage)
	if !ok {
		t.Fatal("item p declaration not found")
	}
	var annotation *ast.PrefixMetadata
	for _, member := range usage.Members {
		if membership, ok := member.(*ast.Membership); ok {
			annotation, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if annotation == nil {
		t.Fatal("annotation prefix not found")
	}
	body := p.Scope.ChildFor(annotation)
	if body == nil {
		t.Fatal("annotation body scope not found")
	}
	x, ok := body.LookupLocal("x")
	if !ok {
		t.Fatal("body declaration x not indexed")
	}
	targets := r.redefinedFeatures(x)
	if len(targets) != 1 || targets[0].Name != "x" {
		t.Fatalf("body declaration redefinition = %v, want metadata feature x", targets)
	}
	value, ok := annotation.Body[0].(*ast.Membership)
	if !ok {
		t.Fatal("body declaration wrapper not found")
	}
	valueUsage, ok := value.Member.(*ast.Usage)
	if !ok {
		t.Fatal("body declaration not a usage")
	}
	ref, ok := valueUsage.Value.(*ast.FeatureReference)
	if !ok || ref.Name == nil {
		t.Fatal("body value reference not found")
	}
	resolved, ok := r.ResolveQualified(body, ref.Name)
	if !ok || resolved.Name != "outer" {
		t.Fatalf("body value resolved to %v, %v; want enclosing feature outer", resolved, ok)
	}
}

func TestMetadataBodyExplicitRedefinitionSurvivesImplicitTarget(t *testing.T) {
	r := resolveDocWithModel(t, "metadata.sysml", `metadata def A {
		attribute x;
		attribute y;
	}
	item p {
		@A {
			x :>> A::y = 1;
		}
	}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("metadata body resolution diagnostics: %v", r.Diagnostics)
	}
	p, ok := r.idx.DocumentRoot("metadata.sysml").LookupLocal("p")
	if !ok {
		t.Fatal("item p not indexed")
	}
	usage, ok := p.Decl.(*ast.Usage)
	if !ok {
		t.Fatal("item p declaration not found")
	}
	var annotation *ast.PrefixMetadata
	for _, member := range usage.Members {
		if membership, ok := member.(*ast.Membership); ok {
			annotation, _ = membership.Member.(*ast.PrefixMetadata)
		}
	}
	if annotation == nil {
		t.Fatal("annotation prefix not found")
	}
	body := p.Scope.ChildFor(annotation)
	if body == nil {
		t.Fatal("annotation body scope not found")
	}
	x, ok := body.LookupLocal("x")
	if !ok {
		t.Fatal("body declaration x not indexed")
	}
	targets := r.redefinedFeatures(x)
	var hasExplicit, hasImplicit bool
	for _, target := range targets {
		switch target.Name {
		case "y":
			hasExplicit = true
		case "x":
			hasImplicit = true
		}
	}
	if !hasExplicit || !hasImplicit {
		t.Fatalf("body declaration redefinition = %v, want explicit y and implicit x targets", targets)
	}
}

func TestMetadataBodyOnAssumeResolvesValue(t *testing.T) {
	qn := func(name string) *ast.QualifiedName {
		return &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name}}}
	}
	metadataFeature := &ast.Usage{
		Kind:  ast.UsageAttribute,
		Ident: ast.Identification{Name: "y"},
	}
	bodyValue := &ast.Usage{
		Kind:  ast.UsageAttribute,
		Ident: ast.Identification{Name: "x"},
		Value: &ast.FeatureReference{Name: qn("y")},
	}
	prefix := &ast.PrefixMetadata{
		Type: qn("A"),
		Body: []ast.Node{
			&ast.Membership{Member: bodyValue},
		},
	}
	assume := &ast.AssumeMember{Prefixes: []*ast.PrefixMetadata{prefix}}
	root := &ast.RootNamespace{
		Members: []ast.Node{
			&ast.Membership{
				Member: &ast.Definition{
					Kind:  ast.DefMetadata,
					Ident: ast.Identification{Name: "A"},
					Members: []ast.Node{
						&ast.Membership{Member: metadataFeature},
					},
				},
			},
			&ast.Membership{
				Member: &ast.Definition{
					Kind:  ast.DefRequirement,
					Ident: ast.Identification{Name: "R"},
					Members: []ast.Node{
						&ast.Membership{Member: assume},
					},
				},
			},
		},
	}
	idx := symbols.NewIndexFromDoc("metadata.sysml", root)
	r := New(idx)
	r.SetModel(localMemberModel{})
	r.ResolveDocument("metadata.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("assume metadata body resolution diagnostics: %v", r.Diagnostics)
	}
	requirement, ok := idx.DocumentRoot("metadata.sysml").LookupLocal("R")
	if !ok || requirement.Scope == nil {
		t.Fatal("requirement scope not found")
	}
	body := requirement.Scope.ChildFor(prefix)
	if body == nil || body.Owner() == nil {
		t.Fatal("assume metadata body scope was not owned by its metadata definition")
	}
	resolved, ok := r.ResolveQualified(body, bodyValue.Value.(*ast.FeatureReference).Name)
	if !ok || resolved == nil || resolved.Decl != metadataFeature {
		t.Fatalf("assume metadata body value resolved to %v, %v; want metadata feature y %v",
			resolved, ok, metadataFeature)
	}
}
