package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A `for` loop's collection is evaluated before its body is entered, so it names what the
// enclosing scope holds and never a declaration of the body, whichever reader asks first.
func TestLoopCollectionResolvesOutsideTheLoopBody(t *testing.T) {
	m, root := buildModelWithStdlibUnwalked(t, `package Q {
	part def T {
		attribute xs[*];
		action a {
			action named for k in xs { attribute xs = 3; }
			action inner for k in ys { attribute ys = 3; }
		}
	}
}`)
	outer := nestedSym(t, root, "Q::T::xs")
	referents := func(path string) []*symbols.Symbol {
		loop := nestedSym(t, root, path)
		var out []*symbols.Symbol
		for _, parameter := range m.nodeParameters(loop) {
			if m.nodeParameterName(loop, parameter) != "seq" {
				continue
			}
			targets, _ := m.ReflectiveElements(m.ValueExpression(parameter), "referent")
			out = append(out, targets...)
		}
		return out
	}
	if got := referents("Q::T::a::named"); len(got) != 1 || got[0] != outer {
		t.Errorf("named's collection names %v, want the xs of T", got)
	}
	if got := referents("Q::T::a::inner"); len(got) != 0 {
		t.Errorf("inner's collection names %v, want nothing: ys is declared in the body only", got)
	}
}
