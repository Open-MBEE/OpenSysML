package passes

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func nameresCtx(t *testing.T, name, src string) (*Context, *ast.RootNamespace) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := newTestIndexFromDoc(name, root)
	return NewContext(name, idx, nil), root
}

func TestNameResolutionPassReportsUnresolved(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { alias A for P::Missing; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) == 0 {
		t.Fatalf("expected an unresolved diagnostic, got none")
	}
	d := got[0]
	if d.Source != "name-resolution" || d.Code != "unresolved" || d.Severity != diag.SeverityError {
		t.Fatalf("got %+v, want source=name-resolution code=unresolved severity=error", d)
	}
}

func TestNameResolutionPassCleanWhenAllResolve(t *testing.T) {
	ctx, root := nameresCtx(t, "a.sysml", "package P { namespace N; alias A for P::N; }")
	got := NameResolutionPass{}.Run(ctx, "a.sysml", root)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

func TestNameResolutionPassLevel(t *testing.T) {
	if (NameResolutionPass{}).Level() != LevelNameResolution {
		t.Fatalf("Level() = %v, want LevelNameResolution", (NameResolutionPass{}).Level())
	}
}

// parseDoc parses src into a RootNamespace, failing on any parse diagnostic.
func parseDoc(t *testing.T, name, src string) *ast.RootNamespace {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics in %s: %+v", name, p.Diagnostics)
	}
	return root
}

// Two documents each declare a top-level package P. Resolution in the global
// namespace is single-valued — KerML 8.2.3.5 resolves a qualified name to one
// membership or to none, its operations selecting `memberships->first()` — so a
// reference to P names the first document's P, and only what that P declares is
// reachable through it. Distinguishable naming constrains a Namespace's own
// members, and the global namespace is not one.
func TestNameResolutionPassResolvesARepeatedTopLevelNameToTheFirst(t *testing.T) {
	rootA := parseDoc(t, "a.sysml", "package P { namespace X; }")
	rootB := parseDoc(t, "b.sysml", "package P { namespace Y; }")
	rootC := parseDoc(t, "c.sysml", "package Q { alias A for P::X; }")
	rootD := parseDoc(t, "d.sysml", "package R { alias B for P::Y; }")

	idx := newTestIndex()
	idx.AddDocument("a.sysml", rootA)
	idx.AddDocument("b.sysml", rootB)
	idx.AddDocument("c.sysml", rootC)
	idx.AddDocument("d.sysml", rootD)

	got := NameResolutionPass{}.Run(NewContext("c.sysml", idx, nil), "c.sysml", rootC)
	if len(got) != 0 {
		t.Fatalf("got %+v, want P::X to resolve through the first P", got)
	}
	// The second P is not consulted, so its member is not reachable under P.
	got = NameResolutionPass{}.Run(NewContext("d.sysml", idx, nil), "d.sysml", rootD)
	if len(got) != 1 || got[0].Code != "unresolved" || got[0].Severity != diag.SeverityError {
		t.Fatalf("got %+v, want one unresolved error for P::Y", got)
	}
}

// nameresDiags runs the pass over one document and returns its diagnostics.
func nameresDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	return NameResolutionPass{}.Run(ctx, "a.sysml", root)
}

