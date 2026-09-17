package docpdf

import (
	"os"
	"path/filepath"
)

// Render converts docrender Markdown to PDF bytes with the named engine
// ("" selects the default). Diagrams are pre-rendered to SVG with mermaid-cli
// and formulas to HTML with KaTeX.
func Render(markdown, engine string, opts Options) ([]byte, error) {
	converter, err := EngineNamed(engine)
	if err != nil {
		return nil, err
	}
	if err := converter.Available(); err != nil {
		return nil, err
	}
	blocks, err := parseBlocks(markdown)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "opensysml-docpdf-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	diagrams, err := renderDiagrams(dir, blocks)
	if err != nil {
		return nil, err
	}
	math, err := renderFormulas(dir, blocks)
	if err != nil {
		return nil, err
	}
	doc := &Prepared{Dir: dir, MarkdownFile: "document.md", HTMLFile: "document.html", MathCSS: math.css, Options: opts}
	switch converter.Capabilities().Input {
	case InputMarkdown:
		md := markdownWithFormulas(markdownWithImages(markdownWithSpanCaptions(markdown), diagrams), math)
		if err := os.WriteFile(filepath.Join(dir, doc.MarkdownFile), []byte(md), 0o600); err != nil {
			return nil, err
		}
	case InputHTML:
		page := documentHTML(blocks, artwork{diagrams: diagrams, math: math}, opts)
		if err := os.WriteFile(filepath.Join(dir, doc.HTMLFile), []byte(page), 0o600); err != nil {
			return nil, err
		}
	}
	return converter.Convert(doc)
}
