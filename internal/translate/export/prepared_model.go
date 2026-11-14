package export

import (
	"fmt"
	"slices"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// PreparedModel serializes shared semantic preparation over immutable documents.
// Encoders and identity tables are request-local; discard it when replacing the model.
type PreparedModel struct {
	mu        sync.Mutex
	documents []ModelDocument
	res       *resolve.Resolver
	model     *semantics.Model
}

// PrepareModel records an immutable document set. Semantic preparation is lazy.
func PrepareModel(documents []ModelDocument) *PreparedModel {
	docs := slices.Clone(documents)
	for i := range docs {
		docs[i].Referenced = false
	}
	return &PreparedModel{documents: docs}
}

// ToRDF writes selected documents, or all when selection is empty, into a fresh graph.
// Unselected declarations still resolve references and participate in identity checks.
func (p *PreparedModel) ToRDF(selection []string, form IDForm) (*rdf.Graph, error) {
	return p.graph(selection, form, nameInheritedLibraryMembers)
}

// ElementGraph is ToRDF for a graph only the element form is written from, as
// ModelElementGraphWith is ModelToRDFWith.
func (p *PreparedModel) ElementGraph(selection []string, form IDForm) (*rdf.Graph, error) {
	return p.graph(selection, form, addLinkedLibraryStubs)
}

func (p *PreparedModel) graph(selection []string, form IDForm, nameLibrary func(*rdf.Graph)) (*rdf.Graph, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	documents := slices.Clone(p.documents)
	for _, name := range selection {
		if !slices.ContainsFunc(documents, func(doc ModelDocument) bool { return doc.File.Name() == name }) {
			return nil, fmt.Errorf("model does not contain document %q", name)
		}
	}
	for i := range documents {
		documents[i].Referenced = len(selection) > 0 && !slices.Contains(selection, documents[i].File.Name())
	}
	if p.res == nil {
		p.res, p.model = analyzeModel(p.documents)
	}
	res, encoders, err := modelEncodersOver(documents, form, p.res, p.model)
	if err != nil {
		return nil, err
	}
	graph, _, _, err := encodeModelDocuments(documents, res, encoders, nameLibrary)
	return graph, err
}
