// Package imagefile preserves the translation image helpers.
package imagefile

import irimagefile "github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"

// ContentType recognizes supported image formats from their bytes.
func ContentType(data []byte) string { return irimagefile.ContentType(data) }

// CheckSVG reports whether data is one well-formed SVG document.
func CheckSVG(data []byte) error { return irimagefile.CheckSVG(data) }

// Described returns the detected type and why unsupported XML is not SVG.
func Described(data []byte) string { return irimagefile.Described(data) }

// Name returns a safe base name with the suffix for ct.
func Name(name, fallback, ct string) string { return irimagefile.Name(name, fallback, ct) }
