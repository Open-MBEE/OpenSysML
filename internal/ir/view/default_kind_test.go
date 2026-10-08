package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const defaultKindModel = `package Demo {
  part def Wheel;
  part def Car {
    part fl : Wheel;
    part fr : Wheel;
    connect fl to fr;
  }
  part def Bare { part w : Wheel; }
  part car : Car {
    part a : Wheel;
    part b : Wheel;
    connect a to b;
  }
  part lone : Wheel;
  item def Cargo;
  state def Lamp { state off; state on; transition off then on; }
  state lamp : Lamp;
  action def Start { action a; action b; first a then b; }
  action start : Start;
  calc def Sum { in x : Real; return x; }
  use case def Drive;
  use case drive : Drive;
  analysis case def Study;
}
`

func defaultKindFixture(t *testing.T) (*Renderer, *symbols.Index) {
	t.Helper()
	return loadSources(t, []string{"default-kind.sysml"}, [][]byte{[]byte(defaultKindModel)})
}

func TestElementKindFollowsWhatTheElementIs(t *testing.T) {
	r, idx := defaultKindFixture(t)
	cases := map[string]Kind{
		"Demo":        KindTree,
		"Demo::Wheel": KindTree,
		"Demo::Car":   KindTree,
		"Demo::Bare":  KindTree,
		"Demo::Cargo": KindTree,
		"Demo::Sum":   KindTree,
		"Demo::lone":  KindTree,
		"Demo::car":   KindInterconnection,
		"Demo::Lamp":  KindState,
		"Demo::lamp":  KindState,
		"Demo::Start": KindAction,
		"Demo::start": KindAction,
		"Demo::Drive": KindCase,
		"Demo::drive": KindCase,
		"Demo::Study": KindCase,
	}
	for fqn, want := range cases {
		if got := r.ElementKind(lookup(t, idx, fqn)); got != want {
			t.Errorf("ElementKind(%s) = %q, want %q", fqn, got, want)
		}
	}
	if got := r.ElementKind(nil); got != KindTree {
		t.Errorf("ElementKind(nil) = %q, want %q", got, KindTree)
	}
}

func TestDefaultKindAgreesOrIsMixed(t *testing.T) {
	r, idx := defaultKindFixture(t)
	syms := func(fqns ...string) []*symbols.Symbol {
		out := make([]*symbols.Symbol, 0, len(fqns))
		for _, fqn := range fqns {
			out = append(out, lookup(t, idx, fqn))
		}
		return out
	}
	cases := []struct {
		names []string
		want  Kind
	}{
		{nil, KindTree},
		{[]string{"Demo::Lamp"}, KindState},
		{[]string{"Demo::Lamp", "Demo::lamp"}, KindState},
		{[]string{"Demo::Start", "Demo::start"}, KindAction},
		{[]string{"Demo::Wheel", "Demo::Car"}, KindTree},
		{[]string{"Demo::car"}, KindInterconnection},
		{[]string{"Demo::Lamp", "Demo::Start"}, KindMixed},
		{[]string{"Demo::Car", "Demo::car"}, KindMixed},
	}
	for _, c := range cases {
		if got := r.DefaultKind(syms(c.names...)); got != c.want {
			t.Errorf("DefaultKind(%v) = %q, want %q", c.names, got, c.want)
		}
	}
}

// Every kind DefaultKind chooses renders through RenderExposed, so the choice
// never names a kind the renderer refuses.
func TestDefaultKindsAreRenderable(t *testing.T) {
	r, idx := defaultKindFixture(t)
	for _, fqn := range []string{"Demo", "Demo::car", "Demo::Lamp", "Demo::Start", "Demo::drive"} {
		sym := lookup(t, idx, fqn)
		kind := r.DefaultKind([]*symbols.Symbol{sym})
		if _, ok := PseudoViewKind(string(kind)); !ok {
			t.Errorf("%s: DefaultKind %q is no pseudo-view kind", fqn, kind)
		}
		if _, err := r.RenderExposed([]*symbols.Symbol{sym}, kind, ""); err != nil {
			t.Errorf("%s: RenderExposed(%q): %v", fqn, kind, err)
		}
	}
}

func TestPilotStylesParseInAnyCaseAndNoteWhatTheyDo(t *testing.T) {
	for _, name := range []string{"ortholine", "ORTHOLINE", "OrthoLine"} {
		style, ok := ParsePilotStyle(name)
		if !ok || style != "ORTHOLINE" {
			t.Errorf("ParsePilotStyle(%q) = %q, %t", name, style, ok)
		}
	}
	if _, ok := ParsePilotStyle("nosuch"); ok {
		t.Error("an unknown name parsed as a pilot style")
	}
	styles := PilotStyles()
	if !slices.IsSorted(styles) || len(styles) != len(pilotStyles) {
		t.Errorf("PilotStyles() = %v, want every style sorted", styles)
	}
	for _, style := range styles {
		notice := style.Notice()
		if !strings.Contains(notice, string(style)) || !strings.Contains(notice, "not drawn") || strings.Contains(notice, "()") {
			t.Errorf("%s.Notice() = %q", style, notice)
		}
	}
	if got := PilotStyle("ORTHOLINE").Notice(); got != "style ORTHOLINE (orthogonal line style) is not drawn" {
		t.Errorf("Notice = %q", got)
	}
}
