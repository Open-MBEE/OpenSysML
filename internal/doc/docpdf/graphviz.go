package docpdf

import (
	"os"
	"path/filepath"
	"strings"
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

func (g *graphvizRasterizer) draw(dir, source, output string) error {
	input := strings.TrimSuffix(output, ".svg") + ".dot"
	if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	args := append(layoutArgs(source), "-Tsvg", "-o", output, input)
	return runTool(dir, g.dot, args...)
}

// graphvizEngines are the layout engines a `// layout:` header may name.
var graphvizEngines = map[string]bool{
	"dot": true, "neato": true, "fdp": true, "sfdp": true, "circo": true, "twopi": true, "osage": true, "patchwork": true,
}

// layoutArgs selects the layout engine from the `// layout:` header the DOT
// writer opens a block with: `neato -n` and `neato -n2` keep the positions
// the block states, as -K and -n ask Graphviz to. A block without a header it
// understands is laid out by dot.
func layoutArgs(source string) []string {
	fallback := []string{"-Kdot"}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break
		}
		spec, ok := strings.CutPrefix(line, "// layout:")
		if !ok {
			continue
		}
		fields := strings.Fields(spec)
		if len(fields) == 0 || !graphvizEngines[fields[0]] {
			return fallback
		}
		args := []string{"-K" + fields[0]}
		for _, flag := range fields[1:] {
			switch flag {
			case "-n", "-n1", "-n2":
				args = append(args, flag)
			default:
				return fallback
			}
		}
		return args
	}
	return fallback
}
