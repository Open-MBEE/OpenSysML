package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// localLookup answers the member lookups a metadata body's implicit target
// needs, searching only what the symbol itself declares.
type localLookup struct{}

func (localLookup) LookupMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym != nil && sym.Scope != nil {
		return sym.Scope.LookupLocal(name)
	}
	return nil, false
}

func (localLookup) LookupContributedMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	return nil, false
}

// resolveStdlib resolves src over the bundled standard library, the context an
// unimported library name is diagnosed in.
func resolveStdlib(t *testing.T, name, src string) *resolve.Resolver {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(localLookup{})
	r.ResolveDocument(name, root)
	return r
}

// memberSymbol returns the symbol scope declares node as, or fails.
func memberSymbol(t *testing.T, scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	t.Helper()
	if sym := scope.MemberDeclaring(node); sym != nil {
		return sym
	}
	t.Fatalf("no member symbol declares %T", node)
	return nil
}

// packageMember returns the member symbol name declares in package pkg.
func packageMember(t *testing.T, r *resolve.Resolver, doc, pkg, name string) *symbols.Symbol {
	t.Helper()
	pkgSym, ok := r.Index().DocumentRoot(doc).LookupLocal(pkg)
	if !ok {
		t.Fatalf("no package %s", pkg)
	}
	sym, ok := pkgSym.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("no member %s::%s", pkg, name)
	}
	return sym
}

// annotation returns the i-th metadata usage among def's members: the Usage a
// `metadata M {…}` member is, or the PrefixMetadata an `@M {…}` one is.
func annotation(t *testing.T, def *symbols.Symbol, i int) ast.Node {
	t.Helper()
	var members []ast.Node
	switch d := def.Decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		t.Fatalf("%s declares %T, not a definition or usage", def.Name, def.Decl)
	}
	var found []ast.Node
	for _, m := range members {
		if w, ok := m.(*ast.Membership); ok {
			m = w.Member
		}
		switch u := m.(type) {
		case *ast.Usage:
			if u.Kind == ast.UsageMetadata {
				found = append(found, u)
			}
		case *ast.PrefixMetadata:
			found = append(found, u)
		}
	}
	if len(found) <= i {
		t.Fatalf("metadata usage %d not among %d", i, len(found))
	}
	return found[i]
}

// typingName returns the name a metadata usage's type is written as.
func typingName(t *testing.T, n ast.Node) *ast.QualifiedName {
	t.Helper()
	switch u := n.(type) {
	case *ast.PrefixMetadata:
		return u.Type
	case *ast.Usage:
		for _, rel := range u.Relationships {
			if rel.Kind == ast.RelTyping {
				return ast.AsQualifiedName(rel.Target)
			}
		}
	}
	t.Fatalf("%T has no type name", n)
	return nil
}

// An unimported library name is unresolved; the diagnostic names the import
// that makes the bare name visible.
func TestUnimportedVerificationMethodNamesTheImport(t *testing.T) {
	for _, form := range []string{
		"metadata VerificationMethod { kind = VerificationMethodKind::test; }",
		"@VerificationMethod { kind = VerificationMethodKind::test; }",
	} {
		src := "package S8 { verification def T { " + form + " } }"
		r := resolveStdlib(t, "d.sysml", src)
		if len(r.Diagnostics) == 0 {
			t.Fatalf("no diagnostics for %q", form)
		}
		first := r.Diagnostics[0].Message
		want := "unresolved reference: VerificationMethod — did you mean VerificationCases::VerificationMethod? To use the bare name, import its package: private import VerificationCases::*;"
		if first != want {
			t.Fatalf("message = %q, want %q", first, want)
		}
		var imported bool
		for _, fix := range r.Diagnostics[0].Fixes {
			if fix.Title != "Import 'VerificationCases::*'" {
				continue
			}
			imported = true
			if len(fix.Edits) != 1 || fix.Edits[0].NewText != "private import VerificationCases::*;" {
				t.Errorf("import fix writes %+v", fix.Edits)
			}
		}
		if !imported {
			t.Errorf("no Import 'VerificationCases::*' fix in %+v", r.Diagnostics[0].Fixes)
		}
	}
}

