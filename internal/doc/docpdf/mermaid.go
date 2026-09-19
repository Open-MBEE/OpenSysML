package docpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderDiagrams renders each Mermaid block to an SVG in dir with the pinned
// mermaid-cli, returning the image file names in block order. A document
// without Mermaid diagrams needs no diagram tool at all; a DOT or PlantUML
// block is kept as source (see dotNotice, plantumlNotice) and never handed to
// Graphviz or PlantUML.
func renderDiagrams(dir string, blocks []block) ([]string, error) {
	var sources []string
	for _, blk := range blocks {
		if blk.Kind == blockMermaid {
			sources = append(sources, blk.Source)
		}
	}
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

// dotNotice and plantumlNotice are written ahead of a DOT or PlantUML block
// the PDF backend keeps as source: it draws neither, as it draws no Mermaid
// diagram without mermaid-cli.
const (
	dotNotice      = "This diagram is written in Graphviz DOT, which the PDF backend does not draw; its source follows."
	plantumlNotice = "This diagram is written in PlantUML, which the PDF backend does not draw; its source follows."
)

// markdownWithImages rewrites the document's Markdown with each Mermaid fence
// replaced by a reference to its rendered image, and each DOT or PlantUML
// fence preceded by its notice, for converters that read Markdown themselves.
func markdownWithImages(markdown string, images []string) string {
	lines := strings.Split(markdown, "\n")
	var out []string
	image := 0
	for i := 0; i < len(lines); i++ {
		switch {
		case lines[i] == mermaidFence && image < len(images):
			i = fenceEnd(lines, i+1)
			out = append(out, "![diagram]("+images[image]+")")
			image++
			continue
		case lines[i] == dotFence:
			out = append(out, "*"+dotNotice+"*", "")
		case lines[i] == plantumlFence:
			out = append(out, "*"+plantumlNotice+"*", "")
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

// fenceEnd returns the index of the closing fence at or after from, or
// len(lines) when the fence is left open.
func fenceEnd(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if lines[i] == "```" {
			return i
		}
	}
	return len(lines)
}
