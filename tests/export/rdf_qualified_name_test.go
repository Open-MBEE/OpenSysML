package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// sysml:qualifiedName has the escapedName() spelling (KerML §8.3.2.1.2), the address
// the IRI encodes is stated beside it, and the graph reads back from either.
func TestQualifiedNameIsEscapedBesideTheAddress(t *testing.T) {
	tests := []struct {
		written   string // the name as the notation writes it
		name      string // the declared name
		qualified string
		address   string
	}{
		{`'two words'`, "two words", `Observatory::Holder::'two words'`, `Observatory::Holder::two words`},
		{`'it\'s'`, "it's", `Observatory::Holder::'it\'s'`, `Observatory::Holder::it's`},
		{`'x::y'`, "x::y", `Observatory::Holder::'x::y'`, `Observatory::Holder::'x::y'`},
		{`'@1'`, "@1", `Observatory::Holder::'@1'`, `Observatory::Holder::'@1'`},
		{`'\'lead'`, "'lead", `Observatory::Holder::'\'lead'`, `Observatory::Holder::'\'lead'`},
		{`plain`, "plain", `Observatory::Holder::plain`, `Observatory::Holder::plain`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := "package Observatory {\n    part def Holder {\n        part " + test.written + " {\n            attribute inner;\n        }\n        part user subsets " + test.written + ";\n    }\n}\n"
			turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			graph, err := rdf.ParseTurtle(turtle)
			if err != nil {
				t.Fatalf("parse turtle: %v", err)
			}
			subject := rdf.IRI(rdf.Element + rdf.EncodeElementID(test.address))
			if name, _ := graph.Lexical(subject, rdf.SysML+"declaredName"); name != test.name {
				t.Fatalf("the IRI encoding %q declares %q, want %q\n%s", test.address, name, test.name, turtle)
			}
			if got, _ := graph.Lexical(subject, rdf.SysML+"qualifiedName"); got != test.qualified {
				t.Errorf("sysml:qualifiedName = %q, want %q", got, test.qualified)
			}
			mapping, stated := graph.Lexical(subject, rdf.OpenSysML+"mappingQualifiedName")
			if stated && mapping != test.address {
				t.Errorf("sysx:mappingQualifiedName = %q, want %q", mapping, test.address)
			}
			if !stated && test.qualified != test.address {
				t.Errorf("the address %q is not stated beside sysml:qualifiedName %q", test.address, test.qualified)
			}
			inner := rdf.IRI(rdf.Element + rdf.EncodeElementID(test.address+"::inner"))
			if got, _ := graph.Lexical(inner, rdf.SysML+"qualifiedName"); got != test.qualified+"::inner" {
				t.Errorf("owned sysml:qualifiedName = %q, want %q", got, test.qualified+"::inner")
			}

			// The graph alone, without its source text, carries the names both ways.
			structural := withoutTriples(t, turtle, "sysx:sourceText")
			back, err := convert.Convert("m.ttl", structural, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			for _, line := range []string{"part " + test.written + " {", "part user subsets " + test.written + ";"} {
				if !strings.Contains(string(back), line) {
					t.Fatalf("%q did not come back:\n%s", line, back)
				}
			}
			again, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if string(again) != string(turtle) {
				t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
			}

			// A graph stating the qualified name alone reads the address out of it.
			alone := withoutProperty(t, structural, "sysx:mappingQualifiedName")
			if got, err := convert.Convert("alone.ttl", alone, convert.FormatTurtle, convert.FormatSysML); err != nil {
				t.Errorf("qualifiedName alone refused: %v", err)
			} else if string(got) != string(back) {
				t.Errorf("qualifiedName alone reads differently\n--- want ---\n%s\n--- got ---\n%s", back, got)
			}

			// An earlier release stated the address itself as sysml:qualifiedName.
			var legacy []string
			for _, block := range strings.Split(string(structural), "\n\n") {
				var kept []string
				address := ""
				for _, line := range strings.Split(block, "\n") {
					if value, ok := strings.CutPrefix(strings.TrimSpace(line), "sysx:mappingQualifiedName "); ok {
						address = strings.TrimSuffix(value, " ;")
						continue
					}
					kept = append(kept, line)
				}
				for i, line := range kept {
					if address != "" && strings.HasPrefix(strings.TrimSpace(line), "sysml:qualifiedName ") {
						kept[i] = "    sysml:qualifiedName " + address + " ;"
					}
				}
				legacy = append(legacy, strings.Join(kept, "\n"))
			}
			if got, err := convert.Convert("legacy.ttl", []byte(strings.Join(legacy, "\n\n")), convert.FormatTurtle, convert.FormatSysML); err != nil {
				t.Errorf("earlier-release graph refused: %v", err)
			} else if string(got) != string(back) {
				t.Errorf("earlier-release graph reads differently\n--- want ---\n%s\n--- got ---\n%s", back, got)
			}

			// api-json states the same qualifiedName and gives the same graph back.
			document, err := export.WriteAPIJSON(graph)
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatalf("decode API JSON: %v", err)
			}
			id, _ := rdf.SubjectID(subject)
			found := false
			for _, element := range elements {
				var elementID, qualified string
				if json.Unmarshal(element["@id"], &elementID) != nil || elementID != id {
					continue
				}
				found = true
				if err := json.Unmarshal(element["qualifiedName"], &qualified); err != nil || qualified != test.qualified {
					t.Errorf("api-json qualifiedName = %q (%v), want %q", qualified, err, test.qualified)
				}
			}
			if !found {
				t.Errorf("api-json has no element %q", id)
			}
			imported, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v", err)
			}
			if got := string(rdf.WriteTurtle(imported)); got != string(rdf.WriteTurtle(graph)) {
				t.Errorf("api-json changed the graph\n--- want ---\n%s\n--- got ---\n%s", rdf.WriteTurtle(graph), got)
			}
		})
	}
}
