package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func build(t *testing.T, src string) *Scope {
	t.Helper()
	sf := source.New("test", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", p.Diagnostics)
	}
	return Build(root)
}

func TestBuildTopLevelPackage(t *testing.T) {
	root := build(t, "package P;")
	sym, ok := root.LookupLocal("P")
	if !ok {
		t.Fatalf("P not found in root scope")
	}
	if sym.Kind != SymbolPackage {
		t.Fatalf("P kind = %v, want package", sym.Kind)
	}
	if _, isPkg := sym.Decl.(*ast.Package); !isPkg {
		t.Fatalf("P Decl type = %T, want *ast.Package", sym.Decl)
	}
}

func TestBuildNestedMembers(t *testing.T) {
	root := build(t, "package Outer { package Inner; namespace N; }")
	outer, ok := root.LookupLocal("Outer")
	if !ok {
		t.Fatalf("Outer not found")
	}
	outerScope := outer.Scope
	if outerScope == nil {
		t.Fatalf("Outer has no child scope")
	}
	if _, ok := outerScope.LookupLocal("Inner"); !ok {
		t.Fatalf("Inner not found in Outer scope")
	}
	nsym, ok := outerScope.LookupLocal("N")
	if !ok || nsym.Kind != SymbolNamespace {
		t.Fatalf("N not found as namespace in Outer scope")
	}
}

func TestBuildShortAndPrimaryNameKeys(t *testing.T) {
	root := build(t, "package <p> Primary;")
	for _, key := range []string{"p", "Primary"} {
		sym, ok := root.LookupLocal(key)
		if !ok {
			t.Fatalf("key %q not found", key)
		}
		if sym.Kind != SymbolPackage {
			t.Fatalf("key %q kind = %v, want package", key, sym.Kind)
		}
	}
	// Both keys must map to the same symbol.
	a, _ := root.LookupLocal("p")
	b, _ := root.LookupLocal("Primary")
	if a != b {
		t.Fatalf("short and primary keys map to different symbols")
	}
}

func TestBuildVisibilityCarried(t *testing.T) {
	root := build(t, "private package Secret;")
	sym, ok := root.LookupLocal("Secret")
	if !ok {
		t.Fatalf("Secret not found")
	}
	if sym.Visibility != ast.VisibilityPrivate {
		t.Fatalf("Secret visibility = %v, want private", sym.Visibility)
	}
}

func TestBuildAliasSymbol(t *testing.T) {
	root := build(t, "package P; alias A for P;")
	sym, ok := root.LookupLocal("A")
	if !ok || sym.Kind != SymbolAlias {
		t.Fatalf("alias A not found as alias symbol")
	}
}

// A named multiplicity owns its body, so the members declared there are its own.
func TestBuildMultiplicityBodyScope(t *testing.T) {
	root := build(t, "package P { multiplicity m [1..2] { feature f; } multiplicity n subsets m; }")
	pkg, _ := root.LookupLocal("P")
	m, ok := pkg.Scope.LookupLocal("m")
	if !ok || m.Kind != SymbolMultiplicity {
		t.Fatalf("m not found as multiplicity symbol")
	}
	if m.Scope == nil {
		t.Fatalf("m has no child scope")
	}
	if f, ok := m.Scope.LookupLocal("f"); !ok || f.OwnerScope != m.Scope {
		t.Fatalf("f not owned by m's body")
	}
	n, ok := pkg.Scope.LookupLocal("n")
	if !ok || n.Kind != SymbolMultiplicity || n.Scope == nil {
		t.Fatalf("n not found as multiplicity symbol with a scope")
	}
	if _, ok := n.Scope.LookupLocal("f"); ok {
		t.Fatalf("n's empty body must not see m's members")
	}
}

func TestBuildErrorNodeSkipped(t *testing.T) {
	// Unknown declaration keyword yields an ErrorNode; builder must not panic
	// and must still register the good package.
	root := build(t, "package Good;")
	if _, ok := root.LookupLocal("Good"); !ok {
		t.Fatalf("Good not registered")
	}
}

