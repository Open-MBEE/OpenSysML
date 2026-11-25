package export_test

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// Conversions share one library index; its semantic model memoizes, so reading
// several element forms at once must not race on it (run under -race).
func TestConcurrentAPIJSONReadsShareTheLibraryIndex(t *testing.T) {
	sources := []string{
		"package P {\n\tpart def V;\n\tpart car : V;\n}",
		"package Q {\n\taction def Step;\n\taction def A {\n\t\taction a : Step;\n\t\tfirst start then a;\n\t}\n}",
		"package R {\n\tattribute def Mass;\n\titem def Box {\n\t\tattribute mass : Mass;\n\t}\n}",
	}
	documents := make([][]byte, len(sources))
	for i, source := range sources {
		graph, err := convert.SysMLToRDF("m.sysml", []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		if documents[i], err = export.WriteAPIJSON(graph); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4*len(documents))
	for range 4 {
		for _, document := range documents {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := export.ReadAPIJSON(document); err != nil {
					errs <- err
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
