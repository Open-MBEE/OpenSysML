package docpdf

import (
	"os"
	"path/filepath"
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
	return runTool(dir, d.d2, "--layout=dagre", "--pad=16", input, output)
}
