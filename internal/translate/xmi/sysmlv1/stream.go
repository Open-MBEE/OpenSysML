package sysmlv1

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// symbols is what a tool's own serialization of a diagram draws: the model
// elements its symbols stand for, and the symbols standing for none.
type symbols struct {
	shown []string
	free  map[string]int
}

// errNotSymbols reports a stream that is not a serialized diagram.
var errNotSymbols = errors.New("the stream is not a serialized diagram: expected an mdOwnedViews root")

// errTornSymbols reports a serialized diagram that ends before its symbols close.
var errTornSymbols = errors.New("the serialized diagram is cut short: its symbols are not all closed")

// readSymbols reads a MagicDraw diagram stream: each mdElement symbol, nested
// to any depth, names in its elementID the element it stands for. A top-level
// symbol naming none is free content, a pasted image or text box, counted by
// class; the frame and property symbols name the diagram itself and are neither.
// A stream that ends with a symbol open is torn, and what it drew is unknown.
func readSymbols(data []byte, diagramID string) (*symbols, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	syms := &symbols{free: map[string]int{}}
	type frame struct {
		tag   string
		class string // the symbol class of a top-level mdElement
		named bool   // whether an elementID beneath it named an element
	}
	var stack []frame
	rooted := false
	seen := map[string]bool{}
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
			f := frame{tag: t.Name.Local}
			if t.Name.Local == "mdElement" && len(stack) == 1 {
				f.class = attr(t, "elementClass")
			}
			if t.Name.Local == "elementID" && len(stack) > 0 && stack[len(stack)-1].tag == "mdElement" {
				id := attr(t, "idref")
				if href := attr(t, "href"); href != "" {
					id = strings.TrimPrefix(href, "#")
				}
				if id != "" {
					stack[len(stack)-1].named = true
				}
				if id != "" && id != diagramID && !seen[id] {
					seen[id] = true
					syms.shown = append(syms.shown, id)
				}
			}
			stack = append(stack, f)
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1].tag != t.Name.Local {
				return nil, errTornSymbols
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if f.class != "" && !f.named {
				syms.free[f.class]++
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

// attr is the value of the attribute named local, whatever its prefix.
func attr(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// readStreams reads each diagram's symbols and appends the elements they stand
// for that the tool's list omits; an unreadable stream leaves the list as written.
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
		// Two spellings name one element when they resolve to it; ones resolving
		// to none are the same only when spelled alike.
		elements := map[*Element]bool{}
		dangling := map[string]bool{}
		listed := func(id string) bool {
			if e := m.shown(id); e != nil {
				was := elements[e]
				elements[e] = true
				return was
			}
			was := dangling[id]
			dangling[id] = true
			return was
		}
		for _, ref := range d.Shown {
			listed(ref.ID)
		}
		for _, id := range syms.shown {
			if !listed(id) {
				d.Shown = append(d.Shown, ElementRef{ID: id})
			}
		}
	}
}

// fragment is the element id an href or id names: a list and the symbols may
// spell one element's href by module file or by project resource.
func fragment(id string) string {
	return id[strings.LastIndexByte(id, '#')+1:]
}
