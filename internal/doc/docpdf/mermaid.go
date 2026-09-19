package docpdf

import (
	"os"
	"path/filepath"
	"strings"
)

// mermaidRasterizer draws Mermaid blocks with mermaid-cli (mmdc), which every
// document holding one needs: a missing mmdc fails the render.
type mermaidRasterizer struct {
	mmdc   string
	config string
}

func (*mermaidRasterizer) name() string { return mermaidTool.name }

func (m *mermaidRasterizer) prepare(dir string) error {
	mmdc, err := mermaidTool.locate("")
	if err != nil {
		return err
	}
	m.mmdc = mmdc
	// HTML labels live in <foreignObject>, which PDF-oriented SVG renderers
	// do not draw; plain <text> labels render everywhere.
	m.config = "mermaid-config.json"
	return os.WriteFile(filepath.Join(dir, m.config), []byte(`{"htmlLabels":false,"flowchart":{"htmlLabels":false},"class":{"htmlLabels":false}}`), 0o600)
}

func (m *mermaidRasterizer) draw(dir, source, output string) error {
	input := strings.TrimSuffix(output, ".svg") + ".mmd"
	if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	args := []string{"--input", input, "--output", output, "--quiet", "--configFile", m.config}
	if puppeteer := strings.TrimSpace(os.Getenv(MermaidPuppeteerEnv)); puppeteer != "" {
		args = append(args, "--puppeteerConfigFile", puppeteer)
	}
	return runTool(dir, m.mmdc, args...)
}
