package view

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// DrawioFile is a draw.io file read back for one view: the page drawn for it,
// its size and unit, and every cell of the page with its geometry made absolute.
type DrawioFile struct {
	// View is the view the page's cells were rendered from, the writer's `view` attribute.
	View string
	// Page is the diagram's name, Pages every diagram name of the file in order.
	Page  string
	Pages []string
	// PageWidth and PageHeight are the model's page size; Unit its unit when stated.
	PageWidth, PageHeight float64
	Unit                  string
	// Cells are the page's cells in document order, the root cells 0 and 1 left out.
	Cells []DrawioCell
	byID  map[string]*DrawioCell
}

// DrawioCell is one cell of a draw.io page: the writer's handle when the cell is
// one of its semantic objects, and what draw.io keeps of it.
type DrawioCell struct {
	ID, Parent string
	// FQN, Kind, View, Link and Label are the `<object>` attributes a semantic
	// cell wraps; all empty on a plain mxCell, whose Value is its text.
	FQN, Kind, View, Link, Label string
	Value                        string
	Vertex, Edge                 bool
	Collapsed                    bool
	// Style is the style string split at its semicolons; a bare word maps to "".
	Style     map[string]string
	StyleText string
	// Geometry is the cell's box in absolute coordinates, every container's
	// offset added, when HasGeometry; Relative marks an edge or label geometry,
	// which is left as written.
	Geometry    DrawioBox
	HasGeometry bool
	Relative    bool
	// Alternate is the full box a collapsed cell expands to, when stated.
	Alternate *DrawioBox
	// Points are an edge's waypoints, Source and Target the cells it joins.
	Points         []Point
	Source, Target string
}

// DrawioBox is a cell's box, as drawioBox is for the writer.
type DrawioBox struct {
	X, Y, Width, Height float64
}

// Cell is the cell with the given id, nil for none.
func (f *DrawioFile) Cell(id string) *DrawioCell {
	return f.byID[id]
}

// DrawioFileError reports a file that is not a draw.io file this reader reads.
type DrawioFileError struct {
	Message string
	Err     error
}

func (e *DrawioFileError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("not a draw.io file: %s: %v", e.Message, e.Err)
	}
	return "not a draw.io file: " + e.Message
}

func (e *DrawioFileError) Unwrap() error { return e.Err }

// DrawioViewError reports that no page of the file, or more than one, was
// rendered from the requested view; Pages names the views the pages carry.
type DrawioViewError struct {
	View  string
	Pages []string
	Found []string
}

func (e *DrawioViewError) Error() string {
	switch {
	case len(e.Found) > 1 && e.View == "":
		return fmt.Sprintf("the file has %d pages (%s); name the view to import", len(e.Found), strings.Join(e.Found, ", "))
	case len(e.Found) > 1:
		return fmt.Sprintf("%d pages of the file were rendered from %s; the import reads one", len(e.Found), e.View)
	case e.View == "":
		return "no page of the file was rendered from a view; the cells carry no view attribute"
	case len(e.Pages) == 0:
		return fmt.Sprintf("no page of the file was rendered from %s; the cells carry no view attribute", e.View)
	}
	return fmt.Sprintf("no page of the file was rendered from %s; the pages were rendered from %s", e.View, strings.Join(e.Pages, ", "))
}

// drawioPageData is one <diagram> as parsed: its name and its model's XML.
type drawioPageData struct {
	name  string
	model []byte
}

