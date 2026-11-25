package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// projectionSource mixes facet, comment and plain declarations for projection tests.
const projectionSource = `package P {
  private import ScalarValues::*;
  part def Plain { attribute x : Real; }
  part def Faceted { attribute y : Real; }
  part def Commented { attribute z : Real; }
  metadata def Facet;
  comment about Commented /* a note */
  metadata facet : Facet about Faceted { note = "a facet"; }
}
`

func projectionGraph(t *testing.T) *rdf.Graph {
	t.Helper()
	return graphOfNotation(t, projectionSource)
}

func rewrittenProjectionGraph(t *testing.T, graph *rdf.Graph, rewrite func(*rdf.Triple) bool) *rdf.Graph {
	t.Helper()
	out := rdf.NewGraphOf(append([]rdf.Triple(nil), graph.Triples()...), graph.Prefixes)
	out.RewriteTriples(rewrite)
	return out
}

func elementByID(t *testing.T, elements []json.RawMessage, id string) json.RawMessage {
	t.Helper()
	for _, element := range elements {
		if elementField(t, element, "@id") == id {
			return element
		}
	}
	t.Fatalf("no element with @id %s", id)
	return nil
}

func projectionWhole(t *testing.T, graph *rdf.Graph) []json.RawMessage {
	t.Helper()
	out, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	return splitAPIJSONElements(t, out)
}

func splitAPIJSONElements(t *testing.T, out []byte) []json.RawMessage {
	t.Helper()
	var elements []json.RawMessage
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatalf("api-json is not an array: %v\n%s", err, out)
	}
	return elements
}

func elementField(t *testing.T, raw json.RawMessage, key string) string {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("element is not an object: %v", err)
	}
	value, ok := object[key]
	if !ok {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}
	return strings.TrimSpace(string(value))
}

func assertProjectedAreByteIdentical(t *testing.T, whole, projected []json.RawMessage) {
	t.Helper()
	byID := map[string]json.RawMessage{}
	for _, element := range whole {
		byID[elementField(t, element, "@id")] = element
	}
	if len(byID) != len(whole) {
		t.Fatalf("the whole corpus has duplicate @ids")
	}
	for _, element := range projected {
		id := elementField(t, element, "@id")
		original, ok := byID[id]
		if !ok {
			t.Errorf("projection emitted %s, which the whole corpus does not hold", id)
			continue
		}
		if !bytes.Equal(original, element) {
			t.Errorf("%s is not byte-identical:\nwhole: %s\nprojected: %s", id, original, element)
		}
	}
}

