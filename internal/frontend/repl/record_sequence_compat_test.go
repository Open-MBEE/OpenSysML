package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

func TestRecordAttributesReportsSequenceUniqueness(t *testing.T) {
	for name, declaration := range map[string]string{
		"unique":    "attribute temps : Real[0..*];",
		"nonunique": "attribute temps : Real[0..*] nonunique;",
	} {
		t.Run(name, func(t *testing.T) {
			s := NewSession()
			model := `package Records {
				private import ScalarValues::*;
				private import AnalysisRecords::*;
				part def Rec :> AnalysisRecords::AnalysisRun { ` + declaration + ` }
			}`
			if errs := errorDiagnostics(s.Submit(model).Diagnostics); len(errs) > 0 {
				t.Fatalf("model has errors: %v", errs)
			}
			idx := s.symbolIndex()
			defs := idx.LookupQualified("Records::Rec")
			if len(defs) != 1 {
				t.Fatalf("Records::Rec resolves to %d symbols", len(defs))
			}
			resolver := resolve.New(idx)
			sem := semantics.NewModel(resolver)
			resolver.SetModel(sem)
			feature, ok := recordAttributes(idx, sem, defs[0])["temps"]
			if !ok || !feature.Multi {
				t.Fatalf("temps = %+v (present %v), want a multi-valued feature", feature, ok)
			}
			wantUnique := name == "unique"
			if feature.Unique != wantUnique {
				t.Errorf("temps.Unique = %v, want %v", feature.Unique, wantUnique)
			}
		})
	}
}

func TestRecordAttributesFollowsInheritedNonunique(t *testing.T) {
	s := NewSession()
	model := `package Records {
		private import ScalarValues::*;
		private import AnalysisRecords::*;
		part def Base { attribute temps : Real[0..*] nonunique; }
		part def Rec :> Base, AnalysisRecords::AnalysisRun { attribute :>> temps; }
	}`
	if errs := errorDiagnostics(s.Submit(model).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	idx := s.symbolIndex()
	defs := idx.LookupQualified("Records::Rec")
	if len(defs) != 1 {
		t.Fatalf("Records::Rec resolves to %d symbols", len(defs))
	}
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	feature, ok := recordAttributes(idx, sem, defs[0])["temps"]
	if !ok || !feature.Multi || feature.Unique {
		t.Errorf("temps = %+v (present %v), want inherited multi nonunique feature", feature, ok)
	}
}
