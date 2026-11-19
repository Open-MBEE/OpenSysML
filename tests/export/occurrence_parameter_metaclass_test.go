package export_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// occurrenceParameters declares each occurrence prefix on a parameter and on a
// member, named and unnamed; the pinned validator accepts it clean.
var occurrenceParameters = filepath.Join("testdata", "convert", "occurrence_parameters.sysml")

// A usage whose prefix states `individual` or a portion kind is an OccurrenceUsage
// named or not, directed or not (SysML.xtext IndividualUsage, PortionUsage; SysML v2 §8.3.9.4).
func TestOccurrencePrefixedParameterIsAnOccurrenceUsage(t *testing.T) {
	source, err := os.ReadFile(occurrenceParameters)
	if err != nil {
		t.Fatal(err)
	}
	document, err := cachedAPIJSON(occurrenceParameters, source, "standard", "graph")
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []struct {
		Type         string  `json:"@type"`
		Name         *string `json:"declaredName"`
		Direction    *string `json:"direction"`
		PortionKind  *string `json:"portionKind"`
		IsIndividual bool    `json:"isIndividual"`
		IsPortion    bool    `json:"isPortion"`
		IsReference  bool    `json:"isReference"`
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	text := func(s *string) string {
		if s == nil {
			return "-"
		}
		return *s
	}
	type row struct {
		metaclass, name, direction, portion string
		individual, isPortion, reference    bool
	}
	var got []row
	for _, e := range elements {
		if e.Direction == nil && e.PortionKind == nil && !e.IsIndividual {
			continue
		}
		got = append(got, row{e.Type, text(e.Name), text(e.Direction), text(e.PortionKind), e.IsIndividual, e.IsPortion, e.IsReference})
	}
	want := []row{
		{"PartDefinition", "Vehicle1", "-", "-", true, false, false},
		{"OccurrenceUsage", "-", "in", "snapshot", false, true, true},
		{"OccurrenceUsage", "atStart", "in", "snapshot", false, true, true},
		{"OccurrenceUsage", "-", "out", "timeslice", false, true, true},
		{"OccurrenceUsage", "-", "in", "-", true, false, true},
		{"OccurrenceUsage", "v", "in", "-", true, false, true},
		{"OccurrenceUsage", "-", "in", "snapshot", true, true, true},
		{"OccurrenceUsage", "vs", "in", "snapshot", true, true, true},
		{"OccurrenceUsage", "-", "in", "timeslice", false, true, true},
		{"ReferenceUsage", "x", "in", "-", false, false, true},
		{"OccurrenceUsage", "-", "-", "timeslice", false, true, false},
		{"OccurrenceUsage", "-", "-", "-", true, false, false},
		{"OccurrenceUsage", "ms", "-", "snapshot", true, true, false},
		{"OccurrenceUsage", "-", "-", "snapshot", true, true, false},
		{"OccurrenceUsage", "rs", "-", "snapshot", false, true, true},
		{"OccurrenceUsage", "-", "-", "timeslice", true, true, true},
	}
	if len(got) != len(want) {
		t.Fatalf("directed, portion and individual usages = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("usage %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// An enumerated value is an EnumerationUsage though it declares no name
// (SysML.xtext EnumeratedValue returns SysML::EnumerationUsage).
func TestUnnamedEnumeratedValueIsAnEnumerationUsage(t *testing.T) {
	document, err := convert.Convert("sizes.sysml", []byte(`package Sizes {
    attribute def Size :> ScalarValues::Real;
    enum def SizeChoice :> Size {
        = 60.0;
        enum = 70.0;
    }
}
`), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []struct {
		Type string `json:"@type"`
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, e := range elements {
		counts[e.Type]++
	}
	if counts["EnumerationUsage"] != 2 || counts["ReferenceUsage"] != 0 {
		t.Errorf("EnumerationUsage = %d, ReferenceUsage = %d; want 2 and 0", counts["EnumerationUsage"], counts["ReferenceUsage"])
	}
}
