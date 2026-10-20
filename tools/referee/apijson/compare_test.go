package apijson

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

const probeFile = "probe.sysml"

// probeSides exports testdata/probe.sysml through this repository's writer and
// reads the committed export the pinned pilot wrote for it, so the comparison
// runs without Java.
func probeSides(t *testing.T) (*oursExport, pilotDocument) {
	t.Helper()
	ours, err := exportOurs("testdata", []string{probeFile})
	if err != nil {
		t.Fatalf("export ours: %v", err)
	}
	content, err := os.ReadFile(filepath.Join("testdata", "probe.pilot.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := readPilotDocument(content)
	if err != nil {
		t.Fatalf("read the pilot probe: %v", err)
	}
	return ours, document
}

func pilotElements(t *testing.T, document pilotDocument) []*element {
	t.Helper()
	result := &pilotExport{Unkeyed: make(map[string]int)}
	elements, _, ambiguous, _ := dropAmbiguous(keyPilot(document, result))
	if ambiguous != 0 {
		t.Fatalf("pilot probe: %d ambiguous elements", ambiguous)
	}
	return elements
}

func rowFor(rows []Row, metaclass, property string) (Row, bool) {
	for _, row := range rows {
		if row.Metaclass == metaclass && row.Property == property {
			return row, true
		}
	}
	return Row{}, false
}

func TestProbeMatchesNamedAndUnnamedElementsAcrossSides(t *testing.T) {
	ours, document := probeSides(t)
	if ours.Refused != 0 || ours.Separate {
		t.Fatalf("ours: refused %d, separate %v", ours.Refused, ours.Separate)
	}
	c := newComparison()
	elements, _ := c.compareRoot("probe", ours.Elements, pilotElements(t, document))
	if elements.Matched < 8 {
		t.Fatalf("matched %d elements, want the probe's named and unnamed ones:\n%+v", elements.Matched, c.unmatchedList())
	}
	rows := c.scorecard()
	for _, want := range []struct{ metaclass, property string }{
		{"PartDefinition", "declaredName"},
		{"PartDefinition", "owner"},
		{"OwningMembership", "memberElement"},
		{"AttributeUsage", "declaredName"},
	} {
		row, ok := rowFor(rows, want.metaclass, want.property)
		if !ok || row.PresentEqual == 0 {
			t.Errorf("(%s, %s): want at least one present-equal comparison, got %+v", want.metaclass, want.property, row)
		}
	}
}

func TestReportIsDeterministic(t *testing.T) {
	var reports [][]byte
	for run := 0; run < 2; run++ {
		ours, document := probeSides(t)
		c := newComparison()
		elements, buckets := c.compareRoot("probe", ours.Elements, pilotElements(t, document))
		rows := c.scorecard()
		report := Report{Totals: Totals{Elements: elements, Properties: buckets, Rows: len(rows)}}
		report.setScorecard(rows)
		examples := Examples{Examples: c.exampleList(rows), Unmatched: c.unmatchedList()}
		encoded, err := json.Marshal(struct {
			Report
			Examples
		}{report, examples})
		if err != nil {
			t.Fatal(err)
		}
		reports = append(reports, encoded)
	}
	if !bytes.Equal(reports[0], reports[1]) {
		t.Fatal("two runs over the same inputs produced different reports")
	}
}

// mutate copies the pilot probe with one payload property of one element replaced.
func mutate(t *testing.T, document pilotDocument, key, property string, value any) pilotDocument {
	t.Helper()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := readPilotDocument(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range copied.Elements {
		if id, _ := item.Payload["@id"].(string); copied.Keys[id] == key {
			item.Payload[property] = value
			return copied
		}
	}
	t.Fatalf("no element keyed %s in the pilot probe", key)
	return pilotDocument{}
}

func TestDisagreementDetectorIsLive(t *testing.T) {
	_, document := probeSides(t)
	pilot := pilotElements(t, document)
	same := newComparison()
	same.compareRoot("probe", pilotElements(t, document), pilot)
	if same.buckets.Differing() != 0 {
		t.Fatalf("the pilot probe compared against itself differs: %+v", same.buckets)
	}

	vehicle := "Derived Probe::Vehicle"
	emptied := mutate(t, document, vehicle, "ownedUsage", []any{})
	flipped := mutate(t, document, vehicle, "isAbstract", true)
	for _, tc := range []struct {
		name, property, bucket string
		scratch                pilotDocument
	}{
		{"an emptied derived collection", "ownedUsage", bucketOursMissing, emptied},
		{"a flipped attribute", "isAbstract", bucketPresentDifferent, flipped},
	} {
		c := newComparison()
		c.compareRoot("probe", pilotElements(t, tc.scratch), pilot)
		row, ok := rowFor(c.scorecard(), "PartDefinition", tc.property)
		if !ok {
			t.Fatalf("%s: no (PartDefinition, %s) row", tc.name, tc.property)
		}
		var moved int
		switch tc.bucket {
		case bucketOursMissing:
			moved = row.OursMissing
		case bucketPresentDifferent:
			moved = row.PresentDifferent
		}
		if moved != 1 || c.buckets.Differing() != 1 {
			t.Errorf("%s: want exactly one %s comparison in the row and none elsewhere, got row %+v, totals %+v", tc.name, tc.bucket, row, c.buckets)
		}
		examples := c.exampleList(c.scorecard())
		if len(examples) != 1 || examples[0].Element != vehicle || examples[0].Bucket != tc.bucket {
			t.Errorf("%s: want one example on %s, got %+v", tc.name, vehicle, examples)
		}
	}
}

func TestBucketOfEquatesOnlyRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ours, pilot any
		want        string
	}{
		{"both absent", nil, nil, bucketAbsentBoth},
		{"null against empty collection", nil, []any{}, bucketAbsentBoth},
		{"ours empty collection", []any{}, []any{ref{"P::a"}}, bucketOursMissing},
		{"pilot null", "x", nil, bucketPilotMissing},
		{"absent boolean is false", nil, false, bucketAbsentBoth},
		{"absent boolean against true", nil, true, bucketOursMissing},
		{"equal references", []any{ref{"P::a"}, ref{"P::b"}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketPresentEqual},
		{"reordered references", []any{ref{"P::b"}, ref{"P::a"}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketPresentDifferent},
		{"unkeyed reference differs", ref{unkeyed}, ref{"P::a"}, bucketPresentDifferent},
		{"numbers", 1.0, 1.0, bucketPresentEqual},
	} {
		if got := bucketOf(tc.ours, tc.pilot); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestCanonicalKeyUnquotesNameSegments(t *testing.T) {
	for in, want := range map[string]string{
		"P::Car":                              "P::Car",
		"'Derived Probe'::Vehicle":            "Derived Probe::Vehicle",
		"'Derived Probe'/OwningMembership[1]": "Derived Probe/OwningMembership[1]",
		`'it\'s'::'a b'/FeatureMembership[0]`: "it's::a b/FeatureMembership[0]",
	} {
		if got := canonicalKey(in); got != want {
			t.Errorf("canonicalKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeyOursAddressesUnnamedElementsStructurally(t *testing.T) {
	raw := []map[string]any{
		{"@id": "P_ns", "@type": "Namespace", "ownedRelationship": []any{map[string]any{"@id": "P_om"}}},
		{"@id": "P_om", "@type": "OwningMembership", "owner": map[string]any{"@id": "P_ns"}, "owningRelatedElement": map[string]any{"@id": "P_ns"}, "ownedRelatedElement": []any{map[string]any{"@id": "P"}}},
		{"@id": "P", "@type": "Package", "qualifiedName": "P", "owner": map[string]any{"@id": "P_ns"}, "ownedRelationship": []any{map[string]any{"@id": "P_im"}, map[string]any{"@id": "P_om2"}}},
		{"@id": "P_im", "@type": "NamespaceImport", "qualifiedName": "P::@1", "owner": map[string]any{"@id": "P"}, "owningRelatedElement": map[string]any{"@id": "P"}},
		{"@id": "P_om2", "@type": "OwningMembership", "owner": map[string]any{"@id": "P"}, "owningRelatedElement": map[string]any{"@id": "P"}, "ownedRelatedElement": []any{map[string]any{"@id": "P_x"}}},
		{"@id": "P_x", "@type": "LiteralInteger", "owner": map[string]any{"@id": "P"}, "value": 1.0},
	}
	elements, unkeyable := keyOurs(raw, "probe.sysml", map[string]int{})
	if unkeyable != 0 {
		t.Fatalf("%d unkeyable elements", unkeyable)
	}
	got := make(map[string]string, len(elements))
	for _, e := range elements {
		got[e.Type] = e.Key
	}
	want := map[string]string{
		"Namespace":        "/probe.sysml/Namespace#0",
		"OwningMembership": "P/OwningMembership[0]",
		"Package":          "P",
		"NamespaceImport":  "P/NamespaceImport[0]",
		"LiteralInteger":   "P/LiteralInteger[0]",
	}
	for typ, key := range want {
		if got[typ] != key {
			t.Errorf("%s keyed %q, want %q", typ, got[typ], key)
		}
	}
	if _, ok := got["OwningMembership"]; !ok {
		t.Fatal("no OwningMembership keyed")
	}
	for _, e := range elements {
		if e.Type == "OwningMembership" && e.Key == "/probe.sysml/Namespace#0/OwningMembership[0]" {
			return
		}
	}
	t.Error("the root namespace's membership is not keyed under the namespace")
}

func TestRootsAreTheDifferentialsPlusTheManualExamples(t *testing.T) {
	want := append(diff.Roots(), manualExamples)
	if got := roots(); !reflect.DeepEqual(got, want) {
		t.Fatalf("roots() = %+v, want %+v", got, want)
	}
}
