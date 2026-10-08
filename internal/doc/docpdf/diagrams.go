package docpdf

import (
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

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

// diagramTool is one form's rasterizer and whether it may be absent: Graphviz,
// PlantUML and D2 are optional, and without them the diagrams stay source.
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
	case view.FormD2:
		return diagramTool{draw: &d2Rasterizer{}, optional: true}, true
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

var (
	svgImageStartTag = regexp.MustCompile(`<image\b[^>]*>`)
	svgImageEndTag   = regexp.MustCompile(`</image\s*>`)
	svgImageHref     = regexp.MustCompile(`\s(?:xlink:)?href\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// embedImages inlines each safe local image an SVG refers to; refused images
// become comments, while valid data URIs and unread or unsupported files stay as written.
func embedImages(path, base string) error {
	svg, err := os.ReadFile(path) // #nosec G304 -- the path is within the render directory
	if err != nil {
		return nil
	}
	matches := svgImageStartTag.FindAllIndex(svg, -1)
	if len(matches) == 0 {
		return nil
	}
	var out []byte
	at := 0
	for _, match := range matches {
		start, tagEnd := match[0], match[1]
		if start < at {
			continue
		}
		tag := svg[start:tagEnd]
		end := tagEnd
		if !strings.HasSuffix(string(tag), "/>") {
			if closing := svgImageEndTag.FindIndex(svg[tagEnd:]); closing != nil {
				end += closing[1]
			}
		}
		out = append(out, svg[at:start]...)
		href := svgImageHref.FindSubmatchIndex(tag)
		if href == nil {
			out = append(out, svg[start:end]...)
			at = end
			continue
		}
		valueStart, valueEnd := href[2], href[3]
		if valueStart < 0 {
			valueStart, valueEnd = href[4], href[5]
		}
		location := html.UnescapeString(string(tag[valueStart:valueEnd]))
		if location == "" {
			out = append(out, svg[start:end]...)
			at = end
			continue
		}
		if view.RemotePictureLocation(location) {
			out = append(out, refusedImageComment(location, view.ErrRemotePicture.Error())...)
			at = end
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(location)), "data:") {
			if err := view.CheckPicture(location, nil); err != nil {
				out = append(out, refusedImageComment(location, err.Error())...)
			} else {
				out = append(out, svg[start:end]...)
			}
			at = end
			continue
		}
		file := filepath.FromSlash(location)
		if !filepath.IsAbs(file) {
			file = filepath.Join(base, file)
		}
		data, err := os.ReadFile(file) // #nosec G304 -- the path is one the drawn view states
		if err != nil {
			out = append(out, svg[start:end]...)
			at = end
			continue
		}
		if err := view.CheckPicture(location, data); err != nil {
			out = append(out, refusedImageComment(location, err.Error())...)
			at = end
			continue
		}
		ct := imagefile.ContentType(data)
		if ct == "" {
			out = append(out, svg[start:end]...)
			at = end
			continue
		}
		uri := "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
		out = append(out, tag[:valueStart]...)
		out = append(out, uri...)
		out = append(out, tag[valueEnd:]...)
		out = append(out, svg[tagEnd:end]...)
		at = end
	}
	out = append(out, svg[at:]...)
	return os.WriteFile(path, out, 0o600)
}

func refusedImageComment(location, reason string) []byte {
	if view.RemotePictureLocation(location) {
		location = strings.ReplaceAll(location, ":", "&#58;")
	}
	location = strings.ReplaceAll(location, "-", "&#45;")
	reason = strings.ReplaceAll(reason, "-", "&#45;")
	return []byte("<!-- not represented: picture " + location + " not drawn; " + reason + " -->")
}

// checkSVG requires the file a tool wrote to be one well-formed SVG document.
func checkSVG(path string) error {
	svg, err := os.ReadFile(path) // #nosec G304 -- the path is within the render directory
	if err != nil {
		return errors.New("wrote no SVG")
	}
	if err := imagefile.CheckSVG(svg); err != nil {
		return fmt.Errorf("wrote no SVG, %v", err)
	}
	return nil
}
