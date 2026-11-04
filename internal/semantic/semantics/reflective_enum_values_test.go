package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestEnumerationValuedPropertiesReadTheirLiteral: direction and portionKind read the library's
// literal (FeatureDirectionKind, PortionKind), as visibility reads VisibilityKind's, not its name.
func TestEnumerationValuedPropertiesReadTheirLiteral(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    action def A { in x; out y; inout z; }
    part def D { timeslice t; snapshot s; }
}
`)
	p := sym(t, root, "P").Scope
	for path, want := range map[*symbols.Symbol]string{
		nested(t, nested(t, p, "A").Scope, "x"): "KerML::Core::FeatureDirectionKind::in",
		nested(t, nested(t, p, "A").Scope, "y"): "KerML::Core::FeatureDirectionKind::out",
		nested(t, nested(t, p, "A").Scope, "z"): "KerML::Core::FeatureDirectionKind::inout",
	} {
		got, ok := m.ReflectiveFeatureValue(path, "direction")
		if !ok || got.Kind != symbols.FilterValueRef || got.RefFQN != want {
			t.Errorf("%s.direction = %v (present %t), want the literal %s", path.Name, got, ok, want)
		}
	}
	for path, want := range map[*symbols.Symbol]string{
		nested(t, nested(t, p, "D").Scope, "t"): "SysML::Systems::PortionKind::timeslice",
		nested(t, nested(t, p, "D").Scope, "s"): "SysML::Systems::PortionKind::snapshot",
	} {
		got, ok := m.ReflectiveFeatureValue(path, "portionKind")
		if !ok || got.Kind != symbols.FilterValueRef || got.RefFQN != want {
			t.Errorf("%s.portionKind = %v (present %t), want the literal %s", path.Name, got, ok, want)
		}
	}
	if got, ok := m.ReflectiveFeatureValue(nested(t, nested(t, p, "D").Scope, "t"), "direction"); !ok || got.Kind != symbols.FilterValueEmpty {
		t.Errorf("an undirected feature's direction = %v (present %t), want present empty", got, ok)
	}
}

// TestReflectiveKerMLEnumerationLiterals: a member feature of a DataType of the reflective KerML
// model is a literal of that enumeration (KerML 1.1 §9.2.17), as an enum def's value is of its own.
func TestReflectiveKerMLEnumerationLiterals(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    datatype Mode { member feature on : Mode[1]; }
}
`)
	for kind, literals := range map[string][]string{
		"FeatureDirectionKind": {"in", "inout", "out"},
		"VisibilityKind":       {"private", "protected", "public"},
	} {
		enum := m.kermlMetaclass(kind)
		if !IsEnumeration(enum) || !IsReflectiveEnumeration(enum) {
			t.Fatalf("%s is not read as an enumeration", kind)
		}
		var got []string
		for _, literal := range m.EnumeratedValuesOf(enum) {
			if EnumerationOwning(literal) != enum {
				t.Errorf("%s::%s is not a literal of %s", kind, literal.Name, kind)
			}
			if IsVariant(literal) {
				t.Errorf("%s::%s reads as a SysML variant", kind, literal.Name)
			}
			got = append(got, literal.Name)
		}
		if !slices.Equal(got, literals) {
			t.Errorf("%s enumerates %v, want %v", kind, got, literals)
		}
	}
	mode := sym(t, root, "P").Scope
	user := nested(t, mode, "Mode")
	if IsEnumeration(user) || EnumerationOwning(nested(t, user.Scope, "on")) != nil {
		t.Error("a user DataType outside the reflective KerML model reads as an enumeration")
	}
	if !IsEnumeration(m.sysmlMetaclass("PortionKind")) {
		t.Error("SysML::Systems::PortionKind is not read as an enumeration")
	}
}
