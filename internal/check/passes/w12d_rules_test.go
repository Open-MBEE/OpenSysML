package passes

import "testing"

// Type::multiplicity is single-valued (KerML 8.3.3.1.1), so a second
// multiplicity member is a validation error, not a syntax error.
func TestW12DOnlyOneMultiplicity(t *testing.T) {
	src := `package Type_Multiplicity {
	classifier C {
		multiplicity subsets Base::zeroOrOne;

		multiplicity subsets Base::zeroToMany;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgOnlyOneMultiplicity) != 1 {
		t.Errorf("want one %q, got %v", msgOnlyOneMultiplicity, msgs)
	}
}

// One multiplicity member is what a type may own, so it reports nothing.
func TestW12DOneMultiplicityIsLegal(t *testing.T) {
	src := `package Type_Multiplicity {
	classifier C {
		multiplicity subsets Base::zeroOrOne;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgOnlyOneMultiplicity) != 0 {
		t.Errorf("want no %q, got %v", msgOnlyOneMultiplicity, msgs)
	}
}

// An end that owns its cross feature inline and also crosses another one
// declares two cross features, and only one is allowed (KerML 8.3.4.5).
func TestW12DDeclaredCrossFeature(t *testing.T) {
	src := `package P {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; }
	assoc A3 {
		end x1 [0..1] feature x : C1 crosses y.b {
			public import y::y1;
		}
		end feature y : C2 crosses x.y1 {
			member feature y1 [0..1] featured by C1;
		}
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgMustBeCrossFeature) != 1 {
		t.Errorf("want one %q, got %v", msgMustBeCrossFeature, msgs)
	}
}

// An end that only crosses does not report the rule.
func TestW12DCrossesAloneIsLegal(t *testing.T) {
	src := `package P {
	class C1 { feature a : C2; }
	class C2 { feature b : C1; }
	assoc A1 {
		end x : C1 crosses y.b;
		end y : C2 crosses x.a;
	}
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if w8cCount(msgs, msgMustBeCrossFeature) != 0 {
		t.Errorf("want no %q, got %v", msgMustBeCrossFeature, msgs)
	}
}