// A legacy KerML type name is written as its normalized metaclass; the
// selector must match the @type the reply carries, not the graph's raw name.
func TestAPIJSONProjectionMatchesWrittenMetaclass(t *testing.T) {
	graph := rewrittenProjectionGraph(t, projectionGraph(t), func(tr *rdf.Triple) bool {
		if tr.Subject.Value == rdf.Element+"P__Plain" && tr.Predicate.Value == rdf.RDFType {
			tr.Object.Value = rdf.SysML + "StructureUsage"
		}
		return true
	})
	whole := projectionWhole(t, graph)
	if got := elementField(t, elementByID(t, whole, "P__Plain"), "@type"); got != "Structure" {
		t.Fatalf("P__Plain is written with @type %q, want Structure", got)
	}
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{Metaclasses: []string{"Structure"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	if len(projected) != 1 || elementField(t, projected[0], "@id") != "P__Plain" {
		t.Fatalf("projection by written metaclass wrote %s", out)
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

// The writer derives qualifiedName when the graph does not state it; the
// selector must match the derived name the reply carries.
func TestAPIJSONProjectionMatchesDerivedQualifiedName(t *testing.T) {
	graph := rewrittenProjectionGraph(t, projectionGraph(t), func(tr *rdf.Triple) bool {
		return tr.Predicate.Value != rdf.SysML+"qualifiedName"
	})
	whole := projectionWhole(t, graph)
	want := elementField(t, elementByID(t, whole, "P__Plain"), "qualifiedName")
	if want != "P::Plain" {
		t.Fatalf("P__Plain is written as %q, want P::Plain", want)
	}
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{QualifiedNames: []string{want}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	if len(projected) != 1 || elementField(t, projected[0], "@id") != "P__Plain" {
		t.Fatalf("projection by derived qualified name wrote %s", out)
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

func TestAPIJSONProjectionByQualifiedName(t *testing.T) {
	graph := projectionGraph(t)
	whole := projectionWhole(t, graph)
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{QualifiedNames: []string{"P::Faceted"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	if len(projected) != 1 {
		t.Fatalf("projection by qualified name emitted %d elements, want 1:\n%s", len(projected), out)
	}
	if got := elementField(t, projected[0], "qualifiedName"); got != "P::Faceted" {
		t.Errorf("projected element has qualifiedName %q, want P::Faceted", got)
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

func TestAPIJSONProjectionByMetaclass(t *testing.T) {
	graph := projectionGraph(t)
	whole := projectionWhole(t, graph)
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{Metaclasses: []string{"PartDefinition"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	if len(projected) != 3 {
		t.Fatalf("projection by metaclass emitted %d elements, want 3:\n%s", len(projected), out)
	}
	for _, element := range projected {
		if got := elementField(t, element, "@type"); got != "PartDefinition" {
			t.Errorf("projection emitted @type %q, want PartDefinition", got)
		}
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

func TestAPIJSONProjectionByAnnotationType(t *testing.T) {
	graph := projectionGraph(t)
	whole := projectionWhole(t, graph)
	for _, name := range []string{"P::Facet", "Facet"} {
		t.Run(name, func(t *testing.T) {
			out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{name}}})
			if err != nil {
				t.Fatalf("WriteAPIJSONWithOptions: %v", err)
			}
			projected := splitAPIJSONElements(t, out)
			byType := map[string]bool{}
			for _, element := range projected {
				byType[elementField(t, element, "@type")] = true
			}
			for _, want := range []string{"MetadataUsage", "Annotation", "PartDefinition"} {
				if !byType[want] {
					t.Errorf("projection by %q did not emit a %s: %v", name, want, byType)
				}
			}
			if byType["Comment"] {
				t.Errorf("projection by %q emitted the unrelated comment", name)
			}
			if len(projected) >= len(whole) {
				t.Errorf("projection emitted %d of %d elements, want fewer", len(projected), len(whole))
			}
			assertProjectedAreByteIdentical(t, whole, projected)
		})
	}
}

func projectionRefs(t *testing.T, element json.RawMessage, key string) []string {
	t.Helper()
	raw := elementField(t, element, key)
	if raw == "" {
		return nil
	}
	var refs []APIJSONReference
	if err := json.Unmarshal([]byte(raw), &refs); err == nil {
		ids := make([]string, 0, len(refs))
		for _, ref := range refs {
			ids = append(ids, ref.ID)
		}
		return ids
	}
	var one APIJSONReference
	if err := json.Unmarshal([]byte(raw), &one); err != nil {
		t.Fatalf("%s is not a reference or array of references: %s", key, raw)
	}
	return []string{one.ID}
}

// A selected annotation must carry the elements that hold its bindings, so a
// reader can follow the projected usage to the literal a metadata feature binds.
func TestAPIJSONProjectionCarriesAnnotationBindings(t *testing.T) {
	t.Run("about", func(t *testing.T) {
		graph := projectionGraph(t)
		whole := projectionWhole(t, graph)
		out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"P::Facet"}}})
		if err != nil {
			t.Fatalf("WriteAPIJSONWithOptions: %v", err)
		}
		projected := splitAPIJSONElements(t, out)

		usage := elementByID(t, projected, "P__facet")
		members := projectionRefs(t, usage, "ownedMember")
		if len(members) != 1 || members[0] != "P__facet__note" {
			t.Fatalf("projected usage owns %v, want [P__facet__note]", members)
		}
		member := elementByID(t, projected, members[0])
		values := projectionRefs(t, member, "value")
		if len(values) != 1 || values[0] != "P__facet__note_pvalue" {
			t.Fatalf("projected member's value is %v, want [P__facet__note_pvalue]", values)
		}
		literal := elementByID(t, projected, values[0])
		if got := elementField(t, literal, "value"); got != "a facet" {
			t.Errorf("bound literal reads %q, want %q", got, "a facet")
		}
		if got := elementField(t, elementByID(t, projected, "P__facet__note_pvalue_om"), "@type"); got != "FeatureValue" {
			t.Errorf("the binding's FeatureValue is absent or written as %q", got)
		}
		for _, element := range projected {
			if id := elementField(t, element, "@id"); id == "P__Facet" {
				t.Error("the projection followed the annotating element's type reference to its definition")
			}
		}
		assertProjectedAreByteIdentical(t, whole, projected)
	})

	t.Run("body", func(t *testing.T) {
		graph := graphOfNotation(t, `package P {
  metadata def Facet;
  part def Vehicle {
    @Facet { note = "n"; }
  }
}
`)
		whole := projectionWhole(t, graph)
		out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"Facet"}}})
		if err != nil {
			t.Fatalf("WriteAPIJSONWithOptions: %v", err)
		}
		projected := splitAPIJSONElements(t, out)

		var usage json.RawMessage
		var featureValues int
		for _, element := range projected {
			switch elementField(t, element, "@type") {
			case "MetadataUsage":
				usage = element
			case "FeatureValue":
				featureValues++
			}
		}
		if usage == nil {
			t.Fatalf("projection emitted no metadata usage: %s", out)
		}
		if featureValues == 0 {
			t.Fatalf("projection emitted no FeatureValue holding a binding: %s", out)
		}
		members := projectionRefs(t, usage, "ownedMember")
		if len(members) == 0 {
			t.Fatalf("projected usage owns no members: %s", out)
		}
		followed := false
		for _, memberID := range members {
			member := elementByID(t, projected, memberID)
			values := projectionRefs(t, member, "value")
			for _, valueID := range values {
				literal := elementByID(t, projected, valueID)
				if got := elementField(t, literal, "value"); got == "n" {
					followed = true
				}
			}
		}
		if !followed {
			t.Errorf("no projected member's value resolves to the bound literal %q: %s", "n", out)
		}
		for _, element := range projected {
			if id := elementField(t, element, "@id"); id == "P__Facet" {
				t.Error("the projection followed the annotating element's type reference to its definition")
			}
		}
		assertProjectedAreByteIdentical(t, whole, projected)
	})
}

// An alias in an annotation's body references its target; the projection must
// write the alias but not follow that reference into the target's declarations.
func TestAPIJSONProjectionDoesNotFollowAliasTargets(t *testing.T) {
	graph := graphOfNotation(t, `package Lib {
  part def Shared { part def Inner; }
}
package P {
  private import Lib::*;
  metadata def Facet;
  part def Vehicle {
    @Facet {
      alias shared for Lib::Shared;
      note = "n";
    }
  }
}
`)
	whole := projectionWhole(t, graph)
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"Facet"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	ids := map[string]bool{}
	for _, element := range projected {
		ids[elementField(t, element, "@id")] = true
	}
	if !ids["P__Vehicle___400__shared"] {
		t.Errorf("projection did not write the alias itself: %v", ids)
	}
	for _, unrelated := range []string{"Lib", "Lib__Shared", "Lib__Shared__Inner", "Lib__Shared__Inner_om"} {
		if ids[unrelated] {
			t.Errorf("projection followed the alias reference and wrote %s", unrelated)
		}
	}
	usage := elementByID(t, projected, "P__Vehicle___400")
	members := projectionRefs(t, usage, "ownedMember")
	found := false
	for _, memberID := range members {
		values := projectionRefs(t, elementByID(t, projected, memberID), "value")
		for _, valueID := range values {
			if got := elementField(t, elementByID(t, projected, valueID), "value"); got == "n" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("projection lost the binding value while dropping the alias target: %s", out)
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

// A metadata definition's declared short name is a selector even when it is
// not the last segment of the definition's qualified name.
func TestAPIJSONProjectionByAnnotationDeclaredShortName(t *testing.T) {
	graph := graphOfNotation(t, `package P {
  metadata def <facet> QualityFacet;
  part def Faceted;
  metadata tag : QualityFacet about Faceted { note = "n"; }
}
`)
	whole := projectionWhole(t, graph)
	for _, name := range []string{"facet", "P::QualityFacet", "QualityFacet"} {
		t.Run(name, func(t *testing.T) {
			out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{name}}})
			if err != nil {
				t.Fatalf("WriteAPIJSONWithOptions: %v", err)
			}
			projected := splitAPIJSONElements(t, out)
			byType := map[string]bool{}
			for _, element := range projected {
				byType[elementField(t, element, "@type")] = true
			}
			for _, want := range []string{"MetadataUsage", "Annotation", "PartDefinition"} {
				if !byType[want] {
					t.Errorf("projection by %q did not emit a %s: %v", name, want, byType)
				}
			}
			assertProjectedAreByteIdentical(t, whole, projected)
		})
	}
}

// A quoted name holds its own "::" inside one segment; a selector must split
// qualified names at segment boundaries, not at the last "::" in the string.
func TestAPIJSONProjectionByAnnotationQuotedName(t *testing.T) {
	graph := graphOfNotation(t, `package P {
  metadata def 'safety::limit';
  part def Faceted;
  metadata tag : 'safety::limit' about Faceted { note = "n"; }
}
`)
	whole := projectionWhole(t, graph)
	for _, name := range []string{"P::'safety::limit'", "'safety::limit'", "safety::limit"} {
		t.Run(name, func(t *testing.T) {
			out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{name}}})
			if err != nil {
				t.Fatalf("WriteAPIJSONWithOptions: %v", err)
			}
			projected := splitAPIJSONElements(t, out)
			byType := map[string]bool{}
			for _, element := range projected {
				byType[elementField(t, element, "@type")] = true
			}
			for _, want := range []string{"MetadataUsage", "Annotation", "PartDefinition"} {
				if !byType[want] {
					t.Errorf("projection by %q did not emit a %s: %v", name, want, byType)
				}
			}
			assertProjectedAreByteIdentical(t, whole, projected)
		})
	}
	if _, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"limit'"}}}); err == nil {
		t.Error("a truncated quoted segment must be refused")
	}
}

