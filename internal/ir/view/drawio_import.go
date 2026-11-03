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
	// Geometry is the cell's box in absolute coordinates when HasGeometry; a
	// Relative vertex (a port) is resolved from Fraction of its parent plus Offset.
	Geometry    DrawioBox
	HasGeometry bool
	Relative    bool
	Fraction    Point
	Offset      Point
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
// returns the page rendered from view; with view empty, the file's one page. A
// page is of the view its semantic cells name; one with none left is of the
// view its name states, the writer naming each page after its view.
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
		if f.View == "" {
			f.View = page.name
		}
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
	var scanner drawioPageScanner
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, drawioMalformedXML(err)
		}
		if err := scanner.token(token); err != nil {
			return nil, err
		}
	}
	if scanner.root == "" {
		return nil, &DrawioFileError{Message: "the file is empty"}
	}
	if len(scanner.pages) == 0 {
		return nil, &DrawioFileError{Message: "the file has no <diagram> page"}
	}
	return scanner.pages, nil
}

// drawioMalformedXML is the file error for XML encoding/xml refuses.
func drawioMalformedXML(err error) error {
	return &DrawioFileError{Message: "malformed XML", Err: err}
}

// drawioPageScanner reads an <mxfile> token by token: the <diagram> it is in,
// whose <mxGraphModel> it re-encodes verbatim, else whose compressed text it collects.
type drawioPageScanner struct {
	pages   []drawioPageData
	root    string
	inFile  bool
	current *drawioPageData
	depth   int
	text    strings.Builder
	model   bytes.Buffer
	enc     *xml.Encoder
}

func (s *drawioPageScanner) token(token xml.Token) error {
	switch tok := token.(type) {
	case xml.StartElement:
		return s.start(tok)
	case xml.EndElement:
		return s.end(tok)
	case xml.CharData:
		return s.chars(tok)
	}
	return nil
}

// start opens the root, a page, or an element of the page's model.
func (s *drawioPageScanner) start(tok xml.StartElement) error {
	if s.root == "" {
		s.root = tok.Name.Local
		if s.root != "mxfile" {
			return &DrawioFileError{Message: fmt.Sprintf("the root element is <%s>, not <mxfile>", s.root)}
		}
		s.inFile = true
		return nil
	}
	if s.current == nil {
		if !s.inFile || tok.Name.Local != "diagram" {
			return &DrawioFileError{Message: fmt.Sprintf("<%s> under <mxfile>; a file holds <diagram> pages", tok.Name.Local)}
		}
		s.current = &drawioPageData{name: attrValue(tok.Attr, "name")}
		s.text.Reset()
		s.model.Reset()
		s.enc = nil
		return nil
	}
	if s.depth == 0 {
		if tok.Name.Local != "mxGraphModel" {
			return &DrawioFileError{Message: fmt.Sprintf("<%s> under <diagram>; a page holds one <mxGraphModel>", tok.Name.Local)}
		}
		s.enc = xml.NewEncoder(&s.model)
	}
	s.depth++
	return s.encode(tok)
}

// end closes an element of the model, finishing it at the model's own close,
// or a page, decoding its text when it held no inline model, or the root.
func (s *drawioPageScanner) end(tok xml.EndElement) error {
	switch {
	case s.current != nil && s.depth > 0:
		s.depth--
		if err := s.encode(tok); err != nil {
			return err
		}
		if s.depth == 0 {
			if err := s.enc.Flush(); err != nil {
				return drawioMalformedXML(err)
			}
			s.current.model = append([]byte(nil), s.model.Bytes()...)
		}
	case s.current != nil:
		if s.current.model == nil {
			decoded, err := decodeDrawioDiagram(s.text.String())
			if err != nil {
				return &DrawioFileError{Message: "page " + strconv.Quote(s.current.name) + " holds no model", Err: err}
			}
			s.current.model = decoded
		}
		s.pages = append(s.pages, *s.current)
		s.current = nil
	default:
		s.inFile = false
	}
	return nil
}

// chars is a page's compressed text, or character data inside its model.
func (s *drawioPageScanner) chars(tok xml.CharData) error {
	if s.current == nil {
		return nil
	}
	if s.depth == 0 {
		s.text.Write(tok)
		return nil
	}
	return s.encode(tok)
}

func (s *drawioPageScanner) encode(tok xml.Token) error {
	if err := s.enc.EncodeToken(tok); err != nil {
		return drawioMalformedXML(err)
	}
	return nil
}

// drawioInflateLimit bounds the XML a compressed page may inflate to, so a
// small file cannot exhaust memory.
const drawioInflateLimit = 64 << 20

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
	inflated, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), drawioInflateLimit+1))
	if err != nil {
		return nil, fmt.Errorf("deflate: %w", err)
	}
	if len(inflated) > drawioInflateLimit {
		return nil, fmt.Errorf("the page inflates past %d MiB", drawioInflateLimit>>20)
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
	r := drawioModelReader{f: &DrawioFile{byID: map[string]*DrawioCell{}}}
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
			err = r.start(tok)
		case xml.EndElement:
			err = r.end(tok)
		}
		if err != nil {
			return nil, err
		}
	}
	if !r.seen {
		return nil, errors.New("no <mxGraphModel>")
	}
	f := r.f
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

