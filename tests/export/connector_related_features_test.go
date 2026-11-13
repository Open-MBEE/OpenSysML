package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// chainedConnectorModel connects features by plain names and by feature chains,
// through a connection and an interface.
const chainedConnectorModel = `package ChainedEnds {
    port def P;
    part def Inner { port p : P; }
    part def Outer { part inner : Inner; }
    part top {
        part a : Outer;
        part b : Inner;
        connection direct connect a to b;
        connection chained connect a.inner.p to b.p;
        interface chainedIf connect a.inner.p to b.p;
    }
}
`

// A connector's relatedFeature, sourceFeature and targetFeature are the features its ends'
// ReferenceSubsettings reference (KerML 8.3.4.5.3): the chain Feature for a chained end.
func TestConnectorRelatedFeaturesAreTheEndsReferencedFeatures(t *testing.T) {
	turtle, err := convert.Convert("c.sysml", []byte(chainedConnectorModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	for _, name := range []string{"direct", "chained", "chainedIf"} {
		subject := rdf.IRI(rdf.Element + "ChainedEnds__top__" + name)
		var referenced []rdf.Term
		for _, end := range graph.Objects(subject, rdf.SysML+"connectorEnd") {
			subsettings := graph.Objects(end, rdf.SysML+"ownedReferenceSubsetting")
			if len(subsettings) != 1 {
				t.Fatalf("%s end <%s> owns %d reference subsettings, want one", name, end.Value, len(subsettings))
			}
			target, ok := graph.Object(subsettings[0], rdf.SysML+"referencedFeature")
			if !ok {
				t.Fatalf("%s end <%s> references nothing", name, end.Value)
			}
			referenced = append(referenced, target)
		}
		if len(referenced) != 2 {
			t.Fatalf("%s has %d ends, want 2", name, len(referenced))
		}
		if name != "direct" && !strings.HasSuffix(referenced[0].Value, "_pchain") {
			t.Errorf("%s first end references <%s>, want the chain Feature", name, referenced[0].Value)
		}
		if got := graph.Objects(subject, rdf.SysML+"relatedFeature"); len(got) != 2 || got[0] != referenced[0] || got[1] != referenced[1] {
			t.Errorf("%s relatedFeature = %v, want the ends' referenced features %v", name, got, referenced)
		}
		if got, ok := graph.Object(subject, rdf.SysML+"sourceFeature"); !ok || got != referenced[0] {
			t.Errorf("%s sourceFeature = %v (%t), want %v", name, got, ok, referenced[0])
		}
		if got := graph.Objects(subject, rdf.SysML+"targetFeature"); len(got) != 1 || got[0] != referenced[1] {
			t.Errorf("%s targetFeature = %v, want [%v]", name, got, referenced[1])
		}
		// Relationship::relatedElement, source and target are what the three redefine (KerML 8.3.4.5.1):
		// the chain Feature for a chained end, as the pilot exports it.
		if got := graph.Objects(subject, rdf.SysML+"relatedElement"); len(got) != 2 || got[0] != referenced[0] || got[1] != referenced[1] {
			t.Errorf("%s relatedElement = %v, want %v", name, got, referenced)
		}
		if got, ok := graph.Object(subject, rdf.SysML+"source"); !ok || got != referenced[0] {
			t.Errorf("%s source = %v (%t), want %v", name, got, ok, referenced[0])
		}
		if got := graph.Objects(subject, rdf.SysML+"target"); len(got) != 1 || got[0] != referenced[1] {
			t.Errorf("%s target = %v, want [%v]", name, got, referenced[1])
		}
	}

	// The API element form states them too, by the ids the chain Features carry.
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("api-json: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("api-json is not an element array: %v", err)
	}
	payloads := map[string]map[string]json.RawMessage{}
	byQualifiedName := map[string]map[string]json.RawMessage{}
	for _, element := range elements {
		var id, qualifiedName string
		if err := json.Unmarshal(element["@id"], &id); err == nil {
			payloads[id] = element
		}
		if err := json.Unmarshal(element["qualifiedName"], &qualifiedName); err == nil {
			byQualifiedName[qualifiedName] = element
		}
	}
	ref := func(raw json.RawMessage) string {
		var r struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("not a reference: %s", raw)
		}
		return r.ID
	}
	refs := func(raw json.RawMessage) []string {
		var rs []json.RawMessage
		if err := json.Unmarshal(raw, &rs); err != nil {
			t.Fatalf("not a reference list: %s", raw)
		}
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = ref(r)
		}
		return out
	}
	for _, name := range []string{"direct", "chained", "chainedIf"} {
		connector, ok := byQualifiedName["ChainedEnds::top::"+name]
		if !ok {
			t.Fatalf("api-json has no element ChainedEnds::top::%s", name)
		}
		var want []string
		for _, end := range refs(connector["connectorEnd"]) {
			subsetting := ref(payloads[end]["ownedReferenceSubsetting"])
			want = append(want, ref(payloads[subsetting]["referencedFeature"]))
		}
		if got := refs(connector["relatedFeature"]); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("api-json %s relatedFeature = %v, want %v", name, got, want)
		}
		if got := ref(connector["sourceFeature"]); got != want[0] {
			t.Errorf("api-json %s sourceFeature = %s, want %s", name, got, want[0])
		}
		if got := refs(connector["targetFeature"]); strings.Join(got, ",") != want[1] {
			t.Errorf("api-json %s targetFeature = %v, want [%s]", name, got, want[1])
		}
		if name == "direct" {
			if got := refs(connector["relatedElement"]); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("api-json %s relatedElement = %v, want %v", name, got, want)
			}
			if got := refs(connector["source"]); strings.Join(got, ",") != want[0] {
				t.Errorf("api-json %s source = %v, want [%s]", name, got, want[0])
			}
			if got := refs(connector["target"]); strings.Join(got, ",") != want[1] {
				t.Errorf("api-json %s target = %v, want [%s]", name, got, want[1])
			}
		}
		if name != "direct" {
			chain := payloads[want[0]]
			if chain == nil || string(chain["@type"]) != `"Feature"` || len(refs(chain["chainingFeature"])) != 3 {
				t.Errorf("api-json %s sourceFeature %s is not a chain Feature of three chaining features: %s", name, want[0], chain)
			}
		}
	}
}

