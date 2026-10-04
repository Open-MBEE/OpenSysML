package model

import (
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// An end usage that may vary in time is implicitly constant (SysML 2.0
// §8.4.2.2), and the checker, reflection and FeatureReadOnly all read the one
// derivation: the same fixture the runtime conformance suite refuses an
// assignment into reports assignment-referent-read-only here.
func TestImplicitEndConstancyAgreesAcrossSurfaces(t *testing.T) {
	src, err := os.ReadFile("../../exec/runtime/testdata/conformance/assign_end_feature_refused.sysml")
	if err != nil {
		t.Fatal(err)
	}
	ws := NewWorkspace()
	ws.OpenAll([]Input{{Name: "assign_end_feature_refused.sysml", Content: src, Version: 1}})
	var found bool
	for _, d := range ws.Diagnostics("assign_end_feature_refused.sysml") {
		if d.Code == "assignment-referent-read-only" {
			found = true
		}
	}
	if !found {
		t.Fatal("assigning an implicitly constant end reported no assignment-referent-read-only")
	}
	matches := ws.index.LookupQualified("test::tune::level")
	if len(matches) != 1 {
		t.Fatalf("test::tune::level resolves to %d symbols, want 1", len(matches))
	}
	level := matches[0]
	if v, ok := ws.model.ReflectiveFeatureValue(level, "isConstant"); !ok || !v.Bool {
		t.Errorf("level reflective isConstant = %v (present %t), want true", v, ok)
	}
	ro := ws.model.FeatureReadOnly(level)
	if ro.Kind != semantics.ReadOnlyConstant || !ro.ImpliedByEnd || ro.Declared != level {
		t.Errorf("level read-only = %+v, want constant, implied by its own end", ro)
	}
}

// A recorded end carries its derived mayTimeVary as a modifier bit, so a
// symbol loaded from an interface record answers the same isConstant,
// isVariable and read-only verdict as the parsed one.
func TestRecordedEndMatchesParsedEnd(t *testing.T) {
	src := []byte(`package Ends {
	part def Thing;
	part def Holder {
		end part e : Thing;
		part notEnd : Thing;
		constant part fixed : Thing;
	}
	connection def Link { end part a : Thing; }
	attribute def A { end part x; }
}`)
	loaded := NewWorkspace()
	loaded.OpenAll([]Input{{Name: "ends.sysml", Content: src, Version: 1}})
	loaded.DiagnosticsAll([]string{"ends.sysml"})
	rec, err := loaded.InterfaceRecord("ends.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if e := findFact(t, rec, "Ends::Holder::e"); !e.Facts.Modifiers.Has(symbols.ModMayTimeVary) {
		t.Fatal("the record of Holder::e carries no ModMayTimeVary")
	}
	if x := findFact(t, rec, "Ends::A::x"); x.Facts.Modifiers.Has(symbols.ModMayTimeVary) {
		t.Fatal("the record of A::x carries ModMayTimeVary, want none")
	}

	ws := NewWorkspace()
	if err := ws.OpenRecorded(rec, src); err != nil {
		t.Fatal(err)
	}
	parsed := func(fqn string) *symbols.Symbol {
		matches := loaded.index.LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("parsed %s: %d matching symbols, want 1", fqn, len(matches))
		}
		return matches[0]
	}
	recorded := func(fqn string) *symbols.Symbol {
		matches := ws.index.LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("recorded %s: %d matching symbols, want 1", fqn, len(matches))
		}
		return matches[0]
	}
	reflective := func(m *semantics.Model, sym *symbols.Symbol, feature string) (bool, bool) {
		v, ok := m.ReflectiveFeatureValue(sym, feature)
		return v.Bool, ok
	}
	fqn := func(sym *symbols.Symbol) string {
		if sym == nil {
			return ""
		}
		return symbols.FQNOf(sym)
	}

	tests := []struct {
		path                   string
		isConstant, isVariable bool
		implied                bool
	}{
		{"Ends::Holder::e", true, true, true},
		{"Ends::Holder::notEnd", false, true, false},
		{"Ends::Holder::fixed", true, true, false},
		{"Ends::Link::a", true, true, true},
		{"Ends::A::x", false, false, false},
	}
	for _, tc := range tests {
		p, r := parsed(tc.path), recorded(tc.path)
		if !r.Recorded() {
			t.Fatalf("%s did not restore as a recorded symbol", tc.path)
		}
		pConst, pConstOK := reflective(loaded.model, p, "isConstant")
		pVar, pVarOK := reflective(loaded.model, p, "isVariable")
		rConst, rConstOK := reflective(ws.model, r, "isConstant")
		rVar, rVarOK := reflective(ws.model, r, "isVariable")
		if !pConstOK || !pVarOK || !rConstOK || !rVarOK {
			t.Fatalf("%s: an isConstant or isVariable flag is underived (parsed %t/%t, recorded %t/%t)",
				tc.path, pConstOK, pVarOK, rConstOK, rVarOK)
		}
		if pConst != tc.isConstant || pVar != tc.isVariable {
			t.Errorf("parsed %s: isConstant/isVariable = %t/%t, want %t/%t", tc.path, pConst, pVar, tc.isConstant, tc.isVariable)
		}
		if rConst != pConst || rVar != pVar {
			t.Errorf("recorded %s: isConstant/isVariable = %t/%t, want the parsed %t/%t", tc.path, rConst, rVar, pConst, pVar)
		}
		pro := loaded.model.FeatureReadOnly(p)
		rro := ws.model.FeatureReadOnly(r)
		if rro.Kind != pro.Kind || fqn(rro.Declared) != fqn(pro.Declared) || rro.ImpliedByEnd != pro.ImpliedByEnd {
			t.Errorf("%s: recorded read-only %+v differs from parsed %+v", tc.path, rro, pro)
		}
		wantKind := semantics.Writable
		if tc.isConstant {
			wantKind = semantics.ReadOnlyConstant
		}
		if pro.Kind != wantKind || pro.ImpliedByEnd != tc.implied {
			t.Errorf("parsed %s: read-only %+v, want kind %v implied %t", tc.path, pro, wantKind, tc.implied)
		}
	}
}

// findFact returns the record's symbol fqn names, for a fact assertion.
func findFact(t *testing.T, rec *libs.InterfaceRecord, fqn string) *symbols.SymbolRecord {
	t.Helper()
	var out *symbols.SymbolRecord
	var walk func(scopeIdx int32, prefix string)
	walk = func(scopeIdx int32, prefix string) {
		for _, m := range rec.Scope.Scopes[scopeIdx].Members {
			sym := &rec.Scope.Symbols[m.Symbol]
			name := prefix + m.Name
			if name == fqn {
				out = sym
			}
			if sym.Scope >= 0 {
				walk(sym.Scope, name+"::")
			}
		}
	}
	walk(0, "")
	if out == nil {
		t.Fatalf("the record holds no %s", fqn)
	}
	return out
}