// Two owned members of one name are a name-conflict warning by default and, as
// a violation of validateNamespaceDistinguishibility (KerML 8.3.2.4.5), an
// error under strict conformance; the finding itself is the same in both modes.
func TestDuplicateOwnedMemberNamesEscalateUnderStrictConformance(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		part def A { attribute x : Real; }
		part def A { attribute y : Real; }
	}`
	for name, mode := range map[string]diag.ConformanceMode{"default": diag.ConformanceDefault, "strict": diag.ConformanceStrict} {
		got := nameresDiagsIn(t, src, mode)
		if len(got) != 2 {
			t.Fatalf("%s mode: got %+v, want two diagnostics", name, got)
		}
		for _, d := range got {
			if d.Code != "name-conflict" || d.Severity != strictSeverity(mode) || d.Message != "Duplicate of other owned member name" {
				t.Fatalf("%s mode: got %+v, want name-conflict findings of severity %v only", name, got, strictSeverity(mode))
			}
		}
	}
}

// nameresDiagsIn runs the name-resolution pass over src under mode, with the
// standard library indexed so metaclasses are known.
func nameresDiagsIn(t *testing.T, src string, mode diag.ConformanceMode) []diag.Diagnostic {
	t.Helper()
	sf := source.New("a.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := newTestIndexFromDoc("a.sysml", root)
	idx.ExpandWildcardImports()
	ctx := NewContextWithOptions("a.sysml", source.KindSysML, idx, nil, Options{Conformance: mode})
	return NameResolutionPass{}.Run(ctx, "a.sysml", root)
}

// Every membership a namespace keeps that is indistinguishable from another —
// owned, alias and inherited (Type::inheritedMembership subsets membership) —
// violates validateNamespaceDistinguishibility, so strict conformance reports
// each of the resolver's wordings for them as an error.
func TestIndistinguishableMembershipsAreErrorsUnderStrictConformance(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"owned repeats owned", `package P { part def A; part def A; }`,
			[]string{"Duplicate of other owned member name", "Duplicate of other owned member name"}},
		{"alias repeats owned", `package P { part def A; part def B; alias A for B; }`,
			[]string{"Duplicate of owned member name"}},
		{"alias repeats alias", `package P { part def B; part def C; alias A for B; alias A for C; }`,
			[]string{"Duplicate of other alias name", "Duplicate of other alias name"}},
		{"owned repeats inherited", `package P { part def Base { part x; } part def Sub :> Base { part x; } }`,
			[]string{"Duplicate of inherited member name 'x' from Base"}},
		{"inherited twice", `package P { part def L { part x; } part def R { part x; } part def Sub :> L, R; }`,
			[]string{"Duplicate of inherited member name 'x' from L, R"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []diag.ConformanceMode{diag.ConformanceDefault, diag.ConformanceStrict} {
				var got []string
				for _, d := range only(nameresDiagsIn(t, tc.src, mode), "name-conflict") {
					if d.Severity != strictSeverity(mode) {
						t.Errorf("%v: %q has severity %v, want %v", mode, d.Message, d.Severity, strictSeverity(mode))
					}
					got = append(got, d.Message)
				}
				if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
					t.Errorf("%v: got %q, want %q", mode, got, tc.want)
				}
			}
		})
	}
}

// An imported collision is a name KerML 7.2.5.4 hides, not a membership the
// namespace keeps, so the warning marking it stays a warning under strict.
func TestImportedNameCollisionStaysAWarningUnderStrictConformance(t *testing.T) {
	const src = `package A { part def Engine; }
	package B { part def Engine; }
	package C { private import A::*; private import B::*; part e : Engine; }`
	got := only(nameresDiagsIn(t, src, diag.ConformanceStrict), "name-conflict")
	if len(got) != 1 || got[0].Severity != diag.SeverityWarning || !strings.HasPrefix(got[0].Message, "Duplicate of imported member name 'Engine'") {
		t.Fatalf("got %+v, want one imported-name warning", got)
	}
}

// Two memberships whose member elements' metaclasses conform in neither
// direction are distinguishable whatever their names (KerML 8.3.2.4.3), so
// strict conformance has nothing to report on them.
func TestDistinguishableMetaclassesStayCleanUnderStrictConformance(t *testing.T) {
	for _, src := range []string{
		`package P { part def A; attribute def A; }`,
		`package P { part def Base { part x; } part def Sub :> Base { attribute x; } }`,
	} {
		if got := only(nameresDiagsIn(t, src, diag.ConformanceStrict), "name-conflict"); len(got) != 0 {
			t.Errorf("%s: got %+v, want none", src, got)
		}
	}
	// A specializing metaclass is not distinguishable: item def is a part def's general.
	got := only(nameresDiagsIn(t, `package P { part def A; item def A; }`, diag.ConformanceStrict), "name-conflict")
	if len(got) != 2 || got[0].Severity != diag.SeverityError {
		t.Fatalf("part def beside item def: got %+v, want two errors", got)
	}
}

// An unnamed parameter takes the effective name of the parameter it implicitly
// redefines, and that name resolves in the owning scope (KerML 7.3.4.5,
// SysML 7.6.5).
func TestImplicitlyRedefiningParameterBindsRedefinedName(t *testing.T) {
	got := nameresDiags(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = image;
		}
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics: image names the anonymous parameter", got)
	}
}

// resolvedInBody resolves name in the body scope of the member called fqn,
// after the pass has built the semantic model.
func resolvedInBody(t *testing.T, src, fqn, name string) (*symbols.Symbol, *symbols.Scope) {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	NameResolutionPass{}.Run(ctx, "a.sysml", root)
	owners := ctx.Index.LookupQualified(fqn)
	if len(owners) != 1 || owners[0].Scope == nil {
		t.Fatalf("looking up %s: got %d symbols with a body scope", fqn, len(owners))
	}
	sym, ok := ctx.Resolver().ResolveName(owners[0].Scope, name, nil)
	if !ok {
		t.Fatalf("%s does not resolve in %s", name, fqn)
	}
	return sym, owners[0].Scope
}

// The name binds the anonymous parameter itself, not the inherited one it
// takes the name from.
func TestImplicitlyRedefiningParameterIsWhatTheNameBinds(t *testing.T) {
	sym, scope := resolvedInBody(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = image;
		}
	}`, "P::shoot", "image")
	if sym.OwnerScope != scope {
		t.Fatalf("image resolved to %s outside the body, want the anonymous parameter", sym.Name)
	}
}

