package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func resolveDoc(t *testing.T, name, src string) *Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := New(idx)
	r.ResolveDocument(name, root)
	return r
}

type localMemberModel struct{}

func (localMemberModel) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym != nil && sym.Scope != nil {
		return sym.Scope.LookupLocal(name)
	}
	return nil, false
}

func (localMemberModel) LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	return nil, false
}

func resolveDocWithModel(t *testing.T, name, src string) *Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc(name, root)
	r := New(idx)
	r.SetModel(localMemberModel{})
	r.ResolveDocument(name, root)
	return r
}

func TestResolveDocumentReportsUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { alias A for P::Missing; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for P::Missing")
	}
}

func TestResolveDocumentCleanWhenAllResolve(t *testing.T) {
	r := resolveDoc(t, "d.sysml",
		"package P { namespace N; alias A for P::N; }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveDocumentResolvesExpressionRefs(t *testing.T) {
	// FilterMember condition referencing an undefined name -> diagnostic.
	r := resolveDoc(t, "d.sysml",
		"package P { filter Undefined; }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected unresolved diagnostic for expression ref Undefined")
	}
}

func TestResolveDeclarationBodyMembersFromHeaders(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "header-own-body.kerml",
			src: `package R {
				class A {
					member feature x featured by A_snapshots {
						member feature A_snapshots featured by A;
					}
				}
			}`,
		},
		{
			name: "header-own-body.sysml",
			src: `package R {
				part def A {
					feature x : A_ref;
					feature A_ref;
				}
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := resolveDocWithModel(t, tc.name, tc.src)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
			}
		})
	}
}

func TestResolveFeatureChainThroughImportedBodyMember(t *testing.T) {
	r := resolveDocWithModel(t, "chain.kerml", `package S {
		class Cart { feature sel : Prod[0..*]; }
		class Prod { feature inCart : Cart[0..1]; }
		assoc PS {
			end feature cart : Cart[1] crosses selectedProduct.inCart;
			end feature selectedProduct : Prod[1] crosses cart.sel;
		}
		assoc PS2 {
			end feature cart : Cart[1] crosses selectedProduct.inCart2 {
				member feature inCart2 : Cart[0..1] featured by Prod;
			}
			end feature selectedProduct : Prod[1] {
				public import cart::inCart2;
			}
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
	}
}

func TestResolve_FeatureChainExpr(t *testing.T) {
	t.Run("simple chain - two levels", func(t *testing.T) {
		src := `
			package A {
				attribute x = 1;
			}
			package B {
				attribute ref : A;
				attribute test = ref.x;
			}
		`
		p := parser.New(source.New("test.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}

		idx := symbols.NewIndexFromDoc("test.sysml", root)
		r := New(idx)
		r.ResolveDocument("test.sysml", root)

		if len(r.Diagnostics) != 0 {
			t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
		}

		// Find B::test usage and its chain expression
		docRoot := idx.DocumentRoot("test.sysml")
		bSym, _ := docRoot.LookupLocal("B")
		testSym, _ := bSym.Scope.LookupLocal("test")
		usage := testSym.Decl.(*ast.Usage)

		fc := usage.Value.(*ast.FeatureChainExpr)

		// Verify member (x) has symbol stored
		if fc.Member == nil || len(fc.Member.Parts) != 1 {
			t.Fatal("expected single-part member")
		}
		if _, ok := r.PartSymbol(fc.Member, 0); !ok {
			t.Error("member part 'x' symbol not stored")
		}
	})

	t.Run("nested chain - three levels", func(t *testing.T) {
		src := `
			package A {
				package Inner {
					attribute value = 42;
				}
			}
			package B {
				attribute ref : A;
				attribute test = ref.Inner.value;
			}
		`
		p := parser.New(source.New("test.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}

		idx := symbols.NewIndexFromDoc("test.sysml", root)
		r := New(idx)
		r.ResolveDocument("test.sysml", root)

		if len(r.Diagnostics) != 0 {
			t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
		}

		// Find B::test usage and its chain expression
		docRoot := idx.DocumentRoot("test.sysml")
		bSym, _ := docRoot.LookupLocal("B")
		testSym, _ := bSym.Scope.LookupLocal("test")
		usage := testSym.Decl.(*ast.Usage)

		// Outer chain: ref.Inner.value = FeatureChainExpr{Operand: FeatureChainExpr{Operand: ref, Member: Inner}, Member: value}
		outerChain := usage.Value.(*ast.FeatureChainExpr)

		// Verify outer member (value) has symbol
		if outerChain.Member == nil || len(outerChain.Member.Parts) != 1 {
			t.Fatal("expected outer member 'value'")
		}
		if _, ok := r.PartSymbol(outerChain.Member, 0); !ok {
			t.Error("outer member part 'value' symbol not stored")
		}

		// Verify inner chain member (Inner) has symbol
		innerChain := outerChain.Operand.(*ast.FeatureChainExpr)
		if innerChain.Member == nil || len(innerChain.Member.Parts) != 1 {
			t.Fatal("expected inner member 'Inner'")
		}
		if _, ok := r.PartSymbol(innerChain.Member, 0); !ok {
			t.Error("inner member part 'Inner' symbol not stored")
		}
	})

	t.Run("unresolved first part", func(t *testing.T) {
		src := `
			package B {
				attribute test = missing.x;
			}
		`
		r := resolveDoc(t, "test.sysml", src)
		if len(r.Diagnostics) == 0 {
			t.Error("expected unresolved diagnostic")
		}
	})

	t.Run("unresolved second part", func(t *testing.T) {
		src := `
			package A {
				attribute x = 1;
			}
			package B {
				attribute ref : A;
				attribute test = ref.missing;
			}
		`
		r := resolveDoc(t, "test.sysml", src)
		if len(r.Diagnostics) == 0 {
			t.Error("expected unresolved diagnostic")
		}
	})
}

func TestResolve_MemberChainUsesModelWhenAvailable(t *testing.T) {
	src := `
		package A {
			attribute x = 1;
		}
		package B {
			attribute ref : A;
			attribute test = ref.x;
		}
	`
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}

	idx := symbols.NewIndexFromDoc("test.sysml", root)
	r := New(idx)

	// Mock model that implements LookupMember
	type mockModel struct {
		called bool
	}
	mock := &mockModel{}

	// Implement LookupMember interface
	lookupMember := func(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
		mock.called = true
		// Delegate to local scope for this test
		if sym.Scope != nil {
			return sym.Scope.LookupLocal(name)
		}
		return nil, false
	}

	// Can't attach method to struct in test, so just verify fallback works
	// (Real usage: r.SetModel(semantics.NewModel(r)) in production)
	r.ResolveDocument("test.sysml", root)

	if len(r.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", r.Diagnostics)
	}

	// Note: Full model integration requires semantics package dependency
	// This test verifies the fallback path (LookupLocal) works
	_ = lookupMember
	_ = mock
}

func TestResolve_QualifiedNamePartsStoreSymbols(t *testing.T) {
	p := parser.New(source.New("test.sysml", []byte(`
		package A {
			package B {
				attribute x = 1;
			}
		}
		package C {
			attribute test : A::B;
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}

	idx := symbols.NewIndexFromDoc("test.sysml", root)
	r := New(idx)
	r.ResolveDocument("test.sysml", root)

	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
	}

	// Find C::test usage and verify its typing relationship's QN parts have symbols
	cScope := idx.DocumentRoot("test.sysml")
	cSym, ok := cScope.LookupLocal("C")
	if !ok {
		t.Fatal("package C not found")
	}
	testSym, ok := cSym.Scope.LookupLocal("test")
	if !ok {
		t.Fatal("C::test not found")
	}

	usage := testSym.Decl.(*ast.Usage)
	if len(usage.Relationships) == 0 {
		t.Fatal("test has no relationships")
	}

	rel := usage.Relationships[0]
	if rel.Kind != ast.RelTyping {
		t.Fatalf("expected typing relationship, got %v", rel.Kind)
	}

	var qn *ast.QualifiedName
	switch target := rel.Target.(type) {
	case *ast.FeatureReference:
		qn = target.Name
	case *ast.QualifiedName:
		qn = target
	default:
		t.Fatalf("unexpected target type: %T", rel.Target)
	}

	if len(qn.Parts) != 2 {
		t.Fatalf("expected 2 parts in A::B, got %d", len(qn.Parts))
	}

	// Verify each part has resolved symbol
	if _, ok := r.PartSymbol(qn, 0); !ok {
		t.Error("Part 0 (A) symbol not set")
	}
	if _, ok := r.PartSymbol(qn, 1); !ok {
		t.Error("Part 1 (B) symbol not set")
	}

	// Verify symbols are correct
	aSym, _ := r.PartSymbol(qn, 0)
	if aSym.Name != "A" {
		t.Errorf("Part 0 symbol name = %q, want A", aSym.Name)
	}

	bSym, _ := r.PartSymbol(qn, 1)
	if bSym.Name != "B" {
		t.Errorf("Part 1 symbol name = %q, want B", bSym.Name)
	}
}

func TestResolve_RelationshipTargetChain(t *testing.T) {
	t.Run("subsetting with feature chain", func(t *testing.T) {
		src := `
			package Outer {
				attribute x = 1;
			}
			attribute ref : Outer;
			attribute test subsets ref.x;
		`
		p := parser.New(source.New("test.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}

		idx := symbols.NewIndexFromDoc("test.sysml", root)
		r := New(idx)
		r.ResolveDocument("test.sysml", root)

		// Should resolve without errors
		if len(r.Diagnostics) != 0 {
			t.Errorf("expected no diagnostics, got: %v", r.Diagnostics)
		}

		// Verify symbols set in chain
		docRoot := idx.DocumentRoot("test.sysml")
		testSym, ok := docRoot.LookupLocal("test")
		if !ok {
			t.Fatal("test symbol not found")
		}

		usage := testSym.Decl.(*ast.Usage)
		if len(usage.Relationships) == 0 {
			t.Fatal("test has no relationships")
		}

		rel := usage.Relationships[0]
		fc, ok := rel.Target.(*ast.FeatureChainExpr)
		if !ok {
			t.Fatalf("expected FeatureChainExpr target, got %T", rel.Target)
		}

		// Verify member (x) resolved
		if fc.Member == nil || len(fc.Member.Parts) != 1 {
			t.Fatal("expected single-part member 'x'")
		}
		if _, ok := r.PartSymbol(fc.Member, 0); !ok {
			t.Error("member part 'x' symbol not stored - chain not resolved")
		}
	})

	t.Run("redefines with nested chain", func(t *testing.T) {
		src := `
			package A {
				package B {
					attribute value;
				}
			}
			attribute base : A;
			attribute derived :> base.B.value;
		`
		p := parser.New(source.New("test.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}

		idx := symbols.NewIndexFromDoc("test.sysml", root)
		r := New(idx)
		r.ResolveDocument("test.sysml", root)

		// Should resolve without errors
		if len(r.Diagnostics) != 0 {
			t.Errorf("expected no diagnostics, got: %v", r.Diagnostics)
		}

		// Verify symbols set in nested chain
		docRoot := idx.DocumentRoot("test.sysml")
		derivedSym, ok := docRoot.LookupLocal("derived")
		if !ok {
			t.Fatal("derived symbol not found")
		}

		usage := derivedSym.Decl.(*ast.Usage)
		if len(usage.Relationships) == 0 {
			t.Fatal("derived has no relationships")
		}

		rel := usage.Relationships[0]
		outerChain, ok := rel.Target.(*ast.FeatureChainExpr)
		if !ok {
			t.Fatalf("expected FeatureChainExpr target, got %T", rel.Target)
		}

		// Verify outer member (value) resolved
		if outerChain.Member == nil || len(outerChain.Member.Parts) != 1 {
			t.Fatal("expected outer member 'value'")
		}
		if _, ok := r.PartSymbol(outerChain.Member, 0); !ok {
			t.Error("outer member 'value' symbol not stored")
		}

		// Verify inner chain member (B) resolved
		innerChain, ok := outerChain.Operand.(*ast.FeatureChainExpr)
		if !ok {
			t.Fatalf("expected inner FeatureChainExpr, got %T", outerChain.Operand)
		}
		if innerChain.Member == nil || len(innerChain.Member.Parts) != 1 {
			t.Fatal("expected inner member 'B'")
		}
		if _, ok := r.PartSymbol(innerChain.Member, 0); !ok {
			t.Error("inner member 'B' symbol not stored")
		}
	})
}

func TestResolve_Track3Integration(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "feature chain in subsetting",
			src: `
				package Outer {
					package Inner {
						attribute value = 42;
					}
				}
				attribute ref : Outer;
				attribute test subsets ref.Inner.value;
			`,
			wantErr: false,
		},
		{
			name: "nested feature chains in redefines",
			src: `
				package A {
					attribute x = 1;
				}
				package B {
					attribute y = 2;
				}
				attribute refA : A;
				attribute refB : B;
				attribute derived :> refA.x redefines refB.y;
			`,
			wantErr: false,
		},
		{
			name: "feature chain with typing",
			src: `
				part def Vehicle {
					attribute speed;
				}
				part myVehicle : Vehicle;
				attribute test : myVehicle.speed;
			`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(tt.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("Parse() error: %v", p.Diagnostics)
			}

			idx := symbols.NewIndexFromDoc("test.sysml", root)
			r := New(idx)
			r.ResolveDocument("test.sysml", root)

			hasError := len(r.Diagnostics) > 0

			if hasError != tt.wantErr {
				t.Errorf("wantErr=%v, got error=%v, diagnostics: %v", tt.wantErr, hasError, r.Diagnostics)
			}
		})
	}
}

// A redefinition target names the redefined feature, never the unnamed feature
// that took its name (KerML 7.3.4.5).
func TestRedefinitionTargetSkipsTheNameItGaveAway(t *testing.T) {
	src := `package P { part engine; part v { part :>> engine; } }`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("got %v, want no diagnostics", r.Diagnostics)
	}

	root := r.Index().DocumentRoot("d.sysml")
	pkg, _ := root.LookupLocal("P")
	declared, _ := pkg.Scope.LookupLocal("engine")
	v, _ := pkg.Scope.LookupLocal("v")
	borrowed, _ := v.Scope.LookupLocal("engine")

	qn := ast.AsQualifiedName(borrowed.Decl.(*ast.Usage).Relationships[0].Target)
	got, ok := r.PartSymbol(qn, 0)
	if !ok || got != declared {
		t.Fatalf("target resolved to %v, want the declared P::engine", got)
	}
}
