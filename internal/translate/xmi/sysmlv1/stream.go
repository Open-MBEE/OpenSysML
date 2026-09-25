package sysmlv1

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/imagefile"
)

// Symbol is one symbol of a tool's own serialization of a diagram: what it
// stands for, where it is drawn, and how, as far as the stream says.
type Symbol struct {
	// ID is the symbol's own xmi:id; "" when the tool wrote none.
	ID string
	// Class is the tool's symbol class, such as "Class", "State", "Transition",
	// "Note", "NoteAnchor", "TextBox" or "ImageShape".
	Class string
	// ElementID names the model element the symbol stands for; "" for free
	// content standing for none, such as a pasted image or a text box.
	ElementID string
	// Parent is the symbol this one is drawn inside; nil at the top level.
	Parent *Symbol
	// Bounds is a shape's rectangle in diagram pixels, y down; nil for a path
	// or a shape whose geometry the tool did not write.
	Bounds *Bounds
	// Points are a path's bends, ends included, in drawing order; nil for a shape.
	Points []Point
	// Ends are the symbol ids at a path's first and second end; "" for a shape.
	Ends [2]string
	// Style holds the symbol's own colour and font choices, overriding the
	// tool's defaults for its class; zero when it made none.
	Style Style
	// Text is what a free text box or note says; "" when it says nothing itself.
	Text string
	// Attachment names the file a pasted image was taken from, when the tool
	// wrote one; "" otherwise.
	Attachment string
	// Image holds a pasted image's own bytes when the tool serialized them in
	// the stream; nil when it wrote only the file's name or they do not read.
	Image []byte
}

// Bounds is a rectangle in diagram pixels: its top-left corner and size.
type Bounds struct {
	X, Y, Width, Height float64
}

// Point is a position in diagram pixels.
type Point struct {
	X, Y float64
}

// Style is a symbol's own presentation choices, each "" or nil when the symbol
// inherits the tool's default. Colours are written "#RRGGBB".
type Style struct {
	// Fill, Pen and Text are the fill, line and text colours.
	Fill, Pen, Text string
	// NoFill reports that the symbol is drawn without a fill (USE_FILL_COLOR false).
	NoFill bool
	// Font is the font of the symbol's text.
	Font *Font
}

// Font is a font choice: family, point size and weight/slant.
type Font struct {
	Name         string
	Size         float64
	Bold, Italic bool
}

// Free reports whether the symbol stands for no model element.
func (s *Symbol) Free() bool { return s.ElementID == "" }

// ImageType is the content type of the symbol's inline image bytes, "" when
// it carries none or they are of no image kind.
func (s *Symbol) ImageType() string { return imagefile.ContentType(s.Image) }

// ImageError reports a pasted image whose inline bytes do not read: the
// octet of its text that is not hexadecimal.
type ImageError struct {
	// Diagram and Symbol are the ids of the diagram and the symbol carrying it.
	Diagram, Symbol string
	// Offset is the position of the offending octet, counting from 0; Octet
	// is what stands there.
	Offset int
	Octet  string
}

func (e *ImageError) Error() string {
	return "the pasted image's bytes do not read: " + e.Reason()
}

// Reason says which octet does not read and why.
func (e *ImageError) Reason() string {
	return fmt.Sprintf("octet %d is %q, not a hexadecimal byte", e.Offset, e.Octet)
}

// IsPath reports whether the symbol is drawn as a line between two others.
func (s *Symbol) IsPath() bool {
	return s.Ends[0] != "" || s.Ends[1] != "" || len(s.Points) > 0 && s.Bounds == nil
}

// symbols is what a stream draws: its symbols in serialized order, the model
// elements they stand for, and the top-level symbols standing for none by class.
type symbols struct {
	list   []*Symbol
	frame  *Bounds
	shown  []string
	free   map[string]int
	images []*ImageError
}

// errNotSymbols reports a stream that is not a serialized diagram.
var errNotSymbols = errors.New("the stream is not a serialized diagram: expected an mdOwnedViews root")

// errTornSymbols reports a serialized diagram that ends before its symbols close.
var errTornSymbols = errors.New("the serialized diagram is cut short: its symbols are not all closed")

