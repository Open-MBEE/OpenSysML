// Package elementids gives queries the elementId a conversion of the model
// writes for each element, read from the RDF export; a binary links it to
// fill the column, and one that does not reports elementId absent.
package elementids

import (
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func init() { queryexec.RegisterElementIDSource(open) }

// open reads, from one conversion of docs made on the first read, the
// elementId each element is written with.
func open(docs []queryexec.ElementIDDocument) queryexec.ElementIDReader {
	documents := make([]export.ElementIDDocument, 0, len(docs))
	for _, doc := range docs {
		documents = append(documents, export.ElementIDDocument{File: doc.File, Root: doc.Root, ParseDiags: doc.ParseDiags})
	}
	return export.NewElementIDReader(documents)
}
