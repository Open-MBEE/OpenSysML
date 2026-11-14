package export_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// elementGraphWritesTheTurtleGraphsElements holds a file's conversion to the element
// form to the array the graph the Turtle writer reads is written as.
func elementGraphWritesTheTurtleGraphsElements() corpusCheck {
	var countsMu sync.Mutex
	compared, refused := 0, 0
	return corpusCheck{
		name: "element_graph_writes_the_turtle_graphs_elements",
		check: func(t *testing.T, file corpusFile) {
			want, wantErr := file.conversions.apiJSON(file.path, file.source, "standard", "graph")
			got, gotErr := convert.Convert(file.path, file.source, convert.FormatSysML, convert.FormatAPIJSON)
			countsMu.Lock()
			defer countsMu.Unlock()
			if wantErr != nil || gotErr != nil {
				refused++
				if wantErr == nil || gotErr == nil || wantErr.Error() != gotErr.Error() {
					t.Fatalf("the element graph answers %v, the Turtle graph %v", gotErr, wantErr)
				}
				return
			}
			compared++
			if !bytes.Equal(got, want) {
				t.Errorf("the element graph writes another element array than the Turtle graph (%d bytes, want %d)", len(got), len(want))
			}
		},
		summarize: func(t *testing.T) {
			countsMu.Lock()
			defer countsMu.Unlock()
			t.Logf("element graph: compared %d models, %d refused alike", compared, refused)
			if compared == 0 {
				t.Error("no model was written from both graphs")
			}
		},
	}
}

// A model of several documents writes the same element array from the element graph as
// from the Turtle graph, whole or by selection, converted fresh or from a prepared model.
func TestAPIJSONElementGraphOfExampleModels(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"runtime-showcase", "qvts", "self-check-rules"} {
		t.Run(root, func(t *testing.T) {
			t.Parallel()
			paths, err := filepath.Glob(filepath.Join("..", "..", "examples", root, "*.sysml"))
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) < 2 {
				t.Fatalf("%d documents under %s: not a model of several", len(paths), root)
			}
			var documents []export.ModelDocument
			var inputs []convert.Input
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				file := source.New(filepath.Base(path), data)
				p := parser.New(file)
				tree := p.ParseFile()
				if len(p.Diagnostics) != 0 {
					t.Fatal(p.Diagnostics)
				}
				documents = append(documents, export.ModelDocument{File: file, Root: tree})
				inputs = append(inputs, convert.Input{Name: file.Name(), Data: data})
			}
			write := func(what string, toGraph func() (*rdf.Graph, error)) []byte {
				t.Helper()
				graph, err := toGraph()
				if err != nil {
					t.Fatalf("%s: %v", what, err)
				}
				out, err := export.WriteAPIJSON(graph)
				if err != nil {
					t.Fatalf("%s: %v", what, err)
				}
				return out
			}
			prepared := export.PrepareModel(documents)
			selections := [][]string{nil, {documents[0].File.Name()}, {documents[len(documents)-1].File.Name()}}
			for _, form := range []export.IDForm{export.IDQualifiedName, export.IDUUID} {
				for _, selection := range selections {
					want := write("prepared Turtle graph", func() (*rdf.Graph, error) { return prepared.ToRDF(selection, form) })
					got := write("prepared element graph", func() (*rdf.Graph, error) { return prepared.ElementGraph(selection, form) })
					if !bytes.Equal(got, want) {
						t.Errorf("prepared model, selection %v, id form %v: the element graph writes another element array", selection, form)
					}
					if selection != nil {
						continue
					}
					fresh := write("element graph", func() (*rdf.Graph, error) { return export.ModelElementGraphWith(documents, form) })
					if !bytes.Equal(fresh, want) {
						t.Errorf("fresh model, id form %v: the element graph writes another element array", form)
					}
				}
			}
			want := write("Turtle graph", func() (*rdf.Graph, error) { return convert.ModelGraph(inputs, convert.Options{}) })
			got, err := convert.ConvertModel(inputs, convert.FormatAPIJSON, convert.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Error("ConvertModel writes another element array than the Turtle graph")
			}
		})
	}
}