func TestBuildDefinitionAndNestedUsages(t *testing.T) {
	src := "part def Car { part engine; attribute mass; }"
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	scope := Build(root)

	syms := scope.LookupLocalAll("Car")
	if len(syms) != 1 {
		t.Fatalf("expected 1 Car symbol, got %d", len(syms))
	}
	car := syms[0]
	if car.Kind != SymbolPartDef {
		t.Fatalf("Car kind = %v, want SymbolPartDef", car.Kind)
	}
	if car.Scope == nil {
		t.Fatalf("Car should own a child scope")
	}
	if len(car.Scope.LookupLocalAll("engine")) != 1 {
		t.Fatalf("engine not registered in Car scope")
	}
	eng := car.Scope.LookupLocalAll("engine")[0]
	if eng.Kind != SymbolPartUsage {
		t.Fatalf("engine kind = %v, want SymbolPartUsage", eng.Kind)
	}
	if len(car.Scope.LookupLocalAll("mass")) != 1 {
		t.Fatalf("mass not registered in Car scope")
	}
	if car.Scope.LookupLocalAll("mass")[0].Kind != SymbolAttributeUsage {
		t.Fatalf("mass kind wrong")
	}
}

func TestBuildAttributeDefKind(t *testing.T) {
	root := parser.New(source.New("<t>", []byte("attribute def Mass;"))).ParseFile()
	scope := Build(root)
	syms := scope.LookupLocalAll("Mass")
	if len(syms) != 1 || syms[0].Kind != SymbolAttributeDef {
		t.Fatalf("Mass symbol wrong: %+v", syms)
	}
}

func TestBuildAnonymousUsageNotNamed(t *testing.T) {
	root := parser.New(source.New("<t>", []byte("part def Car { part; }"))).ParseFile()
	scope := Build(root)
	car := scope.LookupLocalAll("Car")[0]
	if len(car.Scope.LookupLocalAll("")) != 0 {
		t.Fatalf("anonymous usage should not be registered under empty name")
	}
	if len(car.Scope.Children()) != 1 {
		t.Fatalf("expected 1 child scope for the anonymous usage, got %d", len(car.Scope.Children()))
	}
}

// A feature declared without a name takes the name of the feature its single
// owned redefinition names (KerML 7.3.4.5): `part :>> engine;` declares
// `engine`, and a qualified target contributes its last segment.
func TestUnnamedRedefinitionTakesRedefinedName(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; attribute mass; }
		part v : Vehicle { part :>> engine; attribute :>> Vehicle::mass; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	for _, name := range []string{"engine", "mass"} {
		sym, ok := v.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("v members = %v, want %s", v.Scope.MemberNames(), name)
		}
		if !sym.EffectiveName() {
			t.Fatalf("%s should be marked as an effective name, not a declared one", name)
		}
	}
}

// A declared name is the feature's name whatever it redefines.
func TestRedefinitionDoesNotOverrideDeclaredName(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; }
		part v : Vehicle { part motor :>> engine; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	if _, ok := v.Scope.LookupLocal("motor"); !ok {
		t.Fatalf("v members = %v, want motor", v.Scope.MemberNames())
	}
	if _, ok := v.Scope.LookupLocal("engine"); ok {
		t.Fatalf("v should not bind the redefined name when motor is declared")
	}
}

// A reference subsetting is the naming feature when a declaration has both
// (KerML 7.3.4.5 prefers the referenced feature over the redefined one).
func TestReferenceSubsettingOutranksRedefinitionAsNamingFeature(t *testing.T) {
	root := build(t, `package P {
		action takePicture;
		action def Camera { action shoot; }
		part camera : Camera { perform action :>> shoot references takePicture; }
	}`)

	pkg, _ := root.LookupLocal("P")
	camera, _ := pkg.Scope.LookupLocal("camera")
	if _, ok := camera.Scope.LookupLocal("takePicture"); !ok {
		t.Fatalf("camera members = %v, want takePicture", camera.Scope.MemberNames())
	}
	if _, ok := camera.Scope.LookupLocal("shoot"); ok {
		t.Fatalf("camera should be named by the referenced feature, not the redefined one")
	}
}

