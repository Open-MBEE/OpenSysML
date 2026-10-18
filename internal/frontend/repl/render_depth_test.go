package repl

import (
	"strings"
	"testing"
)

const depthModel = `package Deep {
    private import Views::*;
    part vehicle {
        part wheel { part hub { part bearing; } }
        part engine { part piston; }
    }
    view structure { expose vehicle; render asTreeDiagram; }
    view shallow { expose vehicle; attribute depth = 2; render asTreeDiagram; }
}
`

// %render's depth= bounds the walk for that render and overrides the view's depth.
func TestRenderDepthOption(t *testing.T) {
	s := NewSession()
	if res := s.Submit(depthModel); len(res.Diagnostics) != 0 {
		t.Fatalf("model drew %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%render Deep::structure text depth=1"),
		"part wheel (nested deeper than 1 level; not shown)",
		"2 elements nested deeper than 1 level not shown")
	if out := run(t, s, "%render Deep::structure text depth=1"); strings.Contains(out, "hub") {
		t.Errorf("depth=1 draws hub:\n%s", out)
	}
	wants(t, run(t, s, "%render Deep::shallow text"), "part hub (nested deeper than 2 levels; not shown)")
	wants(t, run(t, s, "%render Deep::shallow text depth=3"), "part bearing")
	wants(t, run(t, s, "%render Deep::structure text depth=x"), `depth "x" is no non-negative integer`, "usage: %render")
	wants(t, run(t, s, "%render Deep::structure text depth=1 depth=2"), "usage: %render")
}

// depth= is an option word: it reads right after the view name, and twice is a usage error whatever the first value.
func TestRenderDepthOptionWord(t *testing.T) {
	s := NewSession()
	s.Submit(depthModel)
	wants(t, run(t, s, "%render Deep::structure depth=1"), "part wheel (nested deeper than 1 level; not shown)")
	wants(t, run(t, s, "%render Deep::structure depth=0 depth=2"), "usage: %render")
}
