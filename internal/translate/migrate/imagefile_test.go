package migrate

import (
	"strings"
	"testing"
)

func TestDescribedImageContentType(t *testing.T) {
	if d := describedImageContentType([]byte("just words")); d != "text/plain; charset=utf-8" {
		t.Errorf("describedImageContentType = %q", d)
	}
	icon := "\x00\x00\x01\x00\x01\x00\x10\x10\x00\x00\x01\x00\x20\x00\x68\x04\x00\x00\x16\x00\x00\x00"
	if d := describedImageContentType([]byte(icon)); d != "image/x-icon" {
		t.Errorf("describedImageContentType icon = %q", d)
	}
	if d := describedImageContentType([]byte(`<?xml version="1.0"?><doc/>`)); d != "text/xml; charset=utf-8; no SVG document: a <doc> document" {
		t.Errorf("describedImageContentType xml = %q", d)
	}
	if d := describedImageContentType([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect>`)); !strings.HasSuffix(d, "; no SVG document: XML syntax error on line 1: unexpected EOF") {
		t.Errorf("describedImageContentType unclosed = %q", d)
	}
}
