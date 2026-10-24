package main

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

// -render-depth bounds how many levels are drawn below each exposed element,
// and overrides the depth the view states for that render only.
func TestRenderDepthBoundsTheWalk(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, depthModel, "-render", "Deep::structure", "-render-form", "text", "-render-depth", "1")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"part wheel (nested deeper than 1 level; not shown)", "part engine (nested deeper than 1 level; not shown)"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	for _, hidden := range []string{"hub", "piston"} {
		if strings.Contains(got.stdout, hidden) {
			t.Errorf("stdout draws %s below the bound:\n%s", hidden, got.stdout)
		}
	}
	if !strings.Contains(got.stderr, "2 elements nested deeper than 1 level not shown") {
		t.Errorf("stderr is missing the depth notice:\n%s", got.stderr)
	}

	got = runStreams(t, binary, depthModel, "-render", "Deep::shallow", "-render-form", "text", "-render-depth", "3")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stdout, "part bearing") || strings.Contains(got.stdout, "not shown") {
		t.Errorf("-render-depth 3 does not override the view's depth 2:\n%s", got.stdout)
	}
}

// A negative depth, or one with nothing to render, is refused up front.
func TestRenderDepthIsCheckedUpFront(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, depthModel, "-render", "Deep::structure", "-render-depth", "-1")
	if got.status != 2 || !strings.Contains(got.stderr, `-render-depth: depth "-1" is no non-negative integer`) {
		t.Errorf("status %d, stderr:\n%s", got.status, got.stderr)
	}
	got = runStreams(t, binary, depthModel, "-render-depth", "2")
	if got.status != 2 || !strings.Contains(got.stderr, "-render-depth is how many levels") {
		t.Errorf("status %d, stderr:\n%s", got.status, got.stderr)
	}
}