// attachmentTags are the child tags a pasted image symbol may name its file in;
// its image tag holds the bytes themselves, not a name.
var attachmentTags = map[string]bool{
	"imagePath": true, "imageFile": true, "fileName": true,
	"file": true, "path": true, "url": true, "attachedFile": true, "attachment": true,
}

// readSymbols reads a MagicDraw diagram stream: each mdElement symbol under an
// mdOwnedViews, nested to any depth, names in its elementID the element it
// stands for, in its geometry where it is drawn, in its linkFirstEndID and
// linkSecondEndID what a path joins, and in its properties the colours and font
// it is drawn with, and in its image the bytes of a pasted picture. A top-level
// symbol naming no element is free content, a pasted image or text box,
// counted by class; the frame symbol names the diagram itself and is neither.
// A stream that ends with a symbol open is torn, and what it drew is unknown;
// an image whose bytes do not read is noted and the symbol read without them.
func readSymbols(data []byte, diagramID string) (*symbols, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	syms := &symbols{free: map[string]int{}}
	type frame struct {
		tag    string
		sym    *Symbol   // the symbol this mdElement is, nil for anything else
		prop   *property // the property this mdElement is, nil for anything else
		text   strings.Builder
		valued bool // whether a value child appeared
	}
	var stack []*frame
	rooted := false
	seen := map[string]bool{}
	var symbolPath []*Symbol
	// enclosing is the nearest open symbol, the one a tag belongs to.
	enclosing := func() *Symbol {
		if len(symbolPath) == 0 {
			return nil
		}
		return symbolPath[len(symbolPath)-1]
	}
	// openProperty is the nearest open property, nil when none is.
	openProperty := func() *property {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].prop != nil {
				return stack[i].prop
			}
			if stack[i].sym != nil {
				return nil
			}
		}
		return nil
	}
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !rooted {
				if t.Name.Local != "mdOwnedViews" {
					return nil, errNotSymbols
				}
				rooted = true
			}
			f := &frame{tag: t.Name.Local}
			parentTag := ""
			if len(stack) > 0 {
				parentTag = stack[len(stack)-1].tag
			}
			switch {
			case t.Name.Local == "mdElement" && parentTag == "mdOwnedViews":
				f.sym = &Symbol{ID: attr(t, "id"), Class: attr(t, "elementClass"), Parent: enclosing()}
				syms.list = append(syms.list, f.sym)
				symbolPath = append(symbolPath, f.sym)
			case t.Name.Local == "mdElement" && enclosing() != nil:
				f.prop = &property{class: attr(t, "elementClass")}
			case t.Name.Local == "elementID" && parentTag == "mdElement":
				id := refOf(t)
				if sym := enclosing(); sym != nil && id != "" {
					sym.ElementID = id
				}
				if id != "" && id != diagramID && !seen[id] {
					seen[id] = true
					syms.shown = append(syms.shown, id)
				}
			case (t.Name.Local == "linkFirstEndID" || t.Name.Local == "linkSecondEndID") && parentTag == "mdElement":
				if sym := enclosing(); sym != nil {
					i := 0
					if t.Name.Local == "linkSecondEndID" {
						i = 1
					}
					sym.Ends[i] = refOf(t)
				}
			case t.Name.Local == "value" && parentTag == "mdElement":
				if p := openProperty(); p != nil {
					p.value = attr(t, "value")
					stack[len(stack)-1].valued = true
				}
			case (t.Name.Local == "size" || t.Name.Local == "style") && parentTag == "mdElement":
				if p := openProperty(); p != nil {
					p.set(t.Name.Local, attr(t, "value"))
				}
			}
			stack = append(stack, f)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1].tag != t.Name.Local {
				return nil, errTornSymbols
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			text := strings.TrimSpace(f.text.String())
			switch {
			case f.sym != nil:
				symbolPath = symbolPath[:len(symbolPath)-1]
				if f.sym.Parent == nil && f.sym.Free() {
					syms.free[f.sym.Class]++
				}
				if f.sym.Class == "DiagramFrame" && f.sym.Bounds != nil && syms.frame == nil {
					syms.frame = f.sym.Bounds
				}
			case f.prop != nil:
				if sym := enclosing(); sym != nil {
					f.prop.apply(sym)
				}
			case len(stack) > 0 && stack[len(stack)-1].sym != nil:
				syms.readSymbolField(stack[len(stack)-1].sym, f.tag, text, diagramID)
			case len(stack) > 0 && stack[len(stack)-1].prop != nil:
				p := stack[len(stack)-1].prop
				switch {
				case f.tag == "value" && !f.valued && p.value == "":
					p.value = text
				case text != "":
					p.set(f.tag, text)
				}
			}
		}
	}
	if !rooted {
		return nil, errNotSymbols
	}
	if len(stack) > 0 {
		return nil, errTornSymbols
	}
	return syms, nil
}

