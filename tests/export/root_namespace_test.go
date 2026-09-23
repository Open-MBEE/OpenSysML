package export_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const rootNamespaceModel = `package P {
    part def A;
    part a : A;
}
`

const rootNamespaceTwoRoots = `package P {
    part def A;
}
library package Q {
    part def B;
}
`

func rootElements(t *testing.T, document []byte) []map[string]json.RawMessage {
	t.Helper()
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("not a JSON array: %v\n%s", err, document)
	}
	return elements
}

func rootString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("not a string: %s", raw)
	}
	return s
}

func rootRef(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var ref struct {
		ID string `json:"@id"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatalf("not a reference: %s", raw)
	}
	return ref.ID
}

func rootRefs(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var refs []struct {
		ID string `json:"@id"`
	}
	if err := json.Unmarshal(raw, &refs); err != nil {
		t.Fatalf("not a reference array: %s", raw)
	}
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = ref.ID
	}
	return out
}

// The element form opens with the pilot's document wrapper: an unnamed,
// unowned Namespace whose OwningMembership owns the top-level package, the
// package pointing back through owner/owningRelationship/owningMembership.
func TestAPIJSONRootNamespaceShape(t *testing.T) {
	for _, form := range []struct {
		name string
		id   export.IDForm
		ns   string
		om   string
		pkg  string
	}{
		{"qualified", export.IDQualifiedName, "P_ns", "P_om", "P"},
		{"uuid", export.IDUUID,
			identity.DerivedID(identity.NamespaceOf(rdf.ElementIRIForID("P").Value), "P_ns"),
			identity.DerivedID(identity.NamespaceOf(rdf.ElementIRIForID("P").Value), "P_om"),
			identity.NamespaceOf(rdf.ElementIRIForID("P").Value)},
	} {
		t.Run(form.name, func(t *testing.T) {
			out, err := convert.ConvertWith("p.sysml", []byte(rootNamespaceModel), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{ID: form.id})
			if err != nil {
				t.Fatal(err)
			}
			elements := rootElements(t, out)
			first := elements[0]
			if rootString(t, first["@type"]) != "Namespace" || rootString(t, first["@id"]) != form.ns {
				t.Fatalf("the document does not open with the root Namespace %s:\n%s", form.ns, out)
			}
			for _, key := range []string{"owner", "owningRelationship", "declaredName", "declaredShortName"} {
				if _, ok := first[key]; ok {
					t.Errorf("the root Namespace states %s", key)
				}
			}
			if got := rootRefs(t, first["ownedRelationship"]); len(got) != 1 || got[0] != form.om {
				t.Errorf("root ownedRelationship = %v, want [%s]", got, form.om)
			}
			membership := apiJSONElement(t, elements, form.om)
			if rootString(t, membership["@type"]) != "OwningMembership" {
				t.Errorf("membership @type = %s", membership["@type"])
			}
			if rootRef(t, membership["owner"]) != form.ns || rootRef(t, membership["memberElement"]) != form.pkg ||
				rootRef(t, membership["ownedMemberElement"]) != form.pkg ||
				rootRef(t, membership["membershipOwningNamespace"]) != form.ns {
				t.Errorf("membership ends are wrong:\n%s", membership)
			}
			if got := rootRefs(t, membership["ownedRelatedElement"]); len(got) != 1 || got[0] != form.pkg {
				t.Errorf("membership ownedRelatedElement = %v", got)
			}
			pkg := apiJSONElement(t, elements, form.pkg)
			if rootRef(t, pkg["owner"]) != form.ns || rootRef(t, pkg["owningNamespace"]) != form.ns ||
				rootRef(t, pkg["owningRelationship"]) != form.om || rootRef(t, pkg["owningMembership"]) != form.om {
				t.Errorf("the package does not state its root ownership:\n%s", pkg)
			}
			if form.id == export.IDUUID {
				uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
				for _, element := range elements {
					if id := rootString(t, element["@id"]); !uuid.MatchString(id) {
						t.Errorf("id %q is not a uuid", id)
					}
				}
			}
		})
	}
}

// Two top-level members share one root Namespace, one OwningMembership each,
// in document order.
func TestAPIJSONRootNamespaceOwnsEveryRoot(t *testing.T) {
	out, err := convert.Convert("pq.sysml", []byte(rootNamespaceTwoRoots), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	elements := rootElements(t, out)
	namespaces := 0
	for _, element := range elements {
		if rootString(t, element["@type"]) == "Namespace" {
			namespaces++
		}
	}
	if namespaces != 1 {
		t.Fatalf("%d root Namespaces, want 1", namespaces)
	}
	if got := rootRefs(t, elements[0]["ownedRelationship"]); strings.Join(got, ",") != "P_om,Q_om" {
		t.Errorf("root ownedRelationship = %v", got)
	}
	if got := rootRefs(t, elements[0]["ownedMember"]); strings.Join(got, ",") != "P,Q" {
		t.Errorf("root ownedMember = %v", got)
	}
	for _, id := range []string{"P", "Q"} {
		if rootRef(t, apiJSONElement(t, elements, id)["owner"]) != "P_ns" {
			t.Errorf("%s is not owned by the root", id)
		}
	}
}

// The wrapper is transparent: notation round trips byte for byte in both id
// forms, reading the element form yields the same graph Turtle carries (no
// root Namespace), and writing that graph back reproduces the wrapper.
func TestAPIJSONRootNamespaceTransparent(t *testing.T) {
	for _, model := range []string{rootNamespaceModel, rootNamespaceTwoRoots} {
		for _, id := range []export.IDForm{export.IDQualifiedName, export.IDUUID} {
			document, err := convert.ConvertWith("m.sysml", []byte(model), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{ID: id})
			if err != nil {
				t.Fatal(err)
			}
			back, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
			if err != nil {
				t.Fatal(err)
			}
			if string(back) != model {
				t.Errorf("id form %v: notation moved:\n%s", id, back)
			}
			turtle, err := convert.ConvertWith("m.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle, convert.Options{ID: id})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(turtle), "sysml:Namespace") {
				t.Errorf("Turtle carries the root Namespace:\n%s", turtle)
			}
			fromTurtle, err := rdf.ParseTurtle(turtle)
			if err != nil {
				t.Fatal(err)
			}
			fromJSON, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatal(err)
			}
			if !sameTriples(tripleSet(fromTurtle), tripleSet(fromJSON)) {
				t.Errorf("id form %v: the element form read differs from Turtle:\n%v", id, diffTriples(tripleSet(fromJSON), tripleSet(fromTurtle)))
			}
			again, err := export.WriteAPIJSON(fromJSON)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != string(document) {
				t.Errorf("id form %v: the element form is not idempotent", id)
			}
		}
	}
}

// A document that already carries the pilot's wrapper is not wrapped twice.
func TestAPIJSONRootNamespaceNotDoubled(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "interchange", "p10.toolkit.compact.json"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := export.ReadAPIJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range graph.Subjects() {
		if graph.Type(subject) == rdf.SysML+"Namespace" {
			t.Fatalf("ReadAPIJSON kept the root Namespace <%s>", subject.Value)
		}
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	namespaces := 0
	for _, element := range rootElements(t, out) {
		if rootString(t, element["@type"]) == "Namespace" {
			namespaces++
		}
	}
	if namespaces != 1 {
		t.Fatalf("%d root Namespaces, want 1", namespaces)
	}
}

const rootNamespaceSuffixNames = `package P {
    part def A;
}
package P_ns {
    part def B;
}
package P_om {
    part def C;
}
`

// Top-level names spelled like the wrapper's suffixes stay apart from the
// minted root ids in both id forms: every @id is unique and every element
// keeps a single @type.
func TestAPIJSONRootNamespaceIdsDoNotCollideWithNames(t *testing.T) {
	for _, id := range []export.IDForm{export.IDQualifiedName, export.IDUUID} {
		document, err := convert.ConvertWith("m.sysml", []byte(rootNamespaceSuffixNames), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{ID: id})
		if err != nil {
			t.Fatal(err)
		}
		assertDistinctRootIds(t, document, 3)
		back, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
		if err != nil {
			t.Fatal(err)
		}
		if string(back) != rootNamespaceSuffixNames {
			t.Errorf("id form %v: notation moved:\n%s", id, back)
		}
	}
}

// A foreign document may spell its ids freely: roots whose ids already read
// `P`, `P_ns` and `P_om` are wrapped without merging any subject, and the
// minted ids are the same on every run.
func TestAPIJSONRootNamespaceIdsDoNotCollideWithForeignIds(t *testing.T) {
	foreign := []byte(`[
{"@type":"Package","@id":"P","declaredName":"P"},
{"@type":"Package","@id":"P_ns","declaredName":"Q"},
{"@type":"Package","@id":"P_om","declaredName":"R"},
{"@type":"Package","@id":"P_om_om","declaredName":"S"}
]`)
	graph, err := export.ReadAPIJSON(foreign)
	if err != nil {
		t.Fatal(err)
	}
	out, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := assertDistinctRootIds(t, out, 4)
	for _, id := range []string{"P", "P_ns", "P_om", "P_om_om"} {
		if got := rootString(t, apiJSONElement(t, elements, id)["@type"]); got != "Package" {
			t.Errorf("%s became a %s", id, got)
		}
	}
	if got := rootString(t, elements[0]["@id"]); got != "P_ns_ns" {
		t.Errorf("root Namespace id = %s, want P_ns_ns", got)
	}
	again, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(out) {
		t.Error("minted ids differ between runs")
	}
}

// assertDistinctRootIds checks one Namespace, roots OwningMemberships, no
// repeated @id, and one @type string per element.
func assertDistinctRootIds(t *testing.T, document []byte, roots int) []map[string]json.RawMessage {
	t.Helper()
	elements := rootElements(t, document)
	seen := map[string]bool{}
	namespaces, memberships := 0, 0
	for _, element := range elements {
		id := rootString(t, element["@id"])
		if seen[id] {
			t.Errorf("@id %q appears twice:\n%s", id, document)
		}
		seen[id] = true
		switch rootString(t, element["@type"]) {
		case "Namespace":
			namespaces++
		case "OwningMembership":
			if _, top := element["owningRelatedElement"]; top {
				if rootRef(t, element["owningRelatedElement"]) == rootString(t, elements[0]["@id"]) {
					memberships++
				}
			}
		}
	}
	if namespaces != 1 || memberships != roots {
		t.Errorf("%d root Namespaces and %d root memberships, want 1 and %d:\n%s", namespaces, memberships, roots, document)
	}
	return elements
}
