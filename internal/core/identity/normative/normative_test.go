package normative

import "testing"

// The ids the pilot's sysml.library.xmi carries for these elements.
const (
	scalarValuesID   = "40bb440c-5036-58e1-8675-5afccb8b8f1d"
	realID           = "14c0aa22-5489-59b5-b438-ded26e83ba31"
	realMembershipID = "ab72a695-5fe9-58a3-9d48-9e9a8711862d"
)

func TestNamespaceURL(t *testing.T) {
	if got := NamespaceURL.String(); got != "6ba7b811-9dad-11d1-80b4-00c04fd430c8" {
		t.Fatalf("NamespaceURL = %s", got)
	}
}

func TestPackageID(t *testing.T) {
	if got := ElementID(KerML, "ScalarValues"); got != scalarValuesID {
		t.Errorf("ElementID(KerML, ScalarValues) = %s, want %s", got, scalarValuesID)
	}
	// The root namespace's membership of the package hashes under the package.
	if got := OwningMembershipID(KerML, "ScalarValues"); got != "1cecd337-acef-524c-847a-1f3e7a2e65e9" {
		t.Errorf("OwningMembershipID(KerML, ScalarValues) = %q, want 1cecd337-acef-524c-847a-1f3e7a2e65e9", got)
	}
}

func TestDescendantID(t *testing.T) {
	if got := ElementID(KerML, "ScalarValues::Real"); got != realID {
		t.Errorf("ElementID(KerML, ScalarValues::Real) = %s, want %s", got, realID)
	}
	if got := OwningMembershipID(KerML, "ScalarValues::Real"); got != realMembershipID {
		t.Errorf("OwningMembershipID(KerML, ScalarValues::Real) = %s, want %s", got, realMembershipID)
	}
	// The membership hashes the element's path plus a suffix in the package's
	// name space, so a pilot-style recomputation lands on the same id.
	pkg := uuid5(NamespaceURL, KerML.Prefix()+"ScalarValues")
	if want := uuid5(pkg, "ScalarValues::Real/owningMembership").String(); want != realMembershipID {
		t.Errorf("recomputed membership = %s, want %s", want, realMembershipID)
	}
}

func TestLanguagesDiffer(t *testing.T) {
	kerml, sysml := ElementID(KerML, "Parts::Part"), ElementID(SysML, "Parts::Part")
	if kerml == sysml {
		t.Fatalf("the two prefixes derive one id %s", kerml)
	}
	if om := OwningMembershipID(SysML, "Parts::Part"); om == sysml || om == OwningMembershipID(KerML, "Parts::Part") {
		t.Errorf("the owning membership id %s is not its own", om)
	}
}

func TestEmptyAndUnknown(t *testing.T) {
	if got := ElementID(KerML, ""); got != "" {
		t.Errorf("ElementID(KerML, \"\") = %q", got)
	}
	if got := ElementID(0, "ScalarValues"); got != "" {
		t.Errorf("ElementID(0, ScalarValues) = %q", got)
	}
	if got := OwningMembershipID(KerML, ""); got != "" {
		t.Errorf("OwningMembershipID(KerML, \"\") = %q", got)
	}
}

func TestEscapeName(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"Real":             "Real",
		"_x1":              "_x1",
		"1x":               "'1x'",
		"==":               "'=='",
		"Vehicle Mass":     "'Vehicle Mass'",
		"Péclet":           "'Péclet'",
		"$":                "$",
		"a'b":              `'a\'b'`,
		`a"b`:              `'a\"b'`,
		`a\b`:              `'a\\b'`,
		"a\tb\nc\rd":       `'a\tb\nc\rd'`,
		"a\bb\fc":          `'a\bb\fc'`,
		"ISQ::Length":      "'ISQ::Length'",
		"Kernel Libraries": "'Kernel Libraries'",
	}
	for name, want := range cases {
		if got := EscapeName(name); got != want {
			t.Errorf("EscapeName(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestEscapedSegmentsHash(t *testing.T) {
	// A quoted segment hashes in its quoted spelling, as the norm's
	// qualified names carry it.
	quoted := ElementID(KerML, "BaseFunctions::==")
	pkg := uuid5(NamespaceURL, KerML.Prefix()+"BaseFunctions")
	if want := uuid5(pkg, "BaseFunctions::'=='").String(); quoted != want {
		t.Errorf("ElementID(BaseFunctions::==) = %s, want %s", quoted, want)
	}
}
