package docrender

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestActiveMarkup checks what counts as markup that would run in a page, and
// that the output of the typesetter and the drawing tools does not.
func TestActiveMarkup(t *testing.T) {
	for _, active := range []string{
		`<svg><script>alert(1)</script></svg>`,
		`<svg><SCRIPT href="x.js"/></svg>`,
		`<svg><svg:script>alert(1)</svg:script></svg>`,
		`<svg><g onclick="alert(1)"/></svg>`,
		`<svg><g ONLOAD="alert(1)"/></svg>`,
		`<svg><a href="javascript:alert(1)"><text>x</text></a></svg>`,
		`<svg><a xlink:href=" java&#10;script:alert(1)"><text>x</text></a></svg>`,
		`<svg><a href="JaVaScRiPt:alert(1)"/></svg>`,
		`<svg><a href="data:text/html,<script>alert(1)</script>"/></svg>`,
		`<svg><foreignObject><iframe src="https://example.test"></iframe></foreignObject></svg>`,
		`<svg><set attributeName="href" to="javascript:alert(1)"/></svg>`,
		`<span class="katex"><img src=x onerror=alert(1)></span>`,
		`<span></span></div><script>alert(1)</script>`,
		`<object data="x.swf"></object>`,
	} {
		if _, found := activeMarkup(active); !found {
			t.Errorf("not refused: %s", active)
		}
	}
	for _, inert := range []string{
		`<span class="katex"><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.6833em;"></span><span class="mord mathnormal">E</span></span></span></span>`,
		`<?xml version="1.0"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><style>.n{fill:#fff}</style><g id="node1" class="node"><title>on&lt;script&gt;</title><a xlink:href="https://example.test/a?b=1#c" xlink:title="onclick"><text>javascript: is a language</text></a></g><image href="data:image/png;base64,AAAA"/><image xlink:href="pictures/logo.svg"/><use href="#node1"/></svg>`,
		`<svg><foreignObject><div xmlns="http://www.w3.org/1999/xhtml"><span class="nodeLabel">Engine</span></div></foreignObject></svg>`,
	} {
		if found, active := activeMarkup(inert); active {
			t.Errorf("refused for %s: %s", found, inert)
		}
	}
}

// TestBackendsRefuseActiveMarkup checks every backend refuses a drawn diagram that
// would run in the page, and the HTML backend a typeset formula that would.
func TestBackendsRefuseActiveMarkup(t *testing.T) {
	document := fixtureDocumentSet(t, filepath.Join("testdata", "linked_reports.sysml"), []string{"Observatory::SystemReport"})[0]
	svg := []string{`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`}
	var refused *Error
	if _, err := HTML(document, HTMLOptions{DiagramSVG: svg}); !errors.As(err, &refused) || refused.Kind != ErrorUnsafeDiagram {
		t.Errorf("HTML: err = %v, want %s", err, ErrorUnsafeDiagram)
	} else if !strings.Contains(err.Error(), "<script>") {
		t.Errorf("HTML: the refusal does not name what it found: %v", err)
	}
	if _, err := Markdown(document, MarkdownOptions{DiagramSVG: svg}); !errors.As(err, &refused) || refused.Kind != ErrorUnsafeDiagram {
		t.Errorf("Markdown: err = %v, want %s", err, ErrorUnsafeDiagram)
	}
	if _, err := AsciiDoc(document, AsciiDocOptions{DiagramSVG: svg}); !errors.As(err, &refused) || refused.Kind != ErrorUnsafeDiagram {
		t.Errorf("AsciiDoc: err = %v, want %s", err, ErrorUnsafeDiagram)
	}
	math := map[Formula]string{{Source: "E = mc^2"}: `<span class="katex">E</span><img src=x onerror=alert(1)>`}
	if _, err := HTML(document, HTMLOptions{Math: math}); !errors.As(err, &refused) || refused.Kind != ErrorUnsafeMath {
		t.Errorf("HTML: err = %v, want %s", err, ErrorUnsafeMath)
	}
}