// A qualified name whose first segment is an unimported library namespace is
// diagnosed by what the segment may mean, with fixes writing it qualified or
// importing its package.
func TestUnimportedVerificationMethodKindNamesTheImport(t *testing.T) {
	src := `package S8 {
		verification def T {
			metadata VerificationCases::VerificationMethod { kind = VerificationMethodKind::test; }
		}
	}`
	r := resolveStdlib(t, "d.sysml", src)
	var d *resolve.Diagnostic
	for i := range r.Diagnostics {
		if strings.Contains(r.Diagnostics[i].Message, "VerificationMethodKind") {
			d = &r.Diagnostics[i]
		}
	}
	if d == nil {
		t.Fatalf("no diagnostic names VerificationMethodKind: %v", r.Diagnostics)
	}
	want := "unresolved reference: VerificationMethodKind::test — \"VerificationMethodKind\" is not visible here; did you mean VerificationCases::VerificationMethodKind::test? To use the bare name, import its package: private import VerificationCases::*;"
	if d.Message != want {
		t.Fatalf("message = %q, want %q", d.Message, want)
	}
	var change, imported bool
	for _, fix := range d.Fixes {
		switch fix.Title {
		case "Change 'VerificationMethodKind' to 'VerificationCases::VerificationMethodKind'":
			change = true
			if len(fix.Edits) != 1 || fix.Edits[0].NewText != "VerificationCases::VerificationMethodKind" {
				t.Errorf("change fix writes %+v", fix.Edits)
			}
			if got := src[fix.Edits[0].Span.Offset:fix.Edits[0].Span.End()]; got != "VerificationMethodKind" {
				t.Errorf("the edit replaces %q, want the first segment only", got)
			}
		case "Import 'VerificationCases::*'":
			imported = true
		}
	}
	if !change || !imported {
		t.Errorf("fixes = %+v, want the change and import fixes", d.Fixes)
	}
}

// The same names imported resolve, and the body's `kind` implicitly redefines
// the metadata definition's feature.
func TestImportedVerificationMethodResolves(t *testing.T) {
	src := `package S8 {
		private import VerificationCases::*;
		verification def T {
			metadata VerificationMethod { kind = VerificationMethodKind::test; }
		}
		verification def U {
			@VerificationMethod { kind = VerificationMethodKind::analyze; }
		}
	}`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", r.Diagnostics)
	}
	for _, def := range []string{"T", "U"} {
		defSym := packageMember(t, r, "d.sysml", "S8", def)
		meta := annotation(t, defSym, 0)
		typeSym, ok := r.PartSymbol(typingName(t, meta), 0)
		if !ok || r.Index().GetFQN(typeSym) != "VerificationCases::VerificationMethod" {
			t.Fatalf("the typing of %s resolved to %v", def, typeSym)
		}
		bodyScope := memberSymbol(t, defSym.Scope, meta).Scope
		kindSym, ok := bodyScope.LookupLocal("kind")
		if !ok {
			t.Fatalf("no `kind` in the metadata body of %s", def)
		}
		owner := r.MetadataBodyOwner(kindSym.OwnerScope)
		target := symbols.MetadataBodyTarget(localLookup{}, owner, kindSym.Decl.(*ast.Usage).Ident)
		if got := r.Index().GetFQN(target); got != "VerificationCases::VerificationMethod::kind" {
			t.Fatalf("the body `kind` of %s targets %q", def, got)
		}
	}
}

// Fully qualified names need no import.
func TestQualifiedVerificationMethodResolves(t *testing.T) {
	src := `package S8 {
		verification def T {
			metadata VerificationCases::VerificationMethod { kind = VerificationCases::VerificationMethodKind::test; }
		}
	}`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", r.Diagnostics)
	}
}

// A name two importable declarations answer to names neither: guessing one
// would be wrong as often as right.
func TestTwoImportableCandidatesNameNoImport(t *testing.T) {
	src := `package Lib1 { package N { part def Wheel; } }
		package Lib2 { package N { part def Wheel; } }
		package P { part w : Wheel; }`
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want one", r.Diagnostics)
	}
	if strings.Contains(r.Diagnostics[0].Message, "import its package") {
		t.Fatalf("message = %q, want no import sentence with two candidates", r.Diagnostics[0].Message)
	}
}

// An annotation's body resolves even when its type does not: the unimported
// `@VerificationMethod { kind = …; }` reports the body value as the `metadata`
// form does, qualified-first-segment message and both fixes.
func TestUnimportedPrefixBodyNamesTheValue(t *testing.T) {
	src := "package Demo { verification def T { @VerificationMethod { kind = VerificationMethodKind::test; } } }"
	r := resolveStdlib(t, "d.sysml", src)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %v, want the two unresolved references", r.Diagnostics)
	}
	wantFirst := "unresolved reference: VerificationMethod — did you mean VerificationCases::VerificationMethod? To use the bare name, import its package: private import VerificationCases::*;"
	if r.Diagnostics[0].Message != wantFirst {
		t.Fatalf("first diagnostic = %q, want %q", r.Diagnostics[0].Message, wantFirst)
	}
	wantValue := "unresolved reference: VerificationMethodKind::test — \"VerificationMethodKind\" is not visible here; did you mean VerificationCases::VerificationMethodKind::test? To use the bare name, import its package: private import VerificationCases::*;"
	if r.Diagnostics[1].Message != wantValue {
		t.Fatalf("second diagnostic = %q, want %q", r.Diagnostics[1].Message, wantValue)
	}
	var change, imported bool
	for _, fix := range r.Diagnostics[1].Fixes {
		switch fix.Title {
		case "Change 'VerificationMethodKind' to 'VerificationCases::VerificationMethodKind'":
			change = true
		case "Import 'VerificationCases::*'":
			imported = true
		}
	}
	if !change || !imported {
		t.Errorf("fixes = %+v, want the change and import fixes", r.Diagnostics[1].Fixes)
	}
}