// A declared short name is a declared name too (KerML 7.3.4.5): the feature
// then takes no name from its redefinition and is a member by its short name
// alone, as the pinned pilot reads `part <e> :>> engine;`.
func TestShortNameSuppressesTheDerivedName(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; }
		part v : Vehicle { part <e> :>> engine; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	e, ok := v.Scope.LookupLocal("e")
	if !ok {
		t.Fatalf("v members = %v, want e", v.Scope.MemberNames())
	}
	if _, ok := v.Scope.LookupLocal("engine"); ok {
		t.Fatalf("v members = %v; a short-named redefinition must not answer to engine", v.Scope.MemberNames())
	}
	if e.Name != "e" || e.Naming != NamedByDeclaration || e.NamingTarget != nil {
		t.Errorf("e = %q naming=%v target=%v; want a declared short name and no derived one", e.Name, e.Naming, e.NamingTarget)
	}
}

// The first redefinition names a feature that redefines several (KerML
// 7.3.4.5, Feature::namingFeature): `v.engine` reaches it, `v.motor` nothing.
func TestTwoRedefinitionsNameTheFeatureByTheFirst(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; part motor; }
		part v : Vehicle { part :>> engine :>> motor; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	if names := v.Scope.MemberNames(); len(names) != 1 || names[0] != "engine" {
		t.Fatalf("v members = %v, want [engine]", names)
	}
	e, _ := v.Scope.LookupLocal("engine")
	if e.Naming != NamedByRedefinition || e.NamingTarget == nil {
		t.Errorf("engine naming=%v target=%v; want the first redefinition", e.Naming, e.NamingTarget)
	}
}

// A feature chain is a nameless feature of its own, so a member whose first
// redefinition is a chain takes no name from it — nor from a later plain one —
// while a performed chain is named by its target (pinned pilot on both shapes).
func TestChainRedefinitionNamesNothing(t *testing.T) {
	root := build(t, `package P {
		part def H { part p { part q; action a; } part q2; }
		part h : H { part :>> p.q :>> q2; perform p.a; }
	}`)

	pkg, _ := root.LookupLocal("P")
	h, _ := pkg.Scope.LookupLocal("h")
	if names := h.Scope.MemberNames(); len(names) != 1 || names[0] != "a" {
		t.Fatalf("h members = %v, want [a]", names)
	}
	if anon := h.Scope.AnonymousMembers(); len(anon) != 1 || anon[0].Naming != NamedByDeclaration {
		t.Fatalf("anonymous members of h = %v, want the one chain redefinition with no derived name", anon)
	}
	a, _ := h.Scope.LookupLocal("a")
	if a.Naming != NamedByReference {
		t.Errorf("a naming=%v, want the performed chain's target", a.Naming)
	}
}

// The reference that named a feature is recorded, so resolving it can skip the
// name it gave away.
func TestNamingFeatureIsRecordedOnTheSymbol(t *testing.T) {
	root := build(t, `package P {
		part def Vehicle { part engine; }
		part v : Vehicle { part :>> engine; }
	}`)

	pkg, _ := root.LookupLocal("P")
	v, _ := pkg.Scope.LookupLocal("v")
	engine, _ := v.Scope.LookupLocal("engine")
	usage := engine.Decl.(*ast.Usage)
	if engine.NamingTarget != ast.AsQualifiedName(usage.Relationships[0].Target) {
		t.Fatalf("NamingTarget = %v, want the redefinition's target", engine.NamingTarget)
	}
}

// A named assume or require member owns a constraint usage, so it is a
// constraint-usage symbol the enclosing requirement finds by name, spanning the
// name and holding the body it declares; an anonymous one is an anonymous member.
func TestRequirementConstraintMembersAreSymbols(t *testing.T) {
	src := `package P {
		constraint def C;
		requirement def R {
			assume constraint a : C[0..1] = true { attribute x; }
			require constraint r : C;
			require constraint { true }
		}
	}`
	root := build(t, src)
	pkg, _ := root.LookupLocal("P")
	r, _ := pkg.Scope.LookupLocal("R")

	a, ok := r.Scope.LookupLocal("a")
	if !ok || a.Kind != SymbolConstraintUsage {
		t.Fatalf("a = %v (found %v), want a constraint usage", a, ok)
	}
	if _, isAssume := a.Decl.(*ast.AssumeMember); !isAssume {
		t.Fatalf("a declared by %T, want *ast.AssumeMember", a.Decl)
	}
	if got := src[a.NameSpan.Offset:a.NameSpan.End()]; got != "a" {
		t.Fatalf("a name span covers %q, want the name", got)
	}
	if _, ok := a.Scope.LookupLocal("x"); !ok {
		t.Fatalf("a members = %v, want its body member x", a.Scope.MemberNames())
	}
	if a.Scope.BodyLocal() {
		t.Fatalf("a named constraint usage's scope is not body-local")
	}

	req, ok := r.Scope.LookupLocal("r")
	if !ok || req.Kind != SymbolConstraintUsage {
		t.Fatalf("r = %v (found %v), want a constraint usage", req, ok)
	}
	if _, isRequire := req.Decl.(*ast.RequireMember); !isRequire {
		t.Fatalf("r declared by %T, want *ast.RequireMember", req.Decl)
	}

	if _, ok := r.Scope.LookupLocal(""); ok {
		t.Fatalf("the anonymous require constraint must not be registered under the empty name")
	}
	if anon := r.Scope.AnonymousMembers(); len(anon) != 1 || anon[0].Kind != SymbolConstraintUsage {
		t.Fatalf("anonymous members of R = %v, want the one anonymous require constraint", anon)
	}
}