// readSymbolField reads a symbol's own child element: its geometry, the text of
// a text box, the bytes of a pasted image or the file it was pasted from.
func (syms *symbols) readSymbolField(sym *Symbol, tag, text, diagramID string) {
	switch {
	case tag == "geometry":
		sym.Bounds, sym.Points = parseGeometry(text)
	case tag == "text":
		sym.Text = text
	case tag == "image" && text != "":
		data, offset, octet := decodeOctets(text)
		if octet != "" {
			syms.images = append(syms.images, &ImageError{Diagram: diagramID, Symbol: sym.ID, Offset: offset, Octet: octet})
			return
		}
		sym.Image = data
	case attachmentTags[tag] && text != "" && sym.Attachment == "":
		sym.Attachment = text
	}
}

// decodeOctets reads whitespace-separated hexadecimal octets of one or two digits,
// as MagicDraw writes them ("d a" for 0x0D 0x0A); the first bad token is returned.
func decodeOctets(text string) (data []byte, offset int, octet string) {
	data = make([]byte, 0, len(text)/3+1)
	for i := 0; i < len(text); {
		if isSpace(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && !isSpace(text[j]) {
			j++
		}
		b, ok := hexOctet(text[i:j])
		if !ok {
			return nil, len(data), text[i:j]
		}
		data = append(data, b)
		i = j
	}
	return data, 0, ""
}

// isSpace reports XML white space.
func isSpace(c byte) bool { return c == ' ' || c == '\n' || c == '\r' || c == '\t' }

// hexOctet reads one or two hexadecimal digits as a byte.
func hexOctet(tok string) (byte, bool) {
	if len(tok) == 0 || len(tok) > 2 {
		return 0, false
	}
	var b byte
	for i := 0; i < len(tok); i++ {
		var d byte
		switch c := tok[i]; {
		case '0' <= c && c <= '9':
			d = c - '0'
		case 'a' <= c && c <= 'f':
			d = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			d = c - 'A' + 10
		default:
			return 0, false
		}
		b = b<<4 | d
	}
	return b, true
}

// property is one presentation property serialized under a symbol: a colour,
// a font or a flag, applied to the symbol once its element closes.
type property struct {
	class, id, value string
	fontName         string
	fontSize         string
	fontStyle        string
}

// set records a property's child by tag.
func (p *property) set(tag, text string) {
	switch tag {
	case "propertyID":
		p.id = text
	case "fontName":
		p.fontName = text
	case "size":
		p.fontSize = text
	case "style":
		p.fontStyle = text
	}
}

// apply writes the property onto the symbol's style, by what it is a property of.
func (p *property) apply(sym *Symbol) {
	switch {
	case p.class == "ColorProperty":
		color, ok := javaColor(p.value)
		if !ok {
			return
		}
		switch p.id {
		case "FILL_COLOR":
			sym.Style.Fill = color
		case "PEN_COLOR":
			sym.Style.Pen = color
		case "TEXT_COLOR":
			sym.Style.Text = color
		}
	case p.class == "FontProperty" && p.id == "FONT":
		font := &Font{Name: p.fontName}
		if size, err := strconv.ParseFloat(p.fontSize, 64); err == nil {
			font.Size = size
		}
		if style, err := strconv.Atoi(p.fontStyle); err == nil {
			font.Bold, font.Italic = style&1 != 0, style&2 != 0
		}
		if font.Name != "" || font.Size != 0 {
			sym.Style.Font = font
		}
	case p.class == "BooleanProperty" && p.id == "USE_FILL_COLOR":
		sym.Style.NoFill = p.value == "false"
	case (p.class == "FileProperty" || p.class == "StringProperty") && p.value != "" && sym.Attachment == "":
		if strings.Contains(p.id, "IMAGE") || strings.Contains(p.id, "FILE") || strings.Contains(p.id, "PATH") {
			sym.Attachment = p.value
		}
	}
}

// javaColor writes a Java ARGB colour integer as "#RRGGBB"; a value that is not
// one, or is fully transparent, is no colour.
func javaColor(value string) (string, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n < -1<<31 || n > 1<<32-1 {
		return "", false
	}
	if n < 0 {
		n += 1 << 32
	}
	if n>>24 == 0 {
		return "", false
	}
	return fmt.Sprintf("#%06X", n&0xFFFFFF), true
}

// parseGeometry reads a MagicDraw geometry: "x, y, width, height" for a shape,
// or "x, y; x, y; ..." for a path's points. Anything else is no geometry.
func parseGeometry(text string) (*Bounds, []Point) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if strings.Contains(text, ";") {
		var points []Point
		for _, pair := range strings.Split(text, ";") {
			if strings.TrimSpace(pair) == "" {
				continue
			}
			nums, ok := numbers(pair)
			if !ok || len(nums) != 2 {
				return nil, nil
			}
			points = append(points, Point{nums[0], nums[1]})
		}
		return nil, points
	}
	nums, ok := numbers(text)
	if !ok || len(nums) != 4 {
		return nil, nil
	}
	return &Bounds{nums[0], nums[1], nums[2], nums[3]}, nil
}