// A candidate that names nothing under the written segments is not offered:
// `VerificationMethodKind::bogus` resolves to nothing as
// `VerificationCases::VerificationMethodKind::bogus`, so no qualification or
// import is suggested.
func TestUnimportedVerificationMethodKindNamesNoBogusImport(t *testing.T) {
	src := `package S8 {
		verification def T {
			metadata VerificationMethod { kind = VerificationMethodKind::bogus; }
		}
	}`
	r := resolveStdlib(t, "d.sysml", src)
	var d *resolve.Diagnostic
	for i := range r.Diagnostics {
		if strings.Contains(r.Diagnostics[i].Message, "VerificationMethodKind") {
			d = &r.Diagnostics[i]
		}
	}
	if d == nil {
		t.Fatalf("no diagnostic names VerificationMethodKind: %v", r.Diagnostics)
	}
	if strings.Contains(d.Message, "did you mean VerificationCases::VerificationMethodKind::bogus") ||
		strings.Contains(d.Message, "private import") {
		t.Fatalf("message = %q, want no qualification or import offered", d.Message)
	}
	for _, fix := range d.Fixes {
		if strings.HasPrefix(fix.Title, "Change ") || strings.HasPrefix(fix.Title, "Import ") {
			t.Errorf("fix %q should not be offered for a path that resolves to nothing", fix.Title)
		}
	}
}

// An alias as the first segment is dereferenced when probing the candidate
// path: `Lib::A` is importable, `A` aliases `Lib::Real`, and `A::Item`
// resolves through it.
func TestUnimportedAliasNamesTheImport(t *testing.T) {
	src := `package Lib {
		namespace Real {
			attribute Item;
		}
		alias A for Real;
	}
	package Use {
		part x : A::Item;
	}`
	r := resolveStdlib(t, "d.sysml", src)
	var d *resolve.Diagnostic
	for i := range r.Diagnostics {
		if strings.Contains(r.Diagnostics[i].Message, "A::Item") {
			d = &r.Diagnostics[i]
		}
	}
	if d == nil {
		t.Fatalf("no diagnostic names A::Item: %v", r.Diagnostics)
	}
	want := "unresolved reference: A::Item — \"A\" is not visible here; did you mean Lib::A::Item? To use the bare name, import its package: private import Lib::*;"
	if d.Message != want {
		t.Fatalf("message = %q, want %q", d.Message, want)
	}
	var change, imported bool
	for _, fix := range d.Fixes {
		switch fix.Title {
		case "Change 'A' to 'Lib::A'":
			change = true
		case "Import 'Lib::*'":
			imported = true
		}
	}
	if !change || !imported {
		t.Errorf("fixes = %+v, want the change and import fixes", d.Fixes)
	}
}

// An ambiguous tail segment names nothing a rewrite could resolve to, so no
// qualification or import is offered.
func TestUnimportedAmbiguousTailNamesNoImport(t *testing.T) {
	src := `package Lib {
		part def N :> B1, B2;
		part def B1 { part item; }
		part def B2 { part item; }
	}
	package Use {
		part x : N::item;
	}`
	r := resolveStdlib(t, "d.sysml", src)
	var d *resolve.Diagnostic
	for i := range r.Diagnostics {
		if strings.Contains(r.Diagnostics[i].Message, "N::item") {
			d = &r.Diagnostics[i]
		}
	}
	if d == nil {
		t.Fatalf("no diagnostic names N::item: %v", r.Diagnostics)
	}
	if strings.Contains(d.Message, "private import") || strings.Contains(d.Message, "Lib::N::item") {
		t.Fatalf("message = %q, want no qualification or import offered", d.Message)
	}
	for _, fix := range d.Fixes {
		if strings.HasPrefix(fix.Title, "Change ") || strings.HasPrefix(fix.Title, "Import ") {
			t.Errorf("fix %q should not be offered for an ambiguous path", fix.Title)
		}
	}
}
