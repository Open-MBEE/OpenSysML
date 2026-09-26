package record

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/format"
)

func TestGenerateSequenceMixedQuantityAndNumberFallsBackToText(t *testing.T) {
	res, err := Generate(Request{
		Package: "Records", Case: "P::temps", Provenance: provenance(KindRun),
		Runs: []Run{{
			Spell:   spell(),
			Outputs: []runtime.CalcOutputValue{{Name: "temps", Value: seqOf(kelvin(300), integer(301))}},
		}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"attribute temps : ScalarValues::String;",
		`attribute :>> temps = "[300.0 [K], 301]";`,
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source is missing %q:\n%s", want, res.Source)
		}
	}
	if strings.Contains(res.Source, "tempsUnit") {
		t.Errorf("mixed quantity/plain sequence unexpectedly has a unit companion:\n%s", res.Source)
	}
}

func TestGenerateExistingSequenceRejectsRepeatedValuesForUniqueMember(t *testing.T) {
	_, err := Generate(Request{
		Package: "Records", Case: "P::temps", Provenance: provenance(KindRun),
		Runs: []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "temps", Value: seqOf(realValue(300), realValue(300), realValue(341.2))}}}},
		Existing: Existing{Definition: true, Stem: "temps", Attributes: map[string]Feature{
			"temps": {TypeFQN: scalarValuesReal, Multi: true, Unique: true},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "declares temps unique but the run values repeat a value") {
		t.Fatalf("Generate = %v, want the unique repeated-value refusal", err)
	}
}

func TestGenerateExistingSequenceCompatibility(t *testing.T) {
	cases := map[string]struct {
		values []runtime.Value
		unique bool
	}{
		"unique values in unique member":      {values: []runtime.Value{realValue(300), realValue(310)}, unique: true},
		"repeated values in nonunique member": {values: []runtime.Value{realValue(300), realValue(300)}, unique: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := Generate(Request{
				Package: "Records", Case: "P::temps", Provenance: provenance(KindRun),
				Runs: []Run{{Spell: spell(), Outputs: []runtime.CalcOutputValue{{Name: "temps", Value: seqOf(tc.values...)}}}},
				Existing: Existing{Definition: true, Stem: "temps", Attributes: map[string]Feature{
					"temps": {TypeFQN: scalarValuesReal, Multi: true, Unique: tc.unique},
				}},
			})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if _, err := format.Source("<record>", []byte(res.Source), format.DefaultOptions); err != nil {
				t.Fatalf("generated source does not parse: %v", err)
			}
		})
	}
}
