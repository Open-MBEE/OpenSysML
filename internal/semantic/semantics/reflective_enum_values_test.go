package semantics

import (
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
