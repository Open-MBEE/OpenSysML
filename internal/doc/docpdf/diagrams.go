package docpdf

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// svgNamespace is the namespace the root element of a drawn diagram must be in.
const svgNamespace = "http://www.w3.org/2000/svg"

// rasterizer draws the diagrams of one form to SVG with an external tool.
type rasterizer interface {
	// prepare finds the tool and lays out what every drawing shares in dir; a
	// tool that is not installed is an *Error of kind ErrorToolMissing.
	prepare(dir string) error
	// draw writes source's SVG to the file output names within dir.
	draw(dir, source, output string) error
	// name is the tool's name, for the failure reported when it draws nothing.
	name() string
}

// diagramTool is one form's rasterizer and whether it may be absent: Graphviz
// and PlantUML are optional, and without them the diagrams stay source.
type diagramTool struct {
	draw     rasterizer
	optional bool
}

// diagramToolFor is the tool that draws diagrams written in form (Mermaid
// when empty), and false for a form no tool draws.
func diagramToolFor(form view.Form) (diagramTool, bool) {
	switch form {
	case "", view.FormMermaid:
		return diagramTool{draw: &mermaidRasterizer{}}, true
	case view.FormDot:
		return diagramTool{draw: &graphvizRasterizer{}, optional: true}, true
	case view.FormPlantUML:
		return diagramTool{draw: &plantumlRasterizer{}, optional: true}, true
	}
	return diagramTool{}, false
}

// Graphviz is the docrender.DiagramDrawer the HTML and Markdown backends draw
// the automatic choice's DOT diagrams with: the dot found on PATH or where
// OPENSYSML_DOT points.
type Graphviz struct{}

// Available reports whether a Graphviz dot is found: what decides whether the
// automatic diagram form draws a positioned view through Graphviz or falls
// back to Mermaid.
func (Graphviz) Available() bool {
	_, err := graphvizTool.locate("")
	return err == nil
}

// Draw is DrawSVG.
func (Graphviz) Draw(diagrams []docrender.Diagram) ([]string, error) { return DrawSVG(diagrams) }

// drawDiagrams draws the document's graph-shaped diagrams into dir, each with
// the tool of its form, one image file name per diagram in order; an empty
// name keeps that diagram as source, as an optional tool that is not installed
// leaves every diagram of its form.
func drawDiagrams(dir string, diagrams []docrender.Diagram) ([]string, error) {
	images := make([]string, len(diagrams))
	prepared := map[view.Form]*diagramTool{}
	for i, diagram := range diagrams {
		tool, ok := prepared[diagram.Form]
		if !ok {
			t, found := diagramToolFor(diagram.Form)
			if found {
				if err := t.draw.prepare(dir); err != nil {
					var missing *Error
					if !t.optional || !errors.As(err, &missing) || missing.Kind != ErrorToolMissing {
						return nil, err
					}
					found = false
				}
			}
			if found {
				tool = &t
			}
			prepared[diagram.Form] = tool
		}
		if tool == nil {
			continue
		}
		output := fmt.Sprintf("diagram-%d.svg", i+1)
		if err := tool.draw.draw(dir, diagram.Source, output); err != nil {
			return nil, err
		}
		if err := checkSVG(filepath.Join(dir, output)); err != nil {
			return nil, &Error{Kind: ErrorToolFailed, Tool: tool.draw.name(), Detail: fmt.Sprintf("%s for diagram %d", err, i+1)}
		}
		images[i] = output
	}
	return images, nil
}

// DrawSVG draws every DOT diagram of a document through Graphviz and returns
// its SVG markup, in order, for the HTML and Markdown backends to write
// inline; a diagram in another form has an empty entry. Callers check
// Graphviz.Available first: a missing dot is an error here, not a fallback.
func DrawSVG(diagrams []docrender.Diagram) ([]string, error) {
	svgs := make([]string, len(diagrams))
	var dot []docrender.Diagram
	var at []int
	for i, diagram := range diagrams {
		if diagram.Form == view.FormDot {
			dot, at = append(dot, diagram), append(at, i)
		}
	}
	if len(dot) == 0 {
		return svgs, nil
	}
	dir, err := os.MkdirTemp("", "opensysml-graphviz-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	images, err := drawDiagrams(dir, dot)
	if err != nil {
		return nil, err
	}
	for j, image := range images {
		if image == "" {
			continue
		}
		svg, err := os.ReadFile(filepath.Join(dir, image)) // #nosec G304 -- the path is within the render directory
		if err != nil {
			return nil, err
		}
		svgs[at[j]] = string(svg)
	}
	return svgs, nil
}

// checkSVG requires the file a tool wrote to be well-formed XML with a single
// root, `svg` in the SVG namespace, and no text outside it.
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
