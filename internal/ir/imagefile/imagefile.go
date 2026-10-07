// Package imagefile tells what picture some bytes are and what file name to
// write them under, for every place that writes a picture out of a model.
package imagefile

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strings"
)

// svgNamespace is the XML namespace an SVG document's root element is in.
const svgNamespace = "http://www.w3.org/2000/svg"

// ContentType is the type data's signature gives when it is one of the images
// written (PNG, JPEG, GIF, BMP, WebP), image/svg+xml for one well-formed SVG
// document, or "" when data is no such image.
func ContentType(data []byte) string {
	if ct := signedContentType(data); ct != "" {
		return ct
	}
	if t := bytes.TrimSpace(data); bytes.HasPrefix(t, []byte("<")) && CheckSVG(t) == nil {
		return "image/svg+xml"
	}
	return ""
}

// DecodeDataURL decodes a data URL and reports its declared media type, if any.
func DecodeDataURL(u string) (mediaType string, data []byte, err error) {
	if len(u) < len("data:") || !strings.EqualFold(u[:len("data:")], "data:") {
		return "", nil, errors.New("not a data URL")
	}
	metadata, payload, hasPayload := strings.Cut(u[len("data:"):], ",")
	mediaType, _, _ = strings.Cut(metadata, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if !hasPayload {
		return mediaType, nil, errors.New("missing data URL payload")
	}
	if strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		data, err = base64.StdEncoding.DecodeString(payload)
		return mediaType, data, err
	}
	decoded, err := url.PathUnescape(payload)
	return mediaType, []byte(decoded), err
}

func signedContentType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	case bytes.HasPrefix(data, []byte("BM")):
		return "image/bmp"
	case len(data) >= 14 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:14], []byte("WEBPVP")):
		return "image/webp"
	default:
		return ""
	}
}

// CheckSVG is nil when data is one well-formed SVG document — a single root
// element svg in the SVG namespace and no text outside it — else why it is not.
func CheckSVG(data []byte) error { return checkSVG(data, false, 0) }

// ActiveContentError is an SVG construct that can load or execute active content.
type ActiveContentError struct {
	Construct string
}

func (e *ActiveContentError) Error() string {
	return "the SVG has active content (" + e.Construct + ")"
}

// CheckStaticSVG is nil when data is one well-formed SVG document without active
// content that can execute or load external resources.
func CheckStaticSVG(data []byte) error { return checkSVG(data, true, 0) }

func checkSVG(data []byte, static bool, dataSVGDepth int) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Entity = xml.HTMLEntity
	depth, roots := 0, 0
	var active error
	styleDepth := 0
	var styleText strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if roots == 0 {
				return errors.New("no root element")
			}
			if active != nil {
				return active
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
			if static && active == nil {
				active = svgElementConstruct(node.Name.Local)
				if active == nil {
					active = svgAttributeConstruct(node, dataSVGDepth)
				}
				if active == nil && strings.EqualFold(node.Name.Local, "style") {
					styleDepth = depth + 1
					styleText.Reset()
				}
			}
			depth++
		case xml.EndElement:
			if depth == styleDepth {
				styleDepth = 0
				styleText.Reset()
			}
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(node)) != "" {
				return errors.New("text outside the root element")
			}
			if static && active == nil && styleDepth > 0 {
				_, _ = styleText.Write(node)
				if construct := cssReference(styleText.String()); construct != "" {
					active = &ActiveContentError{Construct: construct + " in <style>"}
				}
			}
		case xml.ProcInst:
			if static && active == nil && strings.EqualFold(node.Target, "xml-stylesheet") {
				active = &ActiveContentError{Construct: "<?xml-stylesheet?>"}
			}
		case xml.Directive:
			if static && active == nil && strings.Contains(string(node), "<!ENTITY") {
				active = &ActiveContentError{Construct: "an <!ENTITY> declaration"}
			}
		}
	}
}

func svgElementConstruct(name string) error {
	switch strings.ToLower(name) {
	case "script", "foreignobject", "iframe", "embed", "object", "handler", "listener":
		return &ActiveContentError{Construct: "<" + name + ">"}
	}
	return nil
}

