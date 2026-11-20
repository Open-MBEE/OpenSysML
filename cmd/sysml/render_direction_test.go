package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const layeredModel = `package Platform {
    part platform {
        part data;
        part application;
        part presentation;
    }
    view plain { expose platform::**; render Views::asInterconnectionDiagram; }
    view tree { expose platform; render Views::asTreeDiagram; }
}
`

// -render-direction names the direction a graph-shaped form flows in, a view's default
// when unset; a name that is no direction is refused with the four there are.
func TestRenderDirection(t *testing.T) {
	binary := buildCLI(t)

	for _, c := range []struct {
		view string
		args []string
		want string
	}{
		{"Platform::plain", []string{"-render-form", "mermaid"}, "flowchart LR\n"},
		{"Platform::plain", []string{"-render-form", "mermaid", "-render-direction", "TB"}, "flowchart TB\n"},
		{"Platform::plain", []string{"-render-form", "mermaid", "-render-direction", "bt"}, "flowchart BT\n"},
		{"Platform::plain", []string{"-render-form", "dot", "-render-direction", "TB"}, "rankdir=TB"},
		{"Platform::plain", []string{"-render-form", "d2", "-render-direction", "RL"}, "direction: left\n"},
		{"Platform::plain", []string{"-render-form", "plantuml", "-render-direction", "LR"}, "left to right direction\n"},
		{"Platform::tree", []string{"-render-form", "mermaid"}, "flowchart TD\n"},
		{"Platform::tree", []string{"-render-form", "mermaid", "-render-direction", "LR"}, "flowchart LR\n"},
	} {
		got := runStreams(t, binary, layeredModel, append([]string{"-render", c.view}, c.args...)...)
		if got.status != exitHolds || !strings.Contains(got.stdout, c.want) {
			t.Errorf("-render %s %v (exit %d) is missing %q:\n%s", c.view, c.args, got.status, c.want, got.output())
		}
	}

	unknown := runStreams(t, binary, layeredModel, "-render", "Platform::plain", "-render-form", "mermaid", "-render-direction", "down")
	if unknown.status != exitUnevaluable || !strings.Contains(unknown.stderr, `-render-direction: unknown direction "down"; the directions are TB, LR, RL, BT`) || unknown.stdout != "" {
		t.Errorf("an unknown direction = %d\n%s", unknown.status, unknown.output())
	}
	alone := runStreams(t, binary, layeredModel, "-render-direction", "TB")
	if alone.status != 2 || !strings.Contains(alone.stderr, "-render-direction is the direction a graph-shaped form of -render or -render-all flows in") {
		t.Errorf("-render-direction alone = %d\n%s", alone.status, alone.output())
	}
}

// -render-all writes every view in the direction -render-direction names.
func TestRenderAllDirection(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	got := runStreams(t, binary, layeredModel, "-render-all", dir, "-render-form", "mermaid", "-render-direction", "BT")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d\n%s", got.status, got.output())
	}
	for _, name := range []string{"Platform.plain.mmd", "Platform.tree.mmd"} {
		written, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !strings.Contains(string(written), "flowchart BT\n") {
			t.Errorf("%s is not written bottom to top (%v):\n%s", name, err, written)
		}
	}
}
