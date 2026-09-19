package passes

import "testing"

// The pilot rejects every occurrence, item or part whose types are not
// occurrence definitions, whether the type is declared or inherited through
// subsetting or a feature chain (OccurrenceUsage_invalid.sysml.xt).
func TestW8DOccurrenceMustBeTypedByOccurrenceDefinitions(t *testing.T) {
	src := `package pkg {
	attribute def Real;
	occurrence def A {
		occurrence areal : Real;
		occurrence avalue :> aValue;
		occurrence twoTypes : PartDef, Real;
	}
	attribute aValue : Real;
	part def PartDef;
	ref a : A;
	event a.areal;
}`
	w8dWantLines(t, src, "occurrence-usage-type", 4, 5, 6, 11)
}

// An event occurrence must reference an occurrence, not a reference usage.
func TestW8DEventMustReferenceAnOccurrence(t *testing.T) {
	src := `package pkg {
	occurrence def A;
	ref a : A;
	event a;
}`
	w8dWantLines(t, src, "event-reference-occurrence", 4)
}

// A legal model stays silent: occurrence definitions type occurrences, and an
// event references an occurrence usage.
func TestW8DLegalOccurrenceTypingsStaySilent(t *testing.T) {
	src := `package pkg {
	occurrence def A {
		occurrence inner : B;
	}
	occurrence def B;
	occurrence a : A;
	event a.inner;
	part def PartDef;
	part p : PartDef;
	event p;
}`
	if lines := w8dLines(t, src, "occurrence-usage-type"); len(lines) != 0 {
		t.Errorf("unexpected occurrence typing diagnostics at %v", lines)
	}
	if lines := w8dLines(t, src, "event-reference-occurrence"); len(lines) != 0 {
		t.Errorf("unexpected event reference diagnostics at %v", lines)
	}
}
