package rdf

import (
	"regexp"
	"slices"
	"testing"
)

var idAlphabet = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestEncodeElementID(t *testing.T) {
	cases := map[string]string{
		"Vehicle":         "Vehicle",
		"Demo::Vehicle":   "Demo__Vehicle",
		"A::B::C":         "A__B__C",
		"A_B::C":          "A_5fB__C",
		"A::B_C":          "A__B_5fC",
		"Importer::@0":    "Importer___400",
		"Vehicle Mass":    "Vehicle_20Mass",
		"has:colon":       "has_3acolon",
		"has#hash":        "has_23hash",
		"has%percent":     "has_25percent",
		"kerb-weight":     "kerb-weight",
		"Fahrzeug::Größe": "Fahrzeug__Gr_c3_b6_c3_9fe",
		"日本::語":           "_e6_97_a5_e6_9c_ac___e8_aa_9e",
	}
	for qname, want := range cases {
		id := EncodeElementID(qname)
		if id != want {
			t.Errorf("EncodeElementID(%q) = %q, want %q", qname, id, want)
		}
		if !idAlphabet.MatchString(id) {
			t.Errorf("EncodeElementID(%q) = %q is outside [A-Za-z0-9_-]+", qname, id)
		}
		got, ok := DecodeElementID(id)
		if !ok || got != qname {
			t.Errorf("DecodeElementID(%q) = %q, %v, want %q", id, got, ok, qname)
		}
	}
}

// The two names differ only in where the underscore came from, so their ids
// colliding would merge two distinct elements.
func TestEncodeElementIDDoesNotCollide(t *testing.T) {
	names := []string{
		"A_B::C", "A::B_C", "A_B_C", "A::B::C",
		"A_5fB", "A__B", "A::_B", "A_::B", "A::5fB",
	}
	seen := map[string]string{}
	for _, qname := range names {
		id := EncodeElementID(qname)
		if other, dup := seen[id]; dup {
			t.Errorf("%q and %q both encode to %q", qname, other, id)
		}
		seen[id] = qname
	}
}

func TestDecodeElementIDRejectsMalformedIDs(t *testing.T) {
	for _, id := range []string{
		"",       // no name is encoded as an empty id
		"A_",     // trailing escape character
		"A_5",    // truncated escape
		"A_5F",   // uppercase hex
		"A_g0",   // non-hex escape
		"A B",    // byte outside the id alphabet
		"A::B",   // raw separator is never emitted
		"A%3AB",  // percent escapes are not this encoding
		"caf_e9", // 0xe9 alone is not valid UTF-8
		"_c3",    // truncated multi-byte sequence
	} {
		if got, ok := DecodeElementID(id); ok {
			t.Errorf("DecodeElementID(%q) = %q, want a rejection", id, got)
		}
	}
}

// A membership id is the member's id plus a marker no element id can carry, so
// it is reversible and can never name an element.
func TestOwningMembershipIDRoundTripsAndCannotCollide(t *testing.T) {
	for _, qname := range []string{
		"Vehicle", "Demo::Vehicle", "A_B::C", "Importer::@0", "日本::語", "A::B_om",
	} {
		id := OwningMembershipID(qname)
		if !idAlphabet.MatchString(id) {
			t.Errorf("OwningMembershipID(%q) = %q is outside [A-Za-z0-9_-]+", qname, id)
		}
		if got, ok := DecodeOwningMembershipID(id); !ok || got != qname {
			t.Errorf("DecodeOwningMembershipID(%q) = %q, %v, want %q", id, got, ok, qname)
		}
		if _, ok := DecodeElementID(id); ok {
			t.Errorf("%q decodes as an element id, so a membership can be mistaken for an element", id)
		}
		if other, ok := DecodeOwningMembershipID(EncodeElementID(qname)); ok {
			t.Errorf("the element id of %q reads as the membership of %q", qname, other)
		}
	}
}

// An expression node's id is its owner's id plus the positions leading to it, so
// it is reversible, valid where an element id is, and never names an element.
func TestExpressionNodeIDRoundTripsAndCannotCollide(t *testing.T) {
	for _, qname := range []string{
		"Vehicle", "Demo::Rover::wheels", "A_B::C", "日本::語", "A::B_pvalue",
		// A name segment starting with 'p' puts the separator inside the owner id.
		"A::part", "Demo::port::p", "P::p::pvalue",
	} {
		for _, positions := range [][]string{
			{"value"}, {"upperBound"}, {"condition", "a0"}, {"end0"}, {"a b"},
		} {
			id := EncodeElementID(qname)
			for _, position := range positions {
				id = ExpressionNodeID(id, position)
			}
			if !idAlphabet.MatchString(id) {
				t.Errorf("the id %q of %q at %v is outside [A-Za-z0-9_-]+", id, qname, positions)
			}
			owner, got, ok := DecodeExpressionNodeID(id)
			if !ok || owner != qname || !slices.Equal(got, positions) {
				t.Errorf("DecodeExpressionNodeID(%q) = %q, %v, %v, want %q, %v",
					id, owner, got, ok, qname, positions)
			}
			if _, ok := DecodeElementID(id); ok {
				t.Errorf("%q decodes as an element id, so a node can be mistaken for an element", id)
			}
			if _, ok := DecodeOwningMembershipID(id); ok {
				t.Errorf("%q decodes as a membership id", id)
			}
		}
		if _, _, ok := DecodeExpressionNodeID(EncodeElementID(qname)); ok {
			t.Errorf("the element id of %q reads as an expression node", qname)
		}
	}
}