// A body's doc, rep, @facet and #facet annotations are owned members; their
// names select them with the declaration that owns them.
func TestAPIJSONProjectionByBodyAnnotationTypes(t *testing.T) {
	graph := graphOfNotation(t, `package P {
  metadata def Facet;
  part def Vehicle {
    doc /* a vehicle */
    rep language "markdown" /* body */
    @Facet { note = "n"; }
  }
  #Facet part def Hashed;
}
`)
	whole := projectionWhole(t, graph)
	for _, tc := range []struct {
		name string
		ids  []string
	}{
		{"Documentation", []string{"P__Vehicle___400", "P__Vehicle"}},
		{"TextualRepresentation", []string{"P__Vehicle___401", "P__Vehicle"}},
		{"P::Facet", []string{"P__Vehicle___402", "P__Hashed___400", "P__Vehicle", "P__Hashed"}},
		{"Facet", []string{"P__Vehicle___402", "P__Hashed___400", "P__Vehicle", "P__Hashed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{tc.name}}})
			if err != nil {
				t.Fatalf("WriteAPIJSONWithOptions: %v", err)
			}
			projected := splitAPIJSONElements(t, out)
			ids := map[string]bool{}
			for _, element := range projected {
				ids[elementField(t, element, "@id")] = true
			}
			for _, want := range tc.ids {
				if !ids[want] {
					t.Errorf("projection by %q did not emit %s: %v", tc.name, want, ids)
				}
			}
			if len(projected) >= len(whole) {
				t.Errorf("projection by %q wrote %d of %d elements, want fewer", tc.name, len(projected), len(whole))
			}
			assertProjectedAreByteIdentical(t, whole, projected)
		})
	}
}