// A prefixed anonymous `assume`/`require constraint` is an anonymous constraint
// usage symbol, as `constraint { … }` is, so metadata written on it is analysed.
// A reference member (`require Other;`) declares a usage named by its reference.
func TestAnonymousRequirementConstraintsAreAnonymousSymbols(t *testing.T) {
	src := `package P {
		metadata def M;
		constraint def C;
		requirement def Other;
		requirement def R {
			require #M constraint { true }
			assume constraint : C;
			require Other;
			assume Other;
		}
	}`
	root := build(t, src)
	pkg, _ := root.LookupLocal("P")
	r, _ := pkg.Scope.LookupLocal("R")

	anon := r.Scope.AnonymousMembers()
	if len(anon) != 2 {
		t.Fatalf("anonymous members of R = %d, want the two constraint declarations", len(anon))
	}
	require, assume := anon[0], anon[1]
	if _, ok := require.Decl.(*ast.RequireMember); !ok || require.Kind != SymbolConstraintUsage {
		t.Fatalf("first anonymous member = %v declared by %T, want a constraint usage of the require", require.Kind, require.Decl)
	}
	if _, ok := assume.Decl.(*ast.AssumeMember); !ok || assume.Kind != SymbolConstraintUsage {
		t.Fatalf("second anonymous member = %v declared by %T, want a constraint usage of the assume", assume.Kind, assume.Decl)
	}
	if require.Name != "" || require.Scope == nil || require.Scope.Owner() != require {
		t.Fatalf("anonymous require = %q owning %v, want no name and a scope of its own", require.Name, require.Scope)
	}
	if body := ConstraintBodyScope(r.Scope, require.Decl); body != require.Scope {
		t.Fatalf("anonymous require body = %v, want its own scope", body)
	}
	refs := r.Scope.LookupLocalAll("Other")
	if len(refs) != 2 {
		t.Fatalf("members of R named Other = %d, want the two reference forms", len(refs))
	}
	for _, sym := range refs {
		if sym.Kind != SymbolConstraintUsage || sym.Naming != NamedByReference || sym.NamingTarget == nil {
			t.Fatalf("reference form %v = %v named %v, want a constraint usage named by its reference", sym.Decl, sym.Kind, sym.Naming)
		}
	}
}

// A metadata usage with a body inside a subject, assume or require member's
// body is a body-local scope, as it is inside any usage's body.
func TestRequirementMemberMetadataBodiesGetScopes(t *testing.T) {
	root := build(t, `package P {
		metadata def M { attribute level; }
		part def Vehicle;
		constraint def C;
		requirement def R {
			subject s : Vehicle { @M { level = 1; } }
			assume constraint a : C { @M { level = 2; } }
			require constraint r : C { @M { level = 3; } }
		}
	}`)
	pkg, _ := root.LookupLocal("P")
	r, _ := pkg.Scope.LookupLocal("R")
	for _, name := range []string{"s", "a", "r"} {
		member, ok := r.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("R members = %v, want %s", r.Scope.MemberNames(), name)
		}
		var bodies int
		for _, child := range member.Scope.Children() {
			if _, ok := child.Node().(*ast.PrefixMetadata); ok && child.BodyLocal() {
				bodies++
			}
		}
		if bodies != 1 {
			t.Errorf("metadata body scopes under %s = %d, want 1", name, bodies)
		}
	}
}
