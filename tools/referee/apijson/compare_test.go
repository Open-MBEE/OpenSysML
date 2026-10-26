package apijson

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

const probeFile = "probe.sysml"

// probeSides exports testdata/probe.sysml through this repository's writer and
// reads the committed export the pinned pilot wrote for it, so the comparison
// runs without Java.
func probeSides(t *testing.T) (*oursExport, pilotDocument) {
	t.Helper()
	ours, err := exportOurs("testdata", []string{probeFile}, nil)
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
		property    string
		ours, pilot any
		want        string
	}{
		{"both absent", "ownedMember", nil, nil, bucketAbsentBoth},
		{"null against empty collection", "ownedMember", nil, []any{}, bucketAbsentBoth},
		{"ours empty collection", "ownedMember", []any{}, []any{ref{"P::a"}}, bucketOursMissing},
		{"pilot null", "declaredName", "x", nil, bucketPilotMissing},
		{"absent boolean is its false default", "isAbstract", nil, false, bucketAbsentBoth},
		{"absent boolean against true", "isAbstract", nil, true, bucketOursMissing},
		{"absent isUnique is its true default", "isUnique", nil, true, bucketAbsentBoth},
		{"absent isUnique against nonunique", "isUnique", nil, false, bucketOursMissing},
		{"stated nonunique against absent", "isUnique", false, nil, bucketPilotMissing},
		{"derived boolean has no default", "isReference", nil, false, bucketOursMissing},
		{"derived boolean stated against absent", "isLibraryElement", false, nil, bucketPilotMissing},
		{"equal references", "ownedMember", []any{ref{"P::a"}, ref{"P::b"}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketPresentEqual},
		{"reordered references", "ownedMember", []any{ref{"P::b"}, ref{"P::a"}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketPresentDifferent},
		{"unkeyed reference is indeterminate", "source", ref{unkeyed}, ref{"P::a"}, bucketIndeterminate},
		{"two unkeyed references are not known equal", "source", ref{unkeyed}, ref{unkeyed}, bucketIndeterminate},
		{"unkeyed reference inside a collection", "ownedMember", []any{ref{"P::a"}, ref{unkeyed}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketIndeterminate},
		{"a known position differs beside an unknown one", "ownedMember", []any{ref{"P::a"}, ref{unkeyed}}, []any{ref{"P::b"}, ref{unkeyed}}, bucketPresentDifferent},
		{"known references reordered beside an unknown one", "ownedMember", []any{ref{"P::b"}, ref{"P::a"}, ref{unkeyed}}, []any{ref{"P::a"}, ref{"P::b"}, ref{unkeyed}}, bucketPresentDifferent},
		{"lengths differ despite an unknown target", "ownedMember", []any{ref{unkeyed}}, []any{ref{"P::a"}, ref{"P::b"}}, bucketPresentDifferent},
		{"same known positions, unknown ones left open", "ownedMember", []any{ref{"P::a"}, ref{unkeyed}}, []any{ref{"P::a"}, ref{unkeyed}}, bucketIndeterminate},
		{"unkeyed id string", "memberElementId", unkeyed, "P::a", bucketIndeterminate},
		{"a reference against a literal", "source", ref{"P::a"}, "P::a", bucketPresentDifferent},
		{"unkeyed reference against absent", "source", ref{unkeyed}, nil, bucketPilotMissing},
		{"numbers", "value", 1.0, 1.0, bucketPresentEqual},
	} {
		if got := bucketOf(tc.property, tc.ours, tc.pilot); got != tc.want {
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

func TestSharedConversionKeysEachDocumentsRootNamespace(t *testing.T) {
	ours, err := exportOurs("testdata", []string{probeFile, "second.sysml"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ours.Separate || ours.Colliding != 0 {
		t.Fatalf("two unrelated documents should share one graph: separate %v, colliding %d", ours.Separate, ours.Colliding)
	}
	keys := make(map[string]string)
	for _, e := range ours.Elements {
		if e.Type == "Namespace" || strings.HasSuffix(e.Key, "/Namespace#0/OwningMembership[0]") {
			keys[e.Key] = e.File
		}
	}
	for _, want := range []string{
		"/" + probeFile + "/Namespace#0", "/" + probeFile + "/Namespace#0/OwningMembership[0]",
		"/second.sysml/Namespace#0", "/second.sysml/Namespace#0/OwningMembership[0]",
	} {
		if _, ok := keys[want]; !ok {
			t.Errorf("no element keyed %s; namespaces keyed %v", want, keys)
		}
	}
}

func TestCollidingDocumentsAreConvertedAlone(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"a.sysml":    "package Gears { part def Gear; }",
		"b.sysml":    "package Gears { part def Gear { attribute teeth : ScalarValues::Integer; } }",
		"uses.sysml": "package Uses { import Gears::*; part g : Gear; }",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ours, err := exportOurs(dir, []string{"a.sysml", "b.sysml", "uses.sysml"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ours.Colliding != 2 || ours.Separate || ours.Unconvertible != 0 {
		t.Fatalf("want the two Gears documents alone and the rest shared: colliding %d, separate %v, unconvertible %d", ours.Colliding, ours.Separate, ours.Unconvertible)
	}
	skipped, err := exportOurs(dir, []string{"a.sysml", "b.sysml", "uses.sysml"}, map[string]bool{"a.sysml": true, "b.sysml": true})
	if err != nil {
		t.Fatal(err)
	}
	if skipped.Skipped != 2 || skipped.Colliding != 0 {
		t.Fatalf("skipping the pilot's failed files: skipped %d, colliding %d", skipped.Skipped, skipped.Colliding)
	}
}

func TestDeclaredNondeterminismScoresTheWholeRow(t *testing.T) {
	expression := func(featuringType any) *element {
		props := map[string]any{"operator": ".", "isAbstract": false}
		if featuringType != nil {
			props["featuringType"] = featuringType
		}
		return &element{Key: "P::a/FeatureChainExpression[0]", Type: "FeatureChainExpression", File: "p.sysml", Props: props}
	}
	for _, tc := range []struct {
		name  string
		ours  *element
		pilot *element
	}{
		{"pilot states a featuringType ours lacks", expression(nil), expression([]any{ref{unkeyed}})},
		{"both state none", expression(nil), expression([]any{})},
		{"the two state different ones", expression([]any{ref{"P::a"}}), expression([]any{ref{"P::b"}})},
	} {
		c := newComparison()
		c.compareRoot("probe", []*element{tc.ours}, []*element{tc.pilot})
		row, ok := rowFor(c.scorecard(), "FeatureChainExpression", "featuringType")
		if !ok {
			t.Fatalf("%s: no (FeatureChainExpression, featuringType) row", tc.name)
		}
		if row.PilotNondeterministic != 1 || row.Buckets.Differing() != 0 || row.AbsentBoth != 0 || row.PresentEqual != 0 || row.Indeterminate != 0 {
			t.Errorf("%s: want the one comparison scored pilot-nondeterministic only, got %+v", tc.name, row.Buckets)
		}
		if len(c.exampleList(c.scorecard())) != 0 {
			t.Errorf("%s: a pilot-nondeterministic comparison is not an example of a difference", tc.name)
		}
		if c.unkeyedReferences != 0 {
			t.Errorf("%s: a declared row's references are not compared, so none is counted unkeyed; got %d", tc.name, c.unkeyedReferences)
		}
		if other, _ := rowFor(c.scorecard(), "FeatureChainExpression", "operator"); other.PresentEqual != 1 || other.PilotNondeterministic != 0 {
			t.Errorf("%s: an undeclared property of the same metaclass is still scored, got %+v", tc.name, other.Buckets)
		}
	}
	c := newComparison()
	pilot := expression([]any{ref{unkeyed}})
	pilot.Type = "FeatureReferenceExpression"
	ours := expression(nil)
	ours.Type = pilot.Type
	c.compareRoot("probe", []*element{ours}, []*element{pilot})
	if row, _ := rowFor(c.scorecard(), pilot.Type, "featuringType"); row.OursMissing != 1 || row.PilotNondeterministic != 0 {
		t.Errorf("the declaration names one metaclass; %s.featuringType is still scored, got %+v", pilot.Type, row.Buckets)
	}
}

func TestDeclaredNondeterminismIsEvidencedAndRecorded(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(committedBaseline)))
	if err != nil {
		t.Fatal(err)
	}
	var recorded Report
	if err := json.Unmarshal(content, &recorded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recorded.PilotNondeterministic, pilotNondeterminism) {
		t.Errorf("the baseline declares %+v, the comparison %+v; re-record the baseline", recorded.PilotNondeterministic, pilotNondeterminism)
	}
	declared := 0
	for _, n := range pilotNondeterminism {
		if n.Evidence == "" {
			t.Errorf("(%s, %s) is declared without evidence", n.Metaclass, n.Property)
		}
		counts, ok := recorded.Scorecard[n.Metaclass+"."+n.Property]
		if !ok {
			t.Errorf("(%s, %s) is declared but the baseline has no such row", n.Metaclass, n.Property)
			continue
		}
		declared += counts[6]
		if counts[6] == 0 || counts != [7]int{0, 0, 0, 0, 0, 0, counts[6]} {
			t.Errorf("(%s, %s) is declared, want every comparison in the pilot-nondeterministic column, got %v", n.Metaclass, n.Property, counts)
		}
	}
	if recorded.Totals.Properties.PilotNondeterministic != declared {
		t.Errorf("the baseline scores %d comparisons pilot-nondeterministic, the declared rows hold %d", recorded.Totals.Properties.PilotNondeterministic, declared)
	}
	for key, counts := range recorded.Scorecard {
		if counts[6] != 0 && !isPilotNondeterministic(strings.SplitN(key, ".", 2)[0], strings.SplitN(key, ".", 2)[1]) {
			t.Errorf("%s has %d pilot-nondeterministic comparisons without a declaration", key, counts[6])
		}
	}
}
