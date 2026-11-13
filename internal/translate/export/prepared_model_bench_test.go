package export

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func BenchmarkRepeatedModelExport(b *testing.B) {
	var documents []ModelDocument
	for i := range 24 {
		var text strings.Builder
		fmt.Fprintf(&text, "package Generated%d { part def Device {", i)
		for j := range 24 {
			fmt.Fprintf(&text, "attribute field%d;", j)
		}
		text.WriteString("} part unit : Device; }")
		file := source.New(fmt.Sprintf("generated%d.sysml", i), []byte(text.String()))
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			b.Fatal(p.Diagnostics)
		}
		documents = append(documents, ModelDocument{File: file, Root: root, Referenced: i >= 4})
	}
	selection := []string{"generated0.sysml", "generated1.sysml", "generated2.sysml", "generated3.sysml"}
	b.Run("fresh", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := ModelToRDFWith(documents, IDQualifiedName); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("prepared", func(b *testing.B) {
		prepared := PrepareModel(documents)
		if _, err := prepared.ToRDF(selection, IDQualifiedName); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			if _, err := prepared.ToRDF(selection, IDQualifiedName); err != nil {
				b.Fatal(err)
			}
		}
	})
}