// ReadDrawio reads a draw.io file as the drawio form writes it, or as draw.io
// desktop saves it (each diagram's model deflated, base64 and URL encoded), and
// returns the page rendered from view; with view empty, the file's one page.
func ReadDrawio(data []byte, view string) (*DrawioFile, error) {
	pages, err := drawioPages(data)
	if err != nil {
		return nil, err
	}
	files := make([]*DrawioFile, 0, len(pages))
	var names []string
	for _, page := range pages {
		f, err := readDrawioModel(page.model)
		if err != nil {
			return nil, &DrawioFileError{Message: "page " + strconv.Quote(page.name), Err: err}
		}
		f.Page = page.name
		files = append(files, f)
		names = append(names, page.name)
	}
	var found []*DrawioFile
	var views []string
	for _, f := range files {
		if f.View != "" {
			views = append(views, f.View)
		}
		if f.View != "" && (view == "" || f.View == view) {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		var foundViews []string
		for _, f := range found {
			foundViews = append(foundViews, f.View)
		}
		return nil, &DrawioViewError{View: view, Pages: views, Found: foundViews}
	}
	found[0].Pages = names
	return found[0], nil
}

// drawioPages splits a file into its diagrams, each with its model's XML:
// inline for an uncompressed file, decoded for a compressed one.
func drawioPages(data []byte) ([]drawioPageData, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var pages []drawioPageData
	var (
		root    string
		inFile  bool
		current *drawioPageData
		depth   int
		text    strings.Builder
		model   bytes.Buffer
		enc     *xml.Encoder
	)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &DrawioFileError{Message: "malformed XML", Err: err}
		}
		switch tok := token.(type) {
		case xml.StartElement:
			if root == "" {
				root = tok.Name.Local
				if root != "mxfile" {
					return nil, &DrawioFileError{Message: fmt.Sprintf("the root element is <%s>, not <mxfile>", root)}
				}
				inFile = true
				continue
			}
			if current == nil {
				if !inFile || tok.Name.Local != "diagram" {
					return nil, &DrawioFileError{Message: fmt.Sprintf("<%s> under <mxfile>; a file holds <diagram> pages", tok.Name.Local)}
				}
				current = &drawioPageData{name: attrValue(tok.Attr, "name")}
				text.Reset()
				model.Reset()
				enc = nil
				continue
			}
			if depth == 0 {
				if tok.Name.Local != "mxGraphModel" {
					return nil, &DrawioFileError{Message: fmt.Sprintf("<%s> under <diagram>; a page holds one <mxGraphModel>", tok.Name.Local)}
				}
				enc = xml.NewEncoder(&model)
			}
			depth++
			if err := enc.EncodeToken(tok); err != nil {
				return nil, &DrawioFileError{Message: "malformed XML", Err: err}
			}
		case xml.EndElement:
			switch {
			case current != nil && depth > 0:
				depth--
				if err := enc.EncodeToken(tok); err != nil {
					return nil, &DrawioFileError{Message: "malformed XML", Err: err}
				}
				if depth == 0 {
					if err := enc.Flush(); err != nil {
						return nil, &DrawioFileError{Message: "malformed XML", Err: err}
					}
					current.model = append([]byte(nil), model.Bytes()...)
				}
			case current != nil:
				if current.model == nil {
					decoded, err := decodeDrawioDiagram(text.String())
					if err != nil {
						return nil, &DrawioFileError{Message: "page " + strconv.Quote(current.name) + " holds no model", Err: err}
					}
					current.model = decoded
				}
				pages = append(pages, *current)
				current = nil
			default:
				inFile = false
			}
		case xml.CharData:
			if current != nil && depth == 0 {
				text.Write(tok)
			} else if current != nil {
				if err := enc.EncodeToken(tok); err != nil {
					return nil, &DrawioFileError{Message: "malformed XML", Err: err}
				}
			}
		}
	}
	if root == "" {
		return nil, &DrawioFileError{Message: "the file is empty"}
	}
	if len(pages) == 0 {
		return nil, &DrawioFileError{Message: "the file has no <diagram> page"}
	}
	return pages, nil
}

// decodeDrawioDiagram undoes draw.io's compressed page encoding: base64, raw
// deflate, then URL percent-encoding of the model's XML.
func decodeDrawioDiagram(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("the page is empty")
	}
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	inflated, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	if err != nil {
		return nil, fmt.Errorf("deflate: %w", err)
	}
	unescaped, err := url.PathUnescape(string(inflated))
	if err != nil {
		return nil, fmt.Errorf("percent-decoding: %w", err)
	}
	return []byte(unescaped), nil
}