func TestAPIJSONProjectionByAnnotationMetaclass(t *testing.T) {
	graph := projectionGraph(t)
	whole := projectionWhole(t, graph)
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"Comment"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	byType := map[string]bool{}
	for _, element := range projected {
		byType[elementField(t, element, "@type")] = true
	}
	for _, want := range []string{"Comment", "Annotation", "PartDefinition"} {
		if !byType[want] {
			t.Errorf("projection by Comment did not emit a %s: %v", want, byType)
		}
	}
	assertProjectedAreByteIdentical(t, whole, projected)
}

func TestAPIJSONProjectionUnionsSelectors(t *testing.T) {
	graph := projectionGraph(t)
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{
		QualifiedNames:  []string{"P::Plain"},
		Metaclasses:     []string{"MetadataDefinition"},
		AnnotationTypes: []string{"P::Facet"},
	}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	projected := splitAPIJSONElements(t, out)
	if len(projected) < 5 {
		t.Fatalf("union of three selectors emitted only %d elements:\n%s", len(projected), out)
	}
}

func TestAPIJSONProjectionTopLevelAnnotationTypeShortName(t *testing.T) {
	// A definition at the root has a qualified name with no "::" segment; its
	// short name is the whole name, not a truncation of it.
	graph := graphOfNotation(t, `metadata def TopFacet;
part def Tagged;
metadata tag : TopFacet about Tagged;
`)
	for _, name := range []string{"TopFacet"} {
		out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{name}}})
		if err != nil {
			t.Fatalf("projection by %q: %v", name, err)
		}
		if len(splitAPIJSONElements(t, out)) == 0 {
			t.Errorf("projection by %q emitted nothing", name)
		}
	}
	_, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{AnnotationTypes: []string{"opFacet"}}})
	var unknown *APIJSONProjectionError
	if !errors.As(err, &unknown) {
		t.Fatalf("a truncated annotation type name must be refused, got %v", err)
	}
}

