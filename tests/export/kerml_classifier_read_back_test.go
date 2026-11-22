package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// apiJSONTypesByID reads the "@type" of every element of an API JSON document by its "@id".
func apiJSONTypesByID(t *testing.T, document []byte) map[string]string {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	types := map[string]string{}
	for _, element := range elements {
		id, _ := element["@id"].(string)
		types[id], _ = element["@type"].(string)
	}
	return types
}

// graphAlone drops what lets a graph of ours come back verbatim, and with it the
// keyword it records when bare is set, leaving the metaclass to say the declaration.
func graphAlone(t *testing.T, turtle []byte, bare bool) []byte {
	t.Helper()
	properties := []string{"sysx:sourceText", "sysx:sourceTail"}
	if bare {
		properties = append(properties, "sysx:declaredKeyword")
	}
	for _, property := range properties {
		turtle = withoutTriples(t, turtle, property)
	}
	return turtle
}

// The `def` form and the def-less form of a KerML type share a metaclass; each comes back
// as written from the graph alone, through Turtle and through API JSON.
func TestKerMLTypeDeclarationFormSurvivesTheGraph(t *testing.T) {
	for _, tc := range []struct{ name, declaration string }{
		{"k.kerml", "class X specializes A;"},
		{"k.sysml", "class X specializes A;"},
		{"k.sysml", "class def X specializes A;"},
		{"k.kerml", "class def X specializes A;"},
		{"k.sysml", "struct def X specializes A;"},
		{"k.sysml", "assoc def X specializes A;"},
		{"k.sysml", "behavior def X specializes A;"},
		{"k.sysml", "predicate def X specializes A;"},
		{"k.sysml", "classifier def X specializes A;"},
		{"k.kerml", "classifier X specializes A;"},
		{"k.sysml", "abstract class def X specializes A;"},
		{"k.kerml", "abstract assoc struct X specializes A;"},
	} {
		t.Run(tc.name+"/"+tc.declaration, func(t *testing.T) {
			src := "package P {\n\tclass A;\n\t" + tc.declaration + "\n}\n"
			turtle, _, err := convert.ConvertTolerant(tc.name, []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stripped := graphAlone(t, turtle, false)
			back, err := convert.Convert("k.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), "\t"+tc.declaration+"\n") && !strings.Contains(string(back), "    "+tc.declaration+"\n") {
				t.Errorf("%q should come back as written from the graph alone\n%s", tc.declaration, back)
			}
			apiJSON, err := convert.Convert("k.ttl", stripped, convert.FormatTurtle, convert.FormatAPIJSON)
			if err != nil {
				t.Fatalf("to api-json: %v", err)
			}
			fromJSON, err := convert.Convert("k.json", apiJSON, convert.FormatAPIJSON, convert.FormatSysML)
			if err != nil {
				t.Fatalf("api-json back to notation: %v", err)
			}
			if string(fromJSON) != string(back) {
				t.Errorf("API JSON reads back\n%s\nwhere Turtle reads back\n%s", fromJSON, back)
			}
			second, _, err := convert.ConvertTolerant(tc.name, back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			if got := graphAlone(t, second, false); string(got) != string(stripped) {
				t.Errorf("the second hop changed the graph:\n--- first ---\n%s--- second ---\n%s", stripped, got)
			}
		})
	}
}

// A graph stating no sysx:declaredKeyword, as one another tool wrote, reads every KerML type
// as the keyword its metaclass states, so converting it again writes the same metaclasses.
func TestKerMLTypeKeywordIsReadFromTheMetaclass(t *testing.T) {
	src := "package P {\n\tclass C;\n\ttype T specializes C;\n\tassoc struct S {\n\t\tend feature a : C;\n\t\tend feature b : C;\n\t}\n\tdatatype D;\n\tdatatype DX specializes D;\n\tfunction F;\n\tfunction FX specializes F;\n}\n"
	first, err := convert.Convert("k.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	bare := graphAlone(t, first, true)
	apiJSON, err := convert.Convert("k.ttl", bare, convert.FormatTurtle, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	for form, convertBack := range map[string]func() ([]byte, error){
		"turtle": func() ([]byte, error) {
			return convert.Convert("k.ttl", bare, convert.FormatTurtle, convert.FormatSysML)
		},
		"api-json": func() ([]byte, error) {
			return convert.Convert("k.json", apiJSON, convert.FormatAPIJSON, convert.FormatSysML)
		},
	} {
		t.Run(form, func(t *testing.T) {
			back, err := convertBack()
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			for _, want := range []string{"type T specializes C", "assoc struct S", "datatype D;", "datatype DX specializes D;", "function F;", "function FX specializes F;"} {
				if !strings.Contains(string(back), want) {
					t.Errorf("the graph without its keywords should read %q\n%s", want, back)
				}
			}
			second, err := convert.Convert("k.kerml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			for _, want := range []string{
				"elmt:P__T\n    a sysml:Type ;", "elmt:P__S\n    a sysml:AssociationStructure ;",
				"elmt:P__DX\n    a sysml:DataType ;", "elmt:P__FX\n    a sysml:Function ;",
				"elmt:P__DX_sc0\n    a sysml:Subclassification ;", "elmt:P__FX_sc0\n    a sysml:Subclassification ;",
			} {
				if !strings.Contains(string(second), want) {
					t.Errorf("converting the notation read back should write %q\n%s", want, second)
				}
			}
		})
	}
}

// legacyKerMLTypeNames are the names earlier releases typed a KerML type declaration with.
var legacyKerMLTypeNames = []struct{ keyword, metaclass, legacy string }{
	{"type", "Type", "ClassUsage"},
	{"class", "Class", "ClassUsage"},
	{"struct", "Structure", "StructureUsage"},
	{"assoc", "Association", "AssociationUsage"},
	{"assoc struct", "AssociationStructure", "AssociationUsage"},
	{"behavior", "Behavior", "BehaviorUsage"},
	{"predicate", "Predicate", "PredicateUsage"},
	{"interaction", "Interaction", "InteractionUsage"},
}

// A graph typed with the names earlier releases wrote (sysml:ClassUsage and its like, with a
// plain Specialization) is the declaration's metaclass in API JSON, which reads back as it.
func TestLegacyKerMLTypeNamesAreTheirMetaclassInAPIJSON(t *testing.T) {
	for _, tc := range legacyKerMLTypeNames {
		for _, recorded := range []bool{true, false} {
			if !recorded && (tc.keyword == "type" || tc.keyword == "assoc struct") {
				// Only the keyword an earlier release recorded tells these from `class` and `assoc`.
				continue
			}
			name := tc.keyword + "/keyword recorded"
			if !recorded {
				name = tc.keyword + "/no keyword"
			}
			t.Run(name, func(t *testing.T) {
				declare := func(name, general string) string {
					head := tc.keyword + " " + name + " specializes " + general
					if strings.HasPrefix(tc.keyword, "assoc") || tc.keyword == "interaction" {
						return head + " { end feature a : C; end feature b : C; }"
					}
					return head + ";"
				}
				src := "package P {\n\tclass C;\n\t" + declare("A", "C") + "\n\t" + declare("X", "A") + "\n}\n"
				first, err := convert.Convert("k.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
				if err != nil {
					t.Fatalf("to turtle: %v", err)
				}
				legacy := graphAlone(t, first, !recorded)
				for _, property := range []string{"sysml:ownedSubclassification", "sysml:subclassifier", "sysml:superclassifier", "sysml:owningClassifier"} {
					legacy = withoutTriples(t, legacy, property)
				}
				text := strings.ReplaceAll(string(legacy), "elmt:P__C\n    a sysml:Class ;", "elmt:P__C\n    a sysml:KEPT ;")
				text = strings.NewReplacer(
					"a sysml:"+tc.metaclass+" ;", "a sysml:"+tc.legacy+" ;",
					"a sysml:Subclassification ;", "a sysml:Specialization ;",
					"_sc0", "_sp0",
				).Replace(text)
				legacy = []byte(strings.ReplaceAll(text, "a sysml:KEPT ;", "a sysml:Class ;"))
				if !strings.Contains(string(legacy), "elmt:P__X\n    a sysml:"+tc.legacy+" ;") {
					t.Fatalf("the fixture does not type X as %s\n%s", tc.legacy, legacy)
				}
				apiJSON, err := convert.Convert("k.ttl", legacy, convert.FormatTurtle, convert.FormatAPIJSON)
				if err != nil {
					t.Fatalf("legacy turtle to api-json: %v", err)
				}
				types := apiJSONTypesByID(t, apiJSON)
				for _, id := range []string{"P__A", "P__X"} {
					if types[id] != tc.metaclass {
						t.Errorf("a legacy-typed `%s` %s is the API JSON type %q, want %s", tc.keyword, id, types[id], tc.metaclass)
					}
				}
				if strings.Contains(string(apiJSON), `"`+tc.legacy+`"`) {
					t.Errorf("the API JSON still names %s, which the metamodel does not declare", tc.legacy)
				}
				fromJSON, err := convert.Convert("k.json", apiJSON, convert.FormatAPIJSON, convert.FormatSysML)
				if err != nil {
					t.Fatalf("api-json back to notation: %v", err)
				}
				if want := tc.keyword + " X specializes A"; !strings.Contains(string(fromJSON), want) {
					t.Errorf("a legacy-typed `%s` should read as %q through API JSON\n%s", tc.keyword, want, fromJSON)
				}
			})
		}
	}
}
