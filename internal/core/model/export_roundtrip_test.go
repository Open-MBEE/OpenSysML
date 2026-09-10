package model

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

const exportFixturesDir = "../export/testdata/convert"

// The RDF mapping's structural predicates alone must carry a model back to
// notation that analyses exactly as the original did: the source text is
// stripped between the hops, so a modifier the mapping drops shows up here as
// a diagnostic the original never had.
func TestNotationFromTheGraphAloneAnalysesLikeTheOriginal(t *testing.T) {
	for _, fixture := range []string{
		"individual_definitions.sysml",
	} {
		t.Run(strings.TrimSuffix(fixture, filepath.Ext(fixture)), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(exportFixturesDir, fixture))
			if err != nil {
				t.Fatal(err)
			}
			turtle, err := export.Convert(fixture, src, export.FormatSysML, export.FormatTurtle)
			if err != nil {
				t.Fatalf("notation -> Turtle: %v", err)
			}
			graph, err := rdf.ParseTurtle(turtle)
			if err != nil {
				t.Fatal(err)
			}
			structural := rdf.NewGraph()
			for _, tr := range graph.Triples() {
				if tr.Predicate != rdf.OpenSysMLTerm("sourceText") && tr.Predicate != rdf.OpenSysMLTerm("sourceTail") {
					structural.AddTriple(tr)
				}
			}
			if structural.Len() == graph.Len() {
				t.Fatal("no source text was recorded, so stripping it proves nothing")
			}
			back, err := export.Convert("back.ttl", rdf.WriteTurtle(structural), export.FormatTurtle, export.FormatSysML)
			if err != nil {
				t.Fatalf("Turtle -> notation: %v", err)
			}
			want, got := analysisFindings(fixture, src), analysisFindings(fixture, back)
			if strings.Join(want, "\n") != strings.Join(got, "\n") {
				t.Errorf("the notation written from the graph alone analyses differently\n--- original ---\n%s\n--- from the graph ---\n%s\n--- notation ---\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"), back)
			}
		})
	}
}

// analysisFindings analyses one document and returns its findings, sorted so
// two analyses compare by what they report rather than where.
func analysisFindings(name string, content []byte) []string {
	ws := NewWorkspace()
	ws.Open(name, content, 1)
	var messages []string
	for _, d := range ws.Diagnostics(name) {
		messages = append(messages, d.Severity.String()+": "+d.Message)
	}
	sort.Strings(messages)
	return messages
}
