// Package imagefile tells what picture some bytes are and what file name to
// write them under, for every place that writes a picture out of a model.
package imagefile

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strings"
)

// svgNamespace is the XML namespace an SVG document's root element is in.
const svgNamespace = "http://www.w3.org/2000/svg"

// ContentType is the image/* type data's signature gives (PNG, JPEG, GIF, BMP,
// WebP), image/svg+xml for one well-formed SVG document, or "" when data is no image.
func ContentType(data []byte) string {
	if ct := http.DetectContentType(data); strings.HasPrefix(ct, "image/") {
		return ct
	}
	if t := bytes.TrimSpace(data); bytes.HasPrefix(t, []byte("<")) && CheckSVG(t) == nil {
		return "image/svg+xml"
	}
	return ""
}

// CheckSVG is nil when data is one well-formed SVG document — a single root
// element svg in the SVG namespace and no text outside it — else why it is not.
func CheckSVG(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Entity = xml.HTMLEntity
	depth, roots := 0, 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if roots == 0 {
				return errors.New("no root element")
			}
			return nil
		}
		if err != nil {
			return err
		}
		switch node := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if roots > 0 {
					return fmt.Errorf("a second root <%s> follows it", node.Name.Local)
				}
				if node.Name.Local != "svg" || node.Name.Space != svgNamespace {
					return fmt.Errorf("a <%s> document", node.Name.Local)
				}
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return errors.New("text outside the root element")
			}
		}
	}
}

// Described is what DetectContentType says of bytes that are no image, for
// telling a reader what was found instead; a markup text says why it is no SVG.
func Described(data []byte) string {
	ct := http.DetectContentType(data)
	if t := bytes.TrimSpace(data); bytes.HasPrefix(t, []byte("<")) {
		if err := CheckSVG(t); err != nil {
			return ct + "; no SVG document: " + err.Error()
		}
	}
	return ct
}

// extensions are the file suffixes each image content type is written
// under, the first being the canonical one.
var extensions = map[string][]string{
	"image/png":     {".png"},
	"image/jpeg":    {".jpg", ".jpeg"},
	"image/gif":     {".gif"},
	"image/webp":    {".webp"},
	"image/bmp":     {".bmp"},
	"image/svg+xml": {".svg"},
}

// plain reports base names a file can be written under.
func plain(base string) bool {
	return base != "" && base != "." && base != ".." && base != "/"
}

// Name is the file name for an image of type ct: the base of name, else of
// fallback, else "image", given the type's suffix unless it carries one already.
func Name(name, fallback, ct string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if !plain(base) {
		base = path.Base(strings.ReplaceAll(fallback, "\\", "/"))
	}
	if !plain(base) {
		base = "image"
	}
	exts := extensions[ct]
	if len(exts) == 0 {
		return base
	}
	if ext := path.Ext(base); slices.Contains(exts, strings.ToLower(ext)) {
		return base
	} else if base != ext {
		base = strings.TrimSuffix(base, ext)
	}
	return base + exts[0]
}
