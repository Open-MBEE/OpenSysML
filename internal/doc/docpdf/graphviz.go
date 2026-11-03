package docpdf

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// graphvizRasterizer draws DOT blocks to SVG with Graphviz's dot executable,
// which is optional: a document whose DOT blocks it cannot draw keeps them as
// source. SVG rather than PDF, as the paged-media engines embed an SVG image
// and refuse a PDF one.
type graphvizRasterizer struct {
	dot string
}

func (*graphvizRasterizer) name() string { return graphvizTool.name }

func (g *graphvizRasterizer) prepare(string) error {
	dot, err := graphvizTool.locate("")
	if err != nil {
		return err
	}
	g.dot = dot
	return nil
}

// draw runs Graphviz in dir, the temporary directory the SVG lands in, with
// the pictures' search path kept at the current directory their paths are relative to.
func (g *graphvizRasterizer) draw(dir, source, output string) error {
	input := strings.TrimSuffix(output, ".svg") + ".dot"
	if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args := append(layoutArgs(source), "-Gimagepath="+cwd, "-Tsvg", "-o", output, input)
	if err := runTool(dir, g.dot, args...); err != nil {
		return err
	}
	return embedImages(filepath.Join(dir, output), cwd)
}

// layoutArgs selects the layout engine from the `// layout:` header the DOT
// writer opens a block with: `neato -n` and `neato -n2` keep the positions
// the block states, as -K and -n ask Graphviz to. A block without a header it
// understands is laid out by dot.
func layoutArgs(source string) []string {
	fallback := []string{"-Kdot"}
	engine, flags := view.DOTLayout(source)
	if engine == "" {
		return fallback
	}
	args := []string{"-K" + engine}
	for _, flag := range flags {
		switch flag {
		case "-n", "-n1", "-n2":
			args = append(args, flag)
		default:
			return fallback
		}
	}
	return args
}
