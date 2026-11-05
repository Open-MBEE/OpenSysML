package export

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Every conversion in the process shares one library index whose semantic
// model memoizes, so stubbing from several goroutines must not race (run under -race).
func TestLibraryIndexStubsConcurrently(t *testing.T) {
	base := libs.SharedBase()
	model := semantics.NewModel(resolve.New(base))
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	index := &libraryIndex{model: model, catalog: catalog, anonymous: identity.AnonymousMemberIndex(model, catalog)}
	names := []string{"Parts::Part", "Parts::parts", "Items::Item", "Actions::Action", "Occurrences::Occurrence::suboccurrences", "ScalarValues::Real", "Base::things", "Items::items"}
	var wg sync.WaitGroup
	for range 4 {
		for _, name := range names {
			element, ok := catalog.ElementNamed(name)
			if !ok {
				t.Fatalf("%s is not in the library catalog", name)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				graph := rdf.NewGraph()
				addLibraryStub(graph, index, rdf.ElementIRIForID(element.ID), element.ID)
				if !graph.HasProperty(rdf.ElementIRIForID(element.ID), rdf.SysML+pElementID) {
					t.Errorf("%s was not stubbed", name)
				}
			}()
		}
	}
	wg.Wait()
}