var svgPresentationURLAttributes = map[string]struct{}{
	"fill": {}, "stroke": {}, "filter": {}, "clip-path": {}, "mask": {},
	"marker": {}, "marker-start": {}, "marker-mid": {}, "marker-end": {}, "cursor": {},
}

func svgAttributeConstruct(element xml.StartElement, dataSVGDepth int) error {
	isAnimation := false
	switch strings.ToLower(element.Name.Local) {
	case "set", "animate", "animatetransform", "animatemotion", "animatecolor":
		isAnimation = true
	}
	for _, attr := range element.Attr {
		local := strings.ToLower(attr.Name.Local)
		if attr.Name.Space == "xmlns" || local == "xmlns" {
			continue
		}
		switch {
		case strings.HasPrefix(local, "on"):
			return &ActiveContentError{Construct: "an " + attr.Name.Local + " attribute on <" + element.Name.Local + ">"}
		case local == "href":
			value := strings.TrimSpace(attr.Value)
			lowerValue := strings.ToLower(value)
			if strings.HasPrefix(lowerValue, "data:image/") {
				mediaType, data, err := DecodeDataURL(value)
				if mediaType == "image/svg+xml" {
					if dataSVGDepth >= 4 {
						return &ActiveContentError{Construct: "data: SVG hrefs nested too deeply"}
					}
					if err != nil {
						return &ActiveContentError{Construct: "an undecodable data: href on <" + element.Name.Local + ">"}
					}
					if err := checkSVG(data, true, dataSVGDepth+1); err != nil {
						var inner *ActiveContentError
						if errors.As(err, &inner) {
							if inner.Construct == "data: SVG hrefs nested too deeply" {
								return inner
							}
							return &ActiveContentError{Construct: "a data: SVG href on <" + element.Name.Local + "> with " + inner.Construct}
						}
						return &ActiveContentError{Construct: "a malformed data: SVG href on <" + element.Name.Local + ">"}
					}
				}
			} else if !strings.HasPrefix(value, "#") {
				name := "an external href on <" + element.Name.Local + ">"
				if attr.Name.Space != "" {
					name = "an external xlink:href on <" + element.Name.Local + ">"
				}
				return &ActiveContentError{Construct: name}
			}
		case local == "base" && (attr.Name.Space == "xml" || attr.Name.Space == "http://www.w3.org/XML/1998/namespace"):
			return &ActiveContentError{Construct: "an xml:base attribute on <" + element.Name.Local + ">"}
		case isAnimation && local == "attributename":
			animated := strings.TrimSpace(attr.Value)
			if colon := strings.LastIndexByte(animated, ':'); colon >= 0 {
				animated = animated[colon+1:]
			}
			target := strings.ToLower(animated)
			if target == "href" || strings.HasPrefix(target, "on") {
				return &ActiveContentError{Construct: "<" + element.Name.Local + "> animating " + attr.Value}
			}
		}
		if local == "style" {
			if construct := cssReference(attr.Value); construct != "" {
				return &ActiveContentError{Construct: construct + " in a style attribute on <" + element.Name.Local + ">"}
			}
		} else if _, ok := svgPresentationURLAttributes[local]; ok {
			if construct := cssReference(attr.Value); construct != "" {
				return &ActiveContentError{Construct: construct + " in the " + attr.Name.Local + " attribute on <" + element.Name.Local + ">"}
			}
		}
	}
	return nil
}

func cssReference(text string) string {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, `\`):
		return "a CSS escape"
	case strings.Contains(text, "@import"):
		return "@import"
	case strings.Contains(text, "image-set("):
		return "image-set()"
	}
	for rest := text; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			break
		}
		argument := strings.TrimLeft(rest[i+4:], " \t\r\n\f")
		if len(argument) > 0 && (argument[0] == '\'' || argument[0] == '"') {
			argument = argument[1:]
			argument = strings.TrimLeft(argument, " \t\r\n\f")
		}
		if !strings.HasPrefix(argument, "#") {
			return "url()"
		}
		rest = rest[i+4:]
	}
	return ""
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
