package docpdf

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// svgNamespace is the namespace the root element of a drawn diagram must be in.
const svgNamespace = "http://www.w3.org/2000/svg"

// diagram is one diagram block's fate on the page: the SVG drawn of it, as a
// file name within the working directory, or the notice its source is kept under.
type diagram struct {
	Image  string
	Notice string
}

// rasterizer draws the diagram blocks of one form to SVG with an external tool.
type rasterizer interface {
	// prepare finds the tool and lays out what every drawing shares in dir; a
	// tool that is not installed is an *Error of kind ErrorToolMissing.
	prepare(dir string) error
	// draw writes source's SVG to the file output names within dir.
	draw(dir, source, output string) error
	// name is the tool's name, for the failure reported when it draws nothing.
	name() string
}

// diagramForm is one diagram form's rasterizer and whether it may be absent:
// Mermaid's must be there, while Graphviz and PlantUML are optional and a
// block whose tool is missing is kept as source under a notice.
type diagramForm struct {
	form     string
	optional bool
	draw     rasterizer
	prepared bool
	missing  *Error
}

// diagramForms are the forms a diagram block is written in, each with the
// tool that draws it. Every render starts from a fresh set, so a tool is
// located once per document.
func diagramForms() map[blockKind]*diagramForm {
	return map[blockKind]*diagramForm{
		blockMermaid:  {form: diagramFormName(blockMermaid), draw: &mermaidRasterizer{}},
		blockDOT:      {form: diagramFormName(blockDOT), optional: true, draw: &graphvizRasterizer{}},
		blockPlantUML: {form: diagramFormName(blockPlantUML), optional: true, draw: &plantumlRasterizer{}},
	}
}

// renderDiagrams draws each diagram block to an SVG in dir, one entry per
// Mermaid, DOT or PlantUML block in document order. A document without
// diagrams needs no diagram tool; a missing Mermaid CLI is an error, while a
// missing Graphviz or PlantUML keeps its blocks as source under a notice.
func renderDiagrams(dir string, blocks []block) ([]diagram, error) {
	forms := diagramForms()
	var out []diagram
	for _, blk := range blocks {
		form, ok := forms[blk.Kind]
		if !ok {
			continue
		}
		d, err := form.render(dir, len(out)+1, blk.Source)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// render draws the n-th diagram of the document, or keeps it as source when
// the form's optional tool is absent.
func (f *diagramForm) render(dir string, n int, source string) (diagram, error) {
	if !f.prepared {
		f.prepared = true
		if err := f.draw.prepare(dir); err != nil {
			var missing *Error
			if !f.optional || !errors.As(err, &missing) || missing.Kind != ErrorToolMissing {
				return diagram{}, err
			}
			f.missing = missing
		}
	}
	if f.missing != nil {
		return diagram{Notice: sourceNotice(f.form, f.missing)}, nil
	}
	output := fmt.Sprintf("diagram-%d.svg", n)
	if err := f.draw.draw(dir, source, output); err != nil {
		return diagram{}, err
	}
	if err := checkSVG(filepath.Join(dir, output)); err != nil {
		return diagram{}, &Error{Kind: ErrorToolFailed, Tool: f.draw.name(), Detail: fmt.Sprintf("%s for diagram %d", err, n)}
	}
	return diagram{Image: output}, nil
}

// checkSVG requires the file a tool wrote to be a well-formed XML document
// whose single root is `svg` in the SVG namespace, with nothing but markup around
// it: a tool exiting 0 without a drawing, or with diagnostics around one, fails here.
func checkSVG(path string) error {
	file, err := os.Open(path) // #nosec G304 -- the path is within the render directory
	if err != nil {
		return errors.New("wrote no SVG")
	}
	defer file.Close()
	dec := xml.NewDecoder(file)
	dec.Entity = xml.HTMLEntity
	depth, roots := 0, 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if roots == 0 {
				return errors.New("wrote no SVG")
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("wrote no SVG, %v", err)
		}
		switch node := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if roots > 0 {
					return fmt.Errorf("wrote no SVG, a second root <%s> follows it", node.Name.Local)
				}
				if node.Name.Local != "svg" || node.Name.Space != svgNamespace {
					return fmt.Errorf("wrote no SVG, a <%s> document", node.Name.Local)
				}
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return errors.New("wrote no SVG, text outside the root element")
			}
		}
	}
}

// sourceNotice is written ahead of a diagram block kept as source. With the
// tool that was missing, it names the variable that points at one.
func sourceNotice(form string, missing *Error) string {
	if missing == nil {
		return fmt.Sprintf("This diagram is written in %s, which the PDF backend did not draw; its source follows.", form)
	}
	return fmt.Sprintf("This diagram is written in %s, which the PDF backend did not draw: %s was not found. Install it and point %s at it to have the diagram drawn; its source follows.",
		form, missing.Tool, missing.EnvVar)
}

// diagramFormName is the form a diagram block is written in, as its notice says it.
func diagramFormName(kind blockKind) string {
	switch kind {
	case blockDOT:
		return "Graphviz DOT"
	case blockPlantUML:
		return "PlantUML"
	}
	return "Mermaid"
}

// figureClass is the class the figure keeping a block's source carries.
func figureClass(kind blockKind) string {
	switch kind {
	case blockDOT:
		return "dot"
	case blockPlantUML:
		return "plantuml"
	}
	return "mermaid"
}

// writeDiagramHTML writes one diagram block: the image drawn of it, or its
// source under the notice.
func writeDiagramHTML(b *strings.Builder, blk block, d diagram) {
	if d.Image != "" {
		b.WriteString("<figure><img src=\"" + html.EscapeString(d.Image) + "\" alt=\"diagram\"></figure>\n")
		return
	}
	notice := d.Notice
	if notice == "" {
		notice = sourceNotice(diagramFormName(blk.Kind), nil)
	}
	writeSourceFigure(b, figureClass(blk.Kind), notice, blk.Source)
}

// markdownWithImages rewrites the document's Markdown with each drawn diagram's
// fence replaced by a reference to its image, and each fence kept as source
// preceded by its notice, for converters that read Markdown themselves.
func markdownWithImages(markdown string, diagrams []diagram) string {
	lines := strings.Split(markdown, "\n")
	var out []string
	n := 0
	for i := 0; i < len(lines); i++ {
		kind, ok := fenceKind(lines[i])
		if !ok {
			out = append(out, lines[i])
			continue
		}
		var d diagram
		if n < len(diagrams) {
			d = diagrams[n]
		}
		n++
		if d.Image != "" {
			i = fenceEnd(lines, i+1)
			out = append(out, "![diagram]("+d.Image+")")
			continue
		}
		notice := d.Notice
		if notice == "" {
			notice = sourceNotice(diagramFormName(kind), nil)
		}
		out = append(out, "*"+notice+"*", "", lines[i])
	}
	return strings.Join(out, "\n")
}

// fenceKind is the diagram block a fence line opens.
func fenceKind(line string) (blockKind, bool) {
	switch line {
	case mermaidFence:
		return blockMermaid, true
	case dotFence:
		return blockDOT, true
	case plantumlFence:
		return blockPlantUML, true
	}
	return 0, false
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