// numbers reads a comma-separated list of numbers.
func numbers(text string) ([]float64, bool) {
	var out []float64
	for _, field := range strings.Split(text, ",") {
		n, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// refOf is the element an idref or href attribute names.
func refOf(t xml.StartElement) string {
	id := attr(t, "idref")
	if href := attr(t, "href"); href != "" {
		id = strings.TrimPrefix(href, "#")
	}
	return id
}

// attr is the value of the attribute named local, whatever its prefix.
func attr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// readStreams lists what each diagram's symbols display: the elements they stand
// for and the listed elements shown under one; an unreadable stream leaves the list.
func (m *Model) readStreams(entries map[string]*zip.File) {
	for i := range m.Diagrams {
		d := &m.Diagrams[i]
		f := entries[d.Stream]
		if d.Stream == "" || f == nil {
			continue
		}
		data, err := readEntry(f)
		if err != nil {
			continue
		}
		syms, err := readSymbols(data, d.ID)
		if err != nil {
			continue
		}
		d.Drawn = true
		d.Free = syms.free
		d.Symbols = syms.list
		d.Frame = syms.frame
		d.ImageErrors = syms.images
		stood := map[*Element]bool{}
		for _, id := range syms.shown {
			if e := m.shown(id); e != nil {
				stood[e] = true
			}
		}
		// Two spellings name one element when they resolve to it; ones resolving
		// to none are the same only when spelled alike.
		elements := map[*Element]bool{}
		dangling := map[string]bool{}
		var shown []ElementRef
		add := func(id string) {
			if e := m.shown(id); e != nil {
				if elements[e] {
					return
				}
				elements[e] = true
			} else {
				if dangling[id] {
					return
				}
				dangling[id] = true
			}
			shown = append(shown, ElementRef{ID: id})
		}
		for _, ref := range d.Shown {
			if displayed(m.shown(ref.ID), stood) {
				add(ref.ID)
			}
		}
		for _, id := range syms.shown {
			add(id)
		}
		d.Shown = shown
	}
}

// displayed reports whether a symbol stands for e or an ancestor of it, which
// shows e in a compartment or on a path.
func displayed(e *Element, stood map[*Element]bool) bool {
	for ; e != nil; e = e.Parent {
		if stood[e] {
			return true
		}
	}
	return false
}

// fragment is the element id an href or id names: a list and the symbols may
// spell one element's href by module file or by project resource.
func fragment(id string) string {
	return id[strings.LastIndexByte(id, '#')+1:]
}
