package docrender

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
)

// nestedDocument evaluates a document of depth nested sections, each holding
// one paragraph beside the next section.
func nestedDocument(b *testing.B, depth int) *docir.Document {
	b.Helper()
	var src strings.Builder
	src.WriteString("package Deep {\n\tprivate import DocumentQueries::*;\n\tpart def Report :> Document {\n\t\tattribute redefines title = \"Deep\";\n")
	for level := range depth {
		fmt.Fprintf(&src, "part s%d : Section {\nattribute redefines title = \"Level %d\";\npart p%d : Paragraph {\nattribute redefines text = \"Text of level %d.\";\n}\n", level, level, level, level)
	}
	src.WriteString(strings.Repeat("}\n", depth))
	src.WriteString("\t}\n}\n")
	path := filepath.Join(b.TempDir(), "deep.sysml")
	if err := os.WriteFile(path, []byte(src.String()), 0o644); err != nil {
		b.Fatalf("write fixture: %v", err)
	}
	return fixtureDocumentAt(b, path, "deep.sysml", "Deep::Report")
}

// BenchmarkHTMLNestedSections renders documents of growing section depth: a
// walk that copies each subtree once per ancestor grows with the square of it.
func BenchmarkHTMLNestedSections(b *testing.B) {
	for _, depth := range []int{8, 32, 128} {
		document := nestedDocument(b, depth)
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := HTML(document, HTMLOptions{}); err != nil {
					b.Fatalf("render: %v", err)
				}
			}
		})
	}
}
