package resolve

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// eachScope visits scope and every scope nested in it.
func eachScope(scope *symbols.Scope, visit func(*symbols.Scope)) {
	visit(scope)
	for _, child := range scope.Children() {
		eachScope(child, visit)
	}
}

// ownsName answers what membership in ownedNames does, for every scope and every key a
// member is registered under: short names, aliases, duplicates and members that bind none.
func TestOwnsNameAgreesWithOwnedNames(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": `package Lib { part def Engine; part def <W> Wheel; }`,
		"app.sysml": `package App {
			public import Lib::*;
			part def <B> Base {
				part <e> engine : Engine;
				part wheel : Wheel;
				action run;
				perform run;
				attribute twice;
				attribute twice;
			}
			part def Sub :> Base {
				protected import Lib::*;
				part :>> engine;
				part :>> missing;
				part <w2> :>> wheel;
				alias Motor for Engine;
				alias <M> for Engine;
				part;
			}
			part def Empty;
		}`,
	})
	r := resolverWithSpecializations(idx)
	scopes, owned := 0, 0
	eachScope(idx.DocumentRoot("app.sysml"), func(scope *symbols.Scope) {
		scopes++
		names := r.ownedNames(scope)
		owned += len(names)
		asked := append([]string{"", "absent", "missing", "Engine", "W"}, names...)
		scope.ForEachNamedMember(func(key string, sym *symbols.Symbol) bool {
			asked = append(asked, key, sym.Name, sym.ShortName)
			return true
		})
		for _, name := range asked {
			if got, want := r.ownsName(scope, name), slices.Contains(names, name); got != want {
				t.Errorf("ownsName(%s, %q) = %v, ownedNames has it: %v (%q)", symbols.FQNOf(scope.Owner()), name, got, want, names)
			}
		}
	})
	if scopes < 4 || owned < 12 {
		t.Fatalf("compared %d owned names over %d scopes: the model no longer exercises the rule", owned, scopes)
	}
	sub := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "Sub")
	for _, name := range []string{"engine", "e", "w2", "Motor", "M"} {
		if !r.ownsName(sub, name) {
			t.Errorf("Sub does not own %q; it owns %q", name, r.ownedNames(sub))
		}
	}
	// A redefinition declaring a short name takes no name from what it redefines (KerML 8.3.3.3).
	for _, name := range []string{"missing", "wheel", ""} {
		if r.ownsName(sub, name) {
			t.Errorf("Sub owns %q, which no member binds; it owns %q", name, r.ownedNames(sub))
		}
	}
}

// Asking whether a namespace hides an imported name lists none of its members: a lookup
// through a specialization asks it of every general type that imports.
func TestHidesImportedNameListsNoOwnedNames(t *testing.T) {
	var src strings.Builder
	src.WriteString("package App {\n\tpart def Base {\n\t\tprotected import Lib::*;\n")
	for i := range 200 {
		fmt.Fprintf(&src, "\t\tattribute <s%d> a%d;\n", i, i)
	}
	src.WriteString("\t}\n}\n")
	idx := indexOf(t, map[string]string{"lib.sysml": protectedLib, "app.sysml": src.String()})
	base := scopeOf(t, scopeOf(t, idx.DocumentRoot("app.sysml"), "App"), "Base")
	r := resolverWithSpecializations(idx)
	for _, name := range []string{"a199", "s199"} {
		if !r.HidesImportedName(base, name) {
			t.Fatalf("Base does not hide %q", name)
		}
		if allocs := testing.AllocsPerRun(20, func() { r.HidesImportedName(base, name) }); allocs != 0 {
			t.Errorf("HidesImportedName(Base, %q) allocates %v times over 200 members, want 0", name, allocs)
		}
	}
	if r.HidesImportedName(base, "Pub") {
		t.Fatal("Base hides Pub, which only its import brings")
	}
	if allocs := testing.AllocsPerRun(20, func() { r.ownsName(base, "Pub") }); allocs != 0 {
		t.Errorf("ownsName(Base, Pub) allocates %v times over 200 members, want 0", allocs)
	}
}