// Two implicit redefinitions need not agree on a name, so neither names the
// parameter and the inherited feature keeps the name (KerML 7.3.4.5).
func TestParameterRedefiningTwoInheritedOnesStaysAnonymous(t *testing.T) {
	sym, scope := resolvedInBody(t, `package P {
		action def B1 { in p1; }
		action def B2 { in p2; }
		action a : B1, B2 { in item; }
	}`, "P::a", "p1")
	if sym.OwnerScope == scope {
		t.Fatalf("p1 resolved to the anonymous parameter of a, want B1::p1")
	}
}

// The name is the redefined parameter's, not the keyword the parameter was
// declared with.
func TestImplicitlyRedefiningParameterDoesNotBindItsKeyword(t *testing.T) {
	got := nameresDiags(t, `package P {
		item def Image;
		action def Shoot { in image : Image; }
		action shoot : Shoot {
			in item;
			attribute x = item;
		}
	}`)
	if len(got) != 1 || got[0].Code != "unresolved" {
		t.Fatalf("got %+v, want one unresolved diagnostic for item", got)
	}
}

// A nested usage sharing a name with an inherited feature is indistinguishable
// from it, which the reference reports as a warning, not an error, and names the
// namespace it is inherited from (SysML 7.6.1, KerML 7.2.2; matched run against
// the pinned validator, w6c).
func TestNameResolutionPassReportsInheritedNameConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine; }
	}`)
	if len(got) != 1 {
		t.Fatalf("got %+v, want one diagnostic", got)
	}
	d := got[0]
	if d.Code != "name-conflict" || d.Source != "name-resolution" || d.Severity != diag.SeverityWarning {
		t.Fatalf("got %+v, want code=name-conflict source=name-resolution severity=warning", d)
	}
	if want := "Duplicate of inherited member name 'engine' from Vehicle"; d.Message != want {
		t.Fatalf("message = %q, want %q", d.Message, want)
	}
	if d.Span.Len != len("engine") {
		t.Fatalf("span = %+v, want the declared name's span", d.Span)
	}
}

// Redefining the inherited feature is how the name is legitimately reused.
func TestRedeclaredInheritedNameIsNoConflictWhenRedefined(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine :>> engine; }
		part w : Vehicle { part :>> engine; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A parameter redefines its inherited counterpart by position, and a case's
// features redefine theirs by name, so neither conflicts.
func TestInheritedNameConflictExemptsRedefiningFeatures(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Vehicle;
		action def Shoot { in image; }
		action shoot : Shoot { in image; }
		use case def Drive { subject vehicle : Vehicle; }
		use case drive : Drive { subject vehicle; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// `render r;` is named by the rendering it references (SysML
// RenderingUsage::namingFeature), so it is an owned member `r` that duplicates
// the inherited `r` it does not redefine, as the pilot reports.
func TestRenderReferenceToInheritedRenderingDuplicatesIt(t *testing.T) {
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		view def Base { rendering r : AsTree; }
		view def Derived :> Base { render r; }
	}`)
	if len(got) != 1 || got[0].Code != "name-conflict" || got[0].Severity != diag.SeverityWarning {
		t.Fatalf("got %+v, want one name-conflict warning", got)
	}
}

