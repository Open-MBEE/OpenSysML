package docpdf

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// d2Rasterizer draws D2 blocks to SVG with the d2 executable, which is
// optional: a document whose D2 blocks it cannot draw keeps them as source.
type d2Rasterizer struct {
	d2 string
}

func (*d2Rasterizer) name() string { return d2Tool.name }

func (d *d2Rasterizer) prepare(string) error {
	d2, err := d2Tool.locate("")
	if err != nil {
		return err
	}
	d.d2 = d2
	return nil
}

// draw runs d2 in dir on the block written to a .d2 file beside the SVG; the
// layout engine is named so a D2_LAYOUT in the environment does not pick one.
func (d *d2Rasterizer) draw(dir, source, output string) error {
	input := strings.TrimSuffix(output, ".svg") + ".d2"
	if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	if err := runTool(dir, d.d2, "--layout=dagre", "--pad=16", input, output); err != nil {
		return err
	}
	return defineMasks(filepath.Join(dir, output))
}

// svgMask matches one <mask> element; d2 writes them flat, never nested.
var svgMask = regexp.MustCompile(`(?s)<mask\b[^>]*>.*?</mask>`)

// defineMasks wraps each <mask> of the SVG at path in <defs>. d2 writes the
// mask that cuts a connection's label out of its line after the connections
// that use it, and WeasyPrint, which rewrites a mask into a group as it
// applies it, then draws that group as content: a canvas-sized white
// rectangle over the whole figure. A mask under <defs> is only ever applied.
func defineMasks(path string) error {
	svg, err := os.ReadFile(path) // #nosec G304 -- the path is within the render directory
	if err != nil {
		return err
	}
	var out []byte
	last := 0
	for _, at := range svgMask.FindAllIndex(svg, -1) {
		out = append(out, svg[last:at[0]]...)
		if strings.HasSuffix(strings.TrimRight(string(out), " \t\r\n"), "<defs>") {
			out = append(out, svg[at[0]:at[1]]...)
		} else {
			out = append(append(append(out, "<defs>"...), svg[at[0]:at[1]]...), "</defs>"...)
		}
		last = at[1]
	}
	if out == nil {
		return nil
	}
	out = append(out, svg[last:]...)
	return os.WriteFile(path, out, 0o600)
}
