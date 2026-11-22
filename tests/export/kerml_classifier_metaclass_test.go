package export_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// kermlClassifierDeclaration is `keyword name specializes general`, with the two ends
// an association or interaction relates.
func kermlClassifierDeclaration(keyword, name, general string) string {
	head := keyword + " " + name
	if general != "" {
		head += " specializes " + general
	}
	if strings.HasPrefix(keyword, "assoc") || keyword == "interaction" {
		return head + " { end feature a : C; end feature b : C; }"
	}
	return head + ";"
}

func kermlClassifierPackage(keyword string) string {
	general := ""
	if keyword == "type" {
		// A `type` declaration states a specialization (KerML.xtext TypeDeclaration).
		general = "C"
	}
	return "package P {\n\tclass C;\n\t" + kermlClassifierDeclaration(keyword, "A", general) + "\n\t" +
		kermlClassifierDeclaration(keyword, "X", "A") + "\n}\n"
}

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

// Each KerML classifier keyword writes its metaclass and a Subclassification at _sc0, in
// Turtle and API JSON alike, and comes back as written from the graph alone.
func TestKerMLClassifierDeclarationsWriteTheirMetaclass(t *testing.T) {
	for _, keyword := range ast.KerMLTypeKeywords() {
		metaclass := ast.KerMLTypeMetaclass(keyword)
		relationship, id := "Subclassification", "P__X_sc0"
		if keyword == "type" {
			// A Type is no Classifier, so its specializes is a plain Specialization.
			relationship, id = "Specialization", "P__X_sp0"
		}
		t.Run(keyword, func(t *testing.T) {
			src := []byte(kermlClassifierPackage(keyword))
			turtle, err := convert.Convert("k.kerml", src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			for _, want := range []string{"elmt:P__X\n    a sysml:" + metaclass + " ;", "elmt:" + id + "\n    a sysml:" + relationship + " ;"} {
				if !strings.Contains(string(turtle), want) {
					t.Errorf("`%s X specializes A` should write %q\n%s", keyword, want, turtle)
				}
			}
			if placeholder := regexp.MustCompile(`a sysml:(Class|Structure|Association|Behavior|Predicate|Interaction)Usage `).Find(turtle); placeholder != nil {
				t.Errorf("`%s` writes %q, which the metamodel does not declare", keyword, placeholder)
			}
			apiJSON, err := convert.Convert("k.kerml", src, convert.FormatSysML, convert.FormatAPIJSON)
			if err != nil {
				t.Fatalf("to api-json: %v", err)
			}
			types := apiJSONTypesByID(t, apiJSON)
			if types["P__X"] != metaclass || types[id] != relationship {
				t.Errorf("api-json types X as %q and %s as %q, want %s and %s", types["P__X"], id, types[id], metaclass, relationship)
			}
			back := backFromTheGraphAloneAs(t, "k.kerml", string(turtle))
			if want := strings.TrimSuffix(strings.SplitN(kermlClassifierDeclaration(keyword, "X", "A"), " {", 2)[0], ";"); !strings.Contains(back, want) {
				t.Errorf("the graph alone should write %q back\n%s", want, back)
			}
			if strings.Contains(back, " def ") {
				t.Errorf("a KerML declaration came back with SysML's `def`\n%s", back)
			}
		})
	}
}

// TestKerMLClassifierSubclassificationHasOneUUID: -id uuid hashes the default id, so the
// Subclassification of a KerML classifier has the UUID a SysML definition's has, on both forms.
func TestKerMLClassifierSubclassificationHasOneUUID(t *testing.T) {
	opts := convert.Options{ID: export.IDUUID}
	subclassification := func(name string, src string) string {
		t.Helper()
		apiJSON, err := convert.ConvertWith(name, []byte(src), convert.FormatSysML, convert.FormatAPIJSON, opts)
		if err != nil {
			t.Fatalf("%s to api-json: %v", name, err)
		}
		var ids []string
		for id, metaclass := range apiJSONTypesByID(t, apiJSON) {
			if metaclass == "Subclassification" {
				ids = append(ids, id)
			}
		}
		if len(ids) != 1 {
			t.Fatalf("%s writes Subclassifications %v, want one", name, ids)
		}
		turtle, err := convert.ConvertWith(name, []byte(src), convert.FormatSysML, convert.FormatTurtle, opts)
		if err != nil {
			t.Fatalf("%s to turtle: %v", name, err)
		}
		if !strings.Contains(string(turtle), "elmt:"+ids[0]+"\n    a sysml:Subclassification ;") {
			t.Fatalf("%s: turtle does not write the Subclassification at %s", name, ids[0])
		}
		return ids[0]
	}
	want := subclassification("p.sysml", "package P {\n\tpart def A;\n\tpart def X :> A;\n}\n")
	for _, keyword := range ast.KerMLTypeKeywords() {
		if keyword == "type" {
			continue
		}
		src := "package P {\n\t" + kermlClassifierDeclaration(keyword, "A", "") + "\n\t" + kermlClassifierDeclaration(keyword, "X", "A") + "\n\tclass C;\n}\n"
		if got := subclassification("k.kerml", src); got != want {
			t.Errorf("`%s X specializes A` has the Subclassification %s, `part def X :> A` has %s", keyword, got, want)
		}
	}
}

// A graph typed with the names earlier releases wrote (sysml:ClassUsage and its like, with a
// plain Specialization) reads back as the declaration, its API JSON the metaclass's.
func TestLegacyKerMLTypeNamesStillRead(t *testing.T) {
	legacyNames := map[string]string{
		"Class": "ClassUsage", "Structure": "StructureUsage", "Association": "AssociationUsage",
		"Behavior": "BehaviorUsage", "Predicate": "PredicateUsage", "Interaction": "InteractionUsage",
	}
	for _, keyword := range []string{"class", "struct", "assoc", "behavior", "predicate", "interaction"} {
		metaclass := ast.KerMLTypeMetaclass(keyword)
		t.Run(keyword, func(t *testing.T) {
			src := []byte("package P {\n\t" + kermlClassifierDeclaration(keyword, "A", "") + "\n\t" + kermlClassifierDeclaration(keyword, "X", "A") + "\n\tdatatype C;\n}\n")
			first, err := convert.Convert("k.kerml", src, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			legacy := withoutTriples(t, first, "sysx:sourceText")
			for _, property := range []string{"sysml:ownedSubclassification", "sysml:subclassifier", "sysml:superclassifier", "sysml:owningClassifier"} {
				legacy = withoutTriples(t, legacy, property)
			}
			legacy = []byte(strings.NewReplacer(
				"a sysml:"+metaclass+" ;", "a sysml:"+legacyNames[metaclass]+" ;",
				"a sysml:Subclassification ;", "a sysml:Specialization ;",
				"P__X_sc0", "P__X_sp0",
			).Replace(string(legacy)))
			back, err := convert.Convert("k.ttl", legacy, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if want := keyword + " X specializes A"; !strings.Contains(string(back), want) {
				t.Errorf("a legacy-typed %s should still read as %q\n%s", keyword, want, back)
			}
			second, err := convert.Convert("k.kerml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle again: %v", err)
			}
			requireSameGraphBytes(t, back, first, second)
			apiJSON, err := convert.Convert("k.ttl", legacy, convert.FormatTurtle, convert.FormatAPIJSON)
			if err != nil {
				t.Fatalf("legacy turtle to api-json: %v", err)
			}
			if got := apiJSONTypesByID(t, apiJSON)["P__X"]; got != metaclass {
				t.Errorf("a legacy-typed %s is the API JSON type %q, want %s", keyword, got, metaclass)
			}
		})
	}
}

// A Class is written def-less, as the KerML grammar has it; one written `class def X`
// records its `def`, so both forms come back as written from the graph alone.
func TestKerMLTypeDeclarationFormSurvivesTheGraph(t *testing.T) {
	for _, tc := range []struct{ name, declaration string }{
		{"k.kerml", "class X specializes A;"},
		{"k.sysml", "class X specializes A;"},
		{"k.sysml", "class def X specializes A;"},
		{"k.sysml", "struct def X specializes A;"},
		{"k.sysml", "classifier def X specializes A;"},
		{"k.kerml", "classifier X specializes A;"},
		{"k.kerml", "abstract assoc struct X specializes A;"},
	} {
		t.Run(tc.name+"/"+tc.declaration, func(t *testing.T) {
			src := "package P {\n\tclass A;\n\t" + tc.declaration + "\n}\n"
			turtle, _, err := convert.ConvertTolerant(tc.name, []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
			back, err := convert.Convert("k.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), tc.declaration) {
				t.Errorf("%q should come back as written from the graph alone\n%s", tc.declaration, back)
			}
		})
	}
}
