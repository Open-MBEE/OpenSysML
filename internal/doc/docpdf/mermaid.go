package docpdf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// mermaidRasterizer draws Mermaid blocks with mermaid-cli (mmdc), which every
// document holding one needs: a missing mmdc fails the render.
type mermaidRasterizer struct {
	mmdc string
}

func (*mermaidRasterizer) name() string { return mermaidTool.name }

func (m *mermaidRasterizer) prepare(string) error {
	mmdc, err := mermaidTool.locate("")
	if err != nil {
		return err
	}
	m.mmdc = mmdc
	return nil
}

// mermaidConfig is the Mermaid configuration one chart is drawn with.
type mermaidConfig struct {
	HTMLLabels  bool       `json:"htmlLabels"`
	MaxTextSize int        `json:"maxTextSize"`
	MaxEdges    int        `json:"maxEdges"`
	Flowchart   htmlLabels `json:"flowchart"`
	Class       htmlLabels `json:"class"`
}

type htmlLabels struct {
	HTMLLabels bool `json:"htmlLabels"`
}

// configFor sizes a chart's configuration to its source: plain <text> labels,
// which PDF-oriented SVG renderers draw where <foreignObject> HTML is lost, and
// the text and edge limits the whole figure fits under.
func configFor(source string) mermaidConfig {
	var config mermaidConfig
	config.MaxTextSize, config.MaxEdges = view.MermaidLimits(source)
	return config
}

func (m *mermaidRasterizer) draw(dir, source, output string) error {
	base := strings.TrimSuffix(output, ".svg")
	input, config := base+".mmd", base+".json"
	if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
		return err
	}
	settings, err := json.Marshal(configFor(source))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, config), settings, 0o600); err != nil {
		return err
	}
	args := []string{"--input", input, "--output", output, "--quiet", "--configFile", config}
	if puppeteer := strings.TrimSpace(os.Getenv(MermaidPuppeteerEnv)); puppeteer != "" {
		args = append(args, "--puppeteerConfigFile", puppeteer)
	}
	return runTool(dir, m.mmdc, args...)
}
