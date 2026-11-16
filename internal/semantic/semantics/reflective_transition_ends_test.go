package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestTransitionReflectsItsEffectMetaclassAndEnds: an effect is the usage EffectBehaviorUsage writes it as,
// and source and target are the ActionUsages the transition's source feature and succession target.
func TestTransitionReflectsItsEffectMetaclassAndEnds(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    private import ScalarValues::*;
    attribute def Sig;
    action def A;
    part def Machine {
        attribute x : Integer = 1;
        state sm {
            entry; then idle;
            state idle { state inner; }
            state busy; state fin; state late;
            transition t1 first idle accept Sig if x > 0 do action e1 then busy;
            transition t2 first busy do action e2 : A { action step; } then fin;
            transition t3 first fin do send new Sig() to sm then late;
            transition t4 first late do assign x := 2 then idle;
            transition t5 first late do { action step; } then idle;
            transition t6 first late then idle;
            transition t7 first idle.inner then busy;
            transition t8 first busy then inner;
            transition t9 first fin do action effect : A then late;
            transition t10 first fin accept effect : Sig do action e10 then late;
        }
    }
}
`)
	sm := nested(t, nested(t, sym(t, root, "P").Scope, "Machine").Scope, "sm").Scope
	idle := nested(t, sm, "idle")
	cases := []struct {
		transition, effect string
		source, target     *symbols.Symbol
	}{
		{"t1", "PerformActionUsage", idle, nested(t, sm, "busy")},
		{"t2", "PerformActionUsage", nested(t, sm, "busy"), nested(t, sm, "fin")},
		{"t3", "SendActionUsage", nested(t, sm, "fin"), nested(t, sm, "late")},
		{"t4", "AssignmentActionUsage", nested(t, sm, "late"), idle},
		{"t5", "ActionUsage", nested(t, sm, "late"), idle},
		{"t6", "", nested(t, sm, "late"), idle},
		{"t7", "", nested(t, idle.Scope, "inner"), nested(t, sm, "busy")},
		// A target nested in a sibling state is named as the endpoint resolution reaches it.
		{"t8", "", nested(t, sm, "busy"), nested(t, idle.Scope, "inner")},
		// A member already named `effect`, the effect itself or another, leaves the effect's kind as declared.
		{"t9", "PerformActionUsage", nested(t, sm, "fin"), nested(t, sm, "late")},
		{"t10", "PerformActionUsage", nested(t, sm, "fin"), nested(t, sm, "late")},
	}
	// t5's braced body is no PerformActionUsageDeclaration: it stays the plain action it is read as.
	for _, tc := range cases {
		transition := nested(t, sm, tc.transition)
		effects, ok := m.ReflectiveElements(transition, "effectAction")
		if !ok {
			t.Fatalf("%s: effectAction is not derived", tc.transition)
		}
		var effect string
		if len(effects) == 1 {
			if meta := m.MetaclassOf(effects[0]); meta != nil {
				effect = meta.Name
			}
		}
		if effect != tc.effect {
			t.Errorf("%s: effectAction is a %q (%d elements), want %q", tc.transition, effect, len(effects), tc.effect)
		}
		for feature, want := range map[string]*symbols.Symbol{"source": tc.source, "target": tc.target} {
			got, ok := m.ReflectiveElements(transition, feature)
			if !ok || len(got) != 1 || got[0] != want {
				t.Errorf("%s: %s = %v (derived %t), want %s", tc.transition, feature, got, ok, want.Name)
			}
		}
	}
	// The body of a performed effect is the effect's own: its step stays a plain ActionUsage.
	e2 := nested(t, nested(t, sm, "t2").Scope, "e2")
	if meta := m.MetaclassOf(nested(t, e2.Scope, "step")); meta == nil || meta.Name != "ActionUsage" {
		t.Errorf("e2::step is a %v, want an ActionUsage", meta)
	}
}
