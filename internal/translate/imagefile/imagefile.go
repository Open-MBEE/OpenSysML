// Package imagefile tells what picture some bytes are and what file name to
// write them under, for every place that writes a picture out of a model.
package imagefile

import (
	"bytes"
	"net/http"
	"path"
	"slices"
	"strings"
)

// ContentType reports the image content type of data: an image/* type by its
// signature (PNG, JPEG, GIF, BMP, WebP), or image/svg+xml for an SVG text,
// which the signature sniffer reads as plain XML. "" when data is no image.
func ContentType(data []byte) string {
	if ct := http.DetectContentType(data); strings.HasPrefix(ct, "image/") {
		return ct
	}
	t := bytes.TrimSpace(data)
	if (bytes.HasPrefix(t, []byte("<?xml")) || bytes.HasPrefix(t, []byte("<svg"))) && bytes.Contains(t, []byte("<svg")) {
		return "image/svg+xml"
	}
	return ""
}

// Described is what DetectContentType says of bytes that are no image, for
// telling a reader what was found instead.
func Described(data []byte) string {
	return http.DetectContentType(data)
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

// Name is the base name to write an image of content type ct under: the base
// of name (either path separator), else of fallback, else "image", given
// the type's suffix unless it already carries one of the type's suffixes.
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