func TestAPIJSONProjectionUnknownNames(t *testing.T) {
	graph := projectionGraph(t)
	for _, tc := range []struct {
		selector string
		p        APIJSONProjection
	}{
		{"qualified_names", APIJSONProjection{QualifiedNames: []string{"P::Missing"}}},
		{"metaclasses", APIJSONProjection{Metaclasses: []string{"NoSuchMetaclass"}}},
		{"annotation_types", APIJSONProjection{AnnotationTypes: []string{"NoSuchFacet"}}},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			_, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &tc.p})
			var projectionErr *APIJSONProjectionError
			if !errors.As(err, &projectionErr) {
				t.Fatalf("want an APIJSONProjectionError, got %v", err)
			}
			if projectionErr.Selector != tc.selector {
				t.Errorf("error names selector %q, want %q", projectionErr.Selector, tc.selector)
			}
		})
	}
}

func TestAPIJSONProjectionEmptyWritesWhole(t *testing.T) {
	graph := projectionGraph(t)
	whole, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*APIJSONProjection{nil, {}, {QualifiedNames: []string{}, Metaclasses: []string{}}} {
		out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: p})
		if err != nil {
			t.Fatalf("WriteAPIJSONWithOptions: %v", err)
		}
		if !bytes.Equal(whole, out) {
			t.Errorf("empty projection changed the output:\n%s\nvs\n%s", whole, out)
		}
	}
}

func TestAPIJSONProjectionDeclaredMetaclassAbsentInModelIsKnown(t *testing.T) {
	graph := projectionGraph(t)
	// ActionUsage is a metaclass of the metamodel but no element of the
	// fixture has it; the projection is empty, not an argument fault.
	out, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{Projection: &APIJSONProjection{Metaclasses: []string{"ActionUsage"}}})
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "[]" {
		t.Errorf("projection of an absent metaclass wrote %s, want []", got)
	}
}
