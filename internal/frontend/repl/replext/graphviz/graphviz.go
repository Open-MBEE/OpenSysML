// Package graphviz registers the drawer %render-document draws DOT diagrams
// with: the PDF backend's Graphviz, so the Markdown it prints carries the SVG
// a render on the command line writes.
package graphviz

import (
	"github.com/Open-MBEE/OpenSysML/internal/doc/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
)

func init() { replext.RegisterDrawer(docpdf.Graphviz{}) }
