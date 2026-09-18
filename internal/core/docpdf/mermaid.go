package docpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderDiagrams renders each Mermaid source to an SVG in dir with the pinned
// mermaid-cli, returning the image file names in source order. A document
// without Mermaid diagrams needs no diagram tool at all.
func renderDiagrams(dir string, sources []string) ([]string, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	mmdc, err := mermaidTool.locate("")
	if err != nil {
		return nil, err
	}
	// HTML labels live in <foreignObject>, which PDF-oriented SVG renderers
	// do not draw; plain <text> labels render everywhere.
	config := "mermaid-config.json"
	if err := os.WriteFile(filepath.Join(dir, config), []byte(`{"htmlLabels":false,"flowchart":{"htmlLabels":false},"class":{"htmlLabels":false}}`), 0o600); err != nil {
		return nil, err
	}
	images := make([]string, 0, len(sources))
	for i, source := range sources {
		input := fmt.Sprintf("diagram-%d.mmd", i+1)
		output := fmt.Sprintf("diagram-%d.svg", i+1)
		if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
			return nil, err
		}
		args := []string{"--input", input, "--output", output, "--quiet", "--configFile", config}
		if puppeteer := strings.TrimSpace(os.Getenv(MermaidPuppeteerEnv)); puppeteer != "" {
			args = append(args, "--puppeteerConfigFile", puppeteer)
		}
		if err := runTool(dir, mmdc, args...); err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, output)); err != nil {
			return nil, &Error{Kind: ErrorToolFailed, Tool: mermaidTool.name, Detail: "wrote no SVG for " + input}
		}
		images = append(images, output)
	}
	return images, nil
}