// A `first a then b;` succession states Relationship::source and target beside the sourceFeature and
// targetFeature that redefine them (KerML 8.3.4.5.1), as a connector written with plain ends does.
func TestInitialSuccessionStatesItsRelationshipEnds(t *testing.T) {
	const model = `package Seq { action act { action a; action b; first a then b; } }`
	turtle, err := convert.Convert("s.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	var subject rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+"sourceFeature" && graph.HasProperty(triple.Subject, rdf.SysML+"targetFeature") {
			subject = triple.Subject
		}
	}
	if subject.Value == "" {
		t.Fatal("no succession with both ends in the graph")
	}
	for _, pair := range [][2]string{{"sourceFeature", "source"}, {"targetFeature", "target"}} {
		stated, ok := graph.Object(subject, rdf.SysML+pair[0])
		if !ok {
			t.Fatalf("%s absent", pair[0])
		}
		if got, ok := graph.Object(subject, rdf.SysML+pair[1]); !ok || got != stated {
			t.Errorf("%s = %v (%t), want %v", pair[1], got, ok, stated)
		}
	}
}

// A `then x;` succession states Relationship::source and target beside the sourceFeature and targetFeature
// it names — the `first start` source, the named member before it, a plain-named target, the library's `done`.
func TestThenSuccessionStatesItsRelationshipEnds(t *testing.T) {
	const model = `package Seq { action act { first start; then action a; then action b; then done; } }`
	turtle, err := convert.Convert("s.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	var successions []rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+"targetFeature" && graph.HasProperty(triple.Subject, rdf.OpenSysML+"endForm") {
			successions = append(successions, triple.Subject)
		}
	}
	if len(successions) != 3 {
		t.Fatalf("%d `then` successions with a targetFeature, want 3", len(successions))
	}
	stated := 0
	for _, subject := range successions {
		for _, pair := range [][2]string{{"sourceFeature", "source"}, {"targetFeature", "target"}} {
			feature, named := graph.Object(subject, rdf.SysML+pair[0])
			got, ok := graph.Object(subject, rdf.SysML+pair[1])
			switch {
			case named && feature.IsIRI():
				if !ok || got != feature {
					t.Errorf("%s %s = %v (%t), want %v", subject.Value, pair[1], got, ok, feature)
				}
				stated++
			case ok:
				t.Errorf("%s %s = %v stated without a %s", subject.Value, pair[1], got, pair[0])
			}
		}
	}
	if stated != 6 {
		t.Errorf("%d Relationship ends stated, want 6 (start→a, a→b, b→done)", stated)
	}
}