// A reference-named member whose target resolves to nothing binds no name, so
// repeating it is no duplicate and its spelling is offered for nothing.
func TestUnresolvedReferenceDerivedNamesAreNoDuplicates(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def H { action a; }
		part h : H {
			perform zz;
			perform zz;
			perform a;
		}
	}`)
	var codes []string
	for _, d := range got {
		codes = append(codes, d.Code)
		if d.Code == "unresolved" && strings.Contains(d.Message, "did you mean") {
			t.Errorf("%s: suggests a name nothing binds", d.Message)
		}
	}
	if want := []string{"unresolved", "unresolved", "name-conflict"}; !slices.Equal(codes, want) {
		t.Fatalf("got %+v, want %v: only the resolved reference duplicates the inherited a", got, want)
	}
}

// A reference-named member whose target is a definition binds no name either,
// against a sibling of the same spelling or an inherited member.
func TestNonFeatureReferenceDerivedNamesAreNoDuplicates(t *testing.T) {
	got := nameresDiags(t, `package P {
		action def A;
		part def H { attribute A; }
		part h : H {
			perform P::A;
			perform P::A;
		}
	}`)
	for _, d := range got {
		if d.Code == "name-conflict" {
			t.Fatalf("got %+v, want no name-conflict: a definition names no feature", got)
		}
	}
}

// The rendering `render r;` names inside a view of `H` is the one `H` renders;
// both redefine the library `viewRendering`, so the name is no duplicate there.
func TestRenderReferenceToOwnRenderingIsNoConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		view def H { render rendering r : AsTree; }
		view h : H { render r; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A `render` of another name redefines only the library `viewRendering`, so the
// inherited rendering and its members stay resolvable in the derived view.
func TestRenderOfAnotherNameLeavesInheritedRenderingResolvable(t *testing.T) {
	sym, _ := resolvedInBody(t, `package P {
		rendering def AsTree;
		rendering def AsTable;
		view def Base { render rendering r : AsTree { attribute a; } }
		view def Derived :> Base {
			render rendering s : AsTable;
			attribute x = r.a;
		}
		view v : Derived { attribute y = r.a; }
	}`, "P::Derived", "r")
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil || sym.OwnerScope.Owner().Name != "Base" {
		t.Fatalf("r resolved to %v, want Base::r", sym)
	}
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		rendering def AsTable;
		view def Base { render rendering r : AsTree { attribute a; } }
		view def Derived :> Base {
			render rendering s : AsTable;
			attribute x = r.a;
		}
		view v : Derived { attribute y = r.a; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A rendering the view declares for itself conflicts with an inherited one of
// the same name, exactly as any other declaration does.
func TestRenderDeclarationOfInheritedNameConflicts(t *testing.T) {
	got := nameresDiags(t, `package P {
		rendering def AsTree;
		view def Base { rendering r : AsTree; }
		view def Derived :> Base { render rendering r : AsTree; }
	}`)
	if len(got) != 1 || got[0].Code != "name-conflict" {
		t.Fatalf("got %+v, want one name-conflict diagnostic", got)
	}
}

// A concern and a viewpoint are requirements, so their bodies are exempt too.
func TestInheritedNameConflictExemptsConcernsAndViewpoints(t *testing.T) {
	got := nameresDiags(t, `package P {
		concern def C { stakeholder s; }
		concern c : C { stakeholder s; }
		viewpoint def V { stakeholder t; }
		viewpoint v : V { stakeholder t; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A name that is not inherited is not a conflict.
func TestDistinctNestedNameIsNoConflict(t *testing.T) {
	got := nameresDiags(t, `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part motor; }
	}`)
	if len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}
