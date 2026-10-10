package jupyter

import (
	"encoding/json"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// The MIME types the rich forms are shown as.
const (
	mimeText     = "text/plain"
	mimeMarkdown = "text/markdown"
	mimeHTML     = "text/html"
	mimeMermaid  = "text/vnd.mermaid"
	mimeDot      = "text/vnd.graphviz"
	mimeSVG      = "image/svg+xml"
	mimeCSV      = "text/csv"
	mimeTSV      = "text/tab-separated-values"
	mimeJSON     = "application/json"
)

// htmlBundle is an HTML fragment, shown as HTML by a front end that renders
// it and as its source by one that does not.
func htmlBundle(lines []string) MIMEBundle {
	text := strings.Join(lines, "\n")
	return MIMEBundle{mimeText: text, mimeHTML: text}
}

// textBundle is a plain-text output.
func textBundle(lines []string) MIMEBundle {
	return MIMEBundle{mimeText: strings.Join(lines, "\n")}
}

// formBundle packages what %render wrote in the form it was asked for: the
// form's own MIME type beside the plain text every front end can show, and for
// a DOT diagram the SVG Graphviz draws when it is installed.
func formBundle(form view.Form, lines []string) MIMEBundle {
	text := strings.Join(lines, "\n")
	bundle := MIMEBundle{mimeText: text}
	switch form {
	case view.FormMermaid:
		bundle[mimeMermaid] = text
	case view.FormMarkdown:
		bundle[mimeMarkdown] = text
	case view.FormCSV:
		bundle[mimeCSV] = text
	case view.FormTSV:
		bundle[mimeTSV] = text
	case view.FormDot:
		bundle[mimeDot] = text
		if svg, ok := drawDot(text); ok {
			bundle[mimeSVG] = svg
		}
	}
	return bundle
}

// vizBundle packages what %viz drew: the bundle of each form it was written
// in, merged, the first form's plain text being the one every front end shows.
func vizBundle(rendered []repl.Rendered) MIMEBundle {
	bundle := MIMEBundle{}
	for i := len(rendered) - 1; i >= 0; i-- {
		for mime, data := range formBundle(rendered[i].Form, rendered[i].Lines) {
			bundle[mime] = data
		}
	}
	return bundle
}

// drawDot is the SVG of a DOT diagram when a Graphviz drawer is linked and
// installed; false otherwise, and the diagram is shown as its source.
func drawDot(dot string) (string, bool) {
	drawer := replext.Drawer()
	if drawer == nil || !drawer.Available() {
		return "", false
	}
	svgs, err := drawer.Draw([]docrender.Diagram{{Form: view.FormDot, Source: dot}})
	if err != nil || len(svgs) != 1 || svgs[0] == "" {
		return "", false
	}
	return svgs[0], true
}

// markdownBundle is a Markdown output, as %render-document writes.
func markdownBundle(lines []string) MIMEBundle {
	text := strings.Join(lines, "\n")
	return MIMEBundle{mimeText: text, mimeMarkdown: text}
}

// jsonBundle is a JSON output when the lines are one JSON document, so a front
// end can show it as a tree; false when they are not.
func jsonBundle(lines []string) (MIMEBundle, bool) {
	text := strings.Join(lines, "\n")
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, false
	}
	return MIMEBundle{mimeText: text, mimeJSON: value}, true
}