// drawioModelReader reads an <mxGraphModel> element by element: the <object>
// wrappers and cells open around the current element, and whether a waypoint Array is.
type drawioModelReader struct {
	f       *DrawioFile
	objects []map[string]string
	open    []*DrawioCell
	points  bool
	seen    bool
}

func (r *drawioModelReader) start(tok xml.StartElement) error {
	attrs := map[string]string{}
	for _, a := range tok.Attr {
		attrs[a.Name.Local] = a.Value
	}
	switch tok.Name.Local {
	case "mxGraphModel":
		r.seen = true
		r.f.PageWidth, _ = strconv.ParseFloat(attrs["pageWidth"], 64)
		r.f.PageHeight, _ = strconv.ParseFloat(attrs["pageHeight"], 64)
		r.f.Unit = attrs["unit"]
	case "object", "UserObject":
		r.objects = append(r.objects, attrs)
	case "mxCell":
		return r.openCell(attrs)
	case "mxGeometry":
		return r.geometry(attrs)
	case "mxRectangle":
		return r.rectangle(attrs)
	case "Array":
		r.points = len(r.open) > 0 && attrs["as"] == "points"
	case "mxPoint":
		return r.point(attrs)
	}
	return nil
}

func (r *drawioModelReader) end(tok xml.EndElement) error {
	switch tok.Name.Local {
	case "object", "UserObject":
		if n := len(r.objects); n > 0 {
			if r.objects[n-1] != nil {
				return fmt.Errorf("object %q wraps no mxCell", r.objects[n-1]["id"])
			}
			r.objects = r.objects[:n-1]
		}
	case "mxCell":
		if cell := r.innermost(); cell != nil {
			r.open = r.open[:len(r.open)-1]
			if cell.ID != "0" && cell.ID != "1" {
				r.f.Cells = append(r.f.Cells, *cell)
			}
			r.f.byID[cell.ID] = nil
		}
	case "Array":
		r.points = false
	}
	return nil
}

// innermost is the cell open around the element being read, nil outside every cell.
func (r *drawioModelReader) innermost() *DrawioCell {
	if len(r.open) == 0 {
		return nil
	}
	return r.open[len(r.open)-1]
}

// openCell opens an mxCell, taking the handle of the <object> wrapping it when one does.
func (r *drawioModelReader) openCell(attrs map[string]string) error {
	cell := &DrawioCell{ID: attrs["id"], Parent: attrs["parent"], Value: attrs["value"],
		Vertex: attrs["vertex"] == "1", Edge: attrs["edge"] == "1", Collapsed: attrs["collapsed"] == "1",
		Source: attrs["source"], Target: attrs["target"], StyleText: attrs["style"], Style: drawioStyleMap(attrs["style"])}
	if n := len(r.objects); n > 0 {
		o := r.objects[n-1]
		cell.ID, cell.FQN, cell.Kind, cell.View, cell.Link, cell.Label = o["id"], o["fqn"], o["kind"], o["view"], o["link"], o["label"]
		r.objects[n-1] = nil
	}
	if cell.ID == "" {
		return errors.New("an mxCell has no id")
	}
	if _, dup := r.f.byID[cell.ID]; dup {
		return fmt.Errorf("cell id %q written twice", cell.ID)
	}
	r.open = append(r.open, cell)
	return nil
}

// geometry is the open cell's mxGeometry.
func (r *drawioModelReader) geometry(attrs map[string]string) error {
	cell := r.innermost()
	if cell == nil {
		return errors.New("an mxGeometry outside every mxCell")
	}
	cell.HasGeometry = true
	cell.Relative = attrs["relative"] == "1"
	var err error
	if cell.Geometry, err = drawioBoxOf(attrs); err != nil {
		return fmt.Errorf("cell %s: geometry: %w", cell.ID, err)
	}
	return nil
}

// rectangle is the open cell's alternate bounds, the box a collapsed cell expands to.
func (r *drawioModelReader) rectangle(attrs map[string]string) error {
	cell := r.innermost()
	if cell == nil || attrs["as"] != "alternateBounds" {
		return nil
	}
	box, err := drawioBoxOf(attrs)
	if err != nil {
		return fmt.Errorf("cell %s: alternate bounds: %w", cell.ID, err)
	}
	cell.Alternate = &box
	return nil
}

// point is one of the open cell's waypoints inside a points Array, else its label offset.
func (r *drawioModelReader) point(attrs map[string]string) error {
	cell := r.innermost()
	if cell == nil {
		return nil
	}
	switch {
	case r.points:
		box, err := drawioBoxOf(attrs)
		if err != nil {
			return fmt.Errorf("cell %s: waypoint: %w", cell.ID, err)
		}
		cell.Points = append(cell.Points, Point{X: box.X, Y: box.Y})
	case attrs["as"] == "offset":
		box, err := drawioBoxOf(attrs)
		if err != nil {
			return fmt.Errorf("cell %s: offset: %w", cell.ID, err)
		}
		cell.Offset = Point{X: box.X, Y: box.Y}
	}
	return nil
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
		if resolved[cell.ID] || !cell.Vertex || !cell.HasGeometry {
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
		if parent.Vertex && parent.HasGeometry {
			if cell.Relative {
				cell.Fraction = Point{X: cell.Geometry.X, Y: cell.Geometry.Y}
				cell.Geometry.X = cell.Fraction.X*parent.Geometry.Width + cell.Offset.X
				cell.Geometry.Y = cell.Fraction.Y*parent.Geometry.Height + cell.Offset.Y
			}
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
