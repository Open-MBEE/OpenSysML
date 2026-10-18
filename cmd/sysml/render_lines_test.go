package main

import (
	"strings"
	"testing"
)

// -render-lines chooses how the dot or plantuml form routes its edges: splines,
// the default, leaves the engine's curved routing unstated; ortho writes
// `splines=ortho` and `skinparam linetype ortho`, polyline likewise; mermaid
// notes the routing; a routing there is none of is refused with the three there
// are, and the flag without something to render likewise.
func TestRenderLines(t *testing.T) {
	binary := buildCLI(t)

	plain := runStreams(t, binary, portedModel, "-render", "Demo::link", "-render-form", "dot")
	if plain.status != exitHolds || strings.Contains(plain.stdout, "splines=") {
		t.Errorf("the default states a routing (exit %d):\n%s", plain.status, plain.output())
	}
	stated := runStreams(t, binary, portedModel, "-render", "Demo::link", "-render-form", "dot", "-render-lines", "splines")
	if stated.status != exitHolds || stated.stdout != plain.stdout {
		t.Errorf("-render-lines splines differs from the default (exit %d):\n%s", stated.status, stated.output())
	}
	for _, c := range []struct{ lines, form, want string }{
		{"ortho", "dot", "splines=ortho"},
		{"polyline", "dot", "splines=polyline"},
		{"ortho", "plantuml", "skinparam linetype ortho\n"},
		{"polyline", "plantuml", "skinparam linetype polyline\n"},
		{"ortho", "mermaid", "%% not represented: lines ortho; mermaid routes no edge itself, the dot and plantuml forms do\n"},
	} {
		got := runStreams(t, binary, portedModel, "-render", "Demo::link", "-render-form", c.form, "-render-lines", c.lines)
		if got.status != exitHolds {
			t.Fatalf("-render-lines %s on %s: exit status = %d, want %d\n%s", c.lines, c.form, got.status, exitHolds, got.output())
		}
		if !strings.Contains(got.stdout, c.want) {
			t.Errorf("-render-lines %s on %s is missing %q:\n%s", c.lines, c.form, c.want, got.stdout)
		}
	}

	unknown := runStreams(t, binary, portedModel, "-render", "Demo::link", "-render-form", "dot", "-render-lines", "curved")
	if unknown.status != exitUnevaluable || !strings.Contains(unknown.stderr, `-render-lines: unknown line routing "curved"; the routings are splines, ortho, polyline`) || unknown.stdout != "" {
		t.Errorf("an unknown routing = %d\n%s", unknown.status, unknown.output())
	}

	alone := runStreams(t, binary, portedModel, "-render-lines", "ortho")
	if alone.status != 2 || !strings.Contains(alone.stderr, "-render-lines is how the dot or plantuml form of -render or -render-all routes its edges") {
		t.Errorf("-render-lines alone = %d\n%s", alone.status, alone.output())
	}
}