// readDrawioModel reads one <mxGraphModel> into its cells, the object wrapper's
// handle on each semantic cell and every vertex geometry made absolute.
func readDrawioModel(model []byte) (*DrawioFile, error) {
	decoder := xml.NewDecoder(bytes.NewReader(model))
	f := &DrawioFile{byID: map[string]*DrawioCell{}}
	var (
		objects []map[string]string
		open    []*DrawioCell
		points  bool
		seen    bool
	)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			attrs := map[string]string{}
			for _, a := range tok.Attr {
				attrs[a.Name.Local] = a.Value
			}
			switch tok.Name.Local {
			case "mxGraphModel":
				seen = true
				f.PageWidth, _ = strconv.ParseFloat(attrs["pageWidth"], 64)
				f.PageHeight, _ = strconv.ParseFloat(attrs["pageHeight"], 64)
				f.Unit = attrs["unit"]
			case "object", "UserObject":
				objects = append(objects, attrs)
			case "mxCell":
				cell := &DrawioCell{ID: attrs["id"], Parent: attrs["parent"], Value: attrs["value"],
					Vertex: attrs["vertex"] == "1", Edge: attrs["edge"] == "1", Collapsed: attrs["collapsed"] == "1",
					Source: attrs["source"], Target: attrs["target"], StyleText: attrs["style"], Style: drawioStyleMap(attrs["style"])}
				if len(objects) > 0 {
					o := objects[len(objects)-1]
					cell.ID, cell.FQN, cell.Kind, cell.View, cell.Link, cell.Label = o["id"], o["fqn"], o["kind"], o["view"], o["link"], o["label"]
					objects[len(objects)-1] = nil
				}
				if cell.ID == "" {
					return nil, errors.New("an mxCell has no id")
				}
				if _, dup := f.byID[cell.ID]; dup {
					return nil, fmt.Errorf("cell id %q written twice", cell.ID)
				}
				open = append(open, cell)
			case "mxGeometry":
				if len(open) == 0 {
					return nil, errors.New("an mxGeometry outside every mxCell")
				}
				cell := open[len(open)-1]
				cell.HasGeometry = true
				cell.Relative = attrs["relative"] == "1"
				cell.Geometry, err = drawioBoxOf(attrs)
				if err != nil {
					return nil, fmt.Errorf("cell %s: geometry: %w", cell.ID, err)
				}
			case "mxRectangle":
				if len(open) > 0 && attrs["as"] == "alternateBounds" {
					box, err := drawioBoxOf(attrs)
					if err != nil {
						return nil, fmt.Errorf("cell %s: alternate bounds: %w", open[len(open)-1].ID, err)
					}
					open[len(open)-1].Alternate = &box
				}
			case "Array":
				points = len(open) > 0 && attrs["as"] == "points"
			case "mxPoint":
				if points && len(open) > 0 {
					box, err := drawioBoxOf(attrs)
					if err != nil {
						return nil, fmt.Errorf("cell %s: waypoint: %w", open[len(open)-1].ID, err)
					}
					cell := open[len(open)-1]
					cell.Points = append(cell.Points, Point{X: box.X, Y: box.Y})
				}
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "object", "UserObject":
				if len(objects) > 0 {
					if objects[len(objects)-1] != nil {
						return nil, fmt.Errorf("object %q wraps no mxCell", objects[len(objects)-1]["id"])
					}
					objects = objects[:len(objects)-1]
				}
			case "mxCell":
				if len(open) > 0 {
					cell := open[len(open)-1]
					open = open[:len(open)-1]
					if cell.ID != "0" && cell.ID != "1" {
						f.Cells = append(f.Cells, *cell)
					}
					f.byID[cell.ID] = nil
				}
			case "Array":
				points = false
			}
		}
	}
	if !seen {
		return nil, errors.New("no <mxGraphModel>")
	}
	for i := range f.Cells {
		f.byID[f.Cells[i].ID] = &f.Cells[i]
	}
	f.byID["0"], f.byID["1"] = nil, nil
	if err := f.resolveGeometry(); err != nil {
		return nil, err
	}
	f.View = f.pageView()
	return f, nil
}

// pageView is the view the page's semantic cells name; the one they agree on.
func (f *DrawioFile) pageView() string {
	views := map[string]bool{}
	for _, cell := range f.Cells {
		if cell.View != "" {
			views[cell.View] = true
		}
	}
	if len(views) != 1 {
		return ""
	}
	for v := range views {
		return v
	}
	return ""
}

// resolveGeometry adds every container's offset to its children's boxes, so a
// nested cell's Geometry is absolute like a Layout's coordinates.
func (f *DrawioFile) resolveGeometry() error {
	resolved := map[string]bool{}
	var resolve func(cell *DrawioCell, chain int) error
	resolve = func(cell *DrawioCell, chain int) error {
		if resolved[cell.ID] || !cell.Vertex || !cell.HasGeometry || cell.Relative {
			return nil
		}
		if chain > len(f.Cells) {
			return fmt.Errorf("cell %s: its parents form a cycle", cell.ID)
		}
		parent := f.byID[cell.Parent]
		if parent == nil {
			if cell.Parent != "1" && cell.Parent != "0" {
				if _, known := f.byID[cell.Parent]; !known {
					return fmt.Errorf("cell %s has parent %q, which is no cell", cell.ID, cell.Parent)
				}
			}
			resolved[cell.ID] = true
			return nil
		}
		if err := resolve(parent, chain+1); err != nil {
			return err
		}
		if parent.Vertex && parent.HasGeometry && !parent.Relative {
			cell.Geometry.X += parent.Geometry.X
			cell.Geometry.Y += parent.Geometry.Y
		}
		resolved[cell.ID] = true
		return nil
	}
	ids := make([]string, 0, len(f.Cells))
	for i := range f.Cells {
		ids = append(ids, f.Cells[i].ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := resolve(f.byID[id], 0); err != nil {
			return err
		}
	}
	return nil
}

// drawioBoxOf reads x, y, width and height attributes, each 0 when absent.
func drawioBoxOf(attrs map[string]string) (DrawioBox, error) {
	var box DrawioBox
	for name, field := range map[string]*float64{"x": &box.X, "y": &box.Y, "width": &box.Width, "height": &box.Height} {
		text, ok := attrs[name]
		if !ok || text == "" {
			continue
		}
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return DrawioBox{}, fmt.Errorf("%s=%q is no number", name, text)
		}
		*field = v
	}
	return box, nil
}

// drawioStyleMap splits a style string into its key=value pairs, keeping a bare
// word (shape names like `swimlane`) under its own name with an empty value.
func drawioStyleMap(style string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(style, ";") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		out[key] = value
	}
	return out
}

// attrValue is the named attribute's value, "" when absent.
func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
