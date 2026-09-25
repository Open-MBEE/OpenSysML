package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A user stereotype specializing a standard one carries its meaning: the class
// is a requirement def with the inherited Id and Text, and the user-added tags
// become a metadata usage of the user stereotype.
func TestUserStereotypeSpecializingStandardOneClassifies(t *testing.T) {
	r := migrateFixtureFile(t, "org_profile")
	wantLine(t, r.Notation, "requirement def <'REQ-1'> 'Flow Requirement' {")
	wantLine(t, r.Notation, "doc /* The pump shall deliver 10 l/s. */")
	wantLine(t, r.Notation, "@'Org Profile'::'Org Requirement' {")
	wantLine(t, r.Notation, "criticality = 'Org Profile'::Criticality::high;")
	wantLine(t, r.Notation, "owner = Acme;")
	wantLine(t, r.Notation, "weight = 2.0;")
	wantNoLine(t, r.Notation, "Id = ")
	wantNoLine(t, r.Notation, "Text = \"The pump")
	wantLine(t, r.Notation, "requirement def <'REQ-2'> 'Safety Requirement' {")
	wantLine(t, r.Notation, "part def Pump {")
	wantLine(t, r.Notation, "attribute def FlowRate {")
	wantLine(t, r.Notation, "satisfy requirement : 'Flow Requirement' {")
	wantLine(t, r.Notation, "@'Org Profile'::'Org Satisfy' {")
	wantNoLine(t, r.Notation, "applied stereotype")
	for _, id := range []string{"_req_flow", "_req_safe", "_pump", "_flow", "_sat"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}

// User stereotypes become metadata defs in their profile's package, typed by
// their tag definitions; a stereotype specializing a user one specializes its def.
func TestUserProfilesBecomeMetadataDefs(t *testing.T) {
	r := migrateFixtureFile(t, "org_profile")
	wantLine(t, r.Notation, "package 'Org Profile' {")
	wantLine(t, r.Notation, "metadata def 'Org Requirement' {")
	wantLine(t, r.Notation, "attribute Rationale : ScalarValues::String;")
	wantLine(t, r.Notation, "attribute criticality : Criticality;")
	wantLine(t, r.Notation, "ref owner;")
	wantLine(t, r.Notation, "metadata def 'Key Requirement' :> 'Org Requirement' {")
	wantLine(t, r.Notation, "attribute notes : ScalarValues::String[0..*];")
	wantLine(t, r.Notation, "notes = (\"First pass.\", \"Second pass.\");")
	wantNoLine(t, r.Notation, "base_")
	wantNoLine(t, r.Notation, "no v2 form for a UML Stereotype")
	wantNoLine(t, r.Notation, "no v2 form for a UML Extension")
	if es := entriesFor(r, "_st_req_base"); len(es) != 1 || es[0].Verdict != migrate.Skipped || !strings.Contains(es[0].Note, "extension end") {
		t.Errorf("_st_req_base entries = %+v", es)
	}
}

// A user stereotype named like a standard one, without a standard general,
// carries none of its meaning: the class is a part def with a metadata usage.
func TestSameNamedUserStereotypeWithoutGeneralIsNotStandard(t *testing.T) {
	r := migrateFixtureFile(t, "org_profile")
	wantLine(t, r.Notation, "part def 'Design Note' {")
	wantLine(t, r.Notation, "@Legacy::Requirement {")
	wantLine(t, r.Notation, "Text = \"Kept for reference only.\";")
	wantNoLine(t, r.Notation, "requirement def 'Design Note'")
	if es := entriesFor(r, "_note"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "without «Block»") {
		t.Errorf("_note entries = %+v", es)
	}
}

// Generalization is followed through several generals and a diamond, from
// Papyrus pathmap hrefs, and user-stereotyped relationships take the v2 form
// of the standard relationship they specialize.
func TestInheritedStereotypeSemanticsFollowEveryGeneral(t *testing.T) {
	r := migrateFixtureFile(t, "profile_inheritance")
	wantLine(t, r.Notation, "metadata def Diamond :> Left, Right;")
	wantLine(t, r.Notation, "part def Gear {")
	wantLine(t, r.Notation, "@Tailoring::Diamond {")
	wantLine(t, r.Notation, "attribute def Torque {")
	wantLine(t, r.Notation, "requirement def <'R-1'> 'Torque Requirement' {")
	wantLine(t, r.Notation, "verify requirement : 'Torque Requirement' {")
	wantLine(t, r.Notation, "@ModelingMetadata::Refinement;")
	wantLine(t, r.Notation, "@Tailoring::'Detailed By';")
	wantLine(t, r.Notation, "connection def 'Derive Speed Requirement' :> RequirementDerivation::Derivation {")
	wantLine(t, r.Notation, "allocate Spin to Shaft {")
	wantLine(t, r.Notation, "@Tailoring::'Assigned To';")
	wantNoLine(t, r.Notation, "applied stereotype")
	for _, id := range []string{"_gear", "_torque", "_req_torque", "_verify", "_refine", "_derive", "_alloc"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Mapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	if es := entriesFor(r, "_trace"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "plain dependency") {
		t.Errorf("_trace entries = %+v", es)
	}
}

// A generalization cycle is written less its closing edge, approximating, and a
// general outside the document or unresolved is reported, not pretended resolved.
func TestGeneralizationCyclesAndUnresolvedGeneralsAreReported(t *testing.T) {
	r := migrateFixtureFile(t, "profile_inheritance")
	wantLine(t, r.Notation, "metadata def 'Ring A' :> 'Ring B';")
	wantLine(t, r.Notation, "metadata def 'Ring B';")
	wantLine(t, r.Notation, "metadata def Lost;")
	wantLine(t, r.Notation, "metadata def Foreign;")
	wantLine(t, r.Notation, "part def 'Lost Thing' {")
	wantLine(t, r.Notation, "@Tailoring::Lost;")
	if es := entriesFor(r, "_st_ring_b"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "closes a cycle") {
		t.Errorf("_st_ring_b entries = %+v", es)
	}
	if es := entriesFor(r, "_st_lost"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "refers to nothing in the document") {
		t.Errorf("_st_lost entries = %+v", es)
	}
	if es := entriesFor(r, "_st_foreign"); len(es) != 1 || es[0].Verdict != migrate.Approximated || !strings.Contains(es[0].Note, "Shared_Profile.xmi#_shared_req is not written") {
		t.Errorf("_st_foreign entries = %+v", es)
	}
}

// The modeling tool's own profiles are known by exact namespace path: their
// content is skipped with a reason naming what it is, while a user profile
// hosted under the tool's domain is written as metadata. A classifier the tool
// also draws as a mockup stays the model's when it behaves or is a «Block».
func TestToolProfilesAreSkippedByExactPathOnly(t *testing.T) {
	r := migrateFixtureFile(t, "tool_profiles")
	wantLine(t, r.Notation, "package 'My Profile' {")
	wantLine(t, r.Notation, "metadata def Tracked {")
	wantLine(t, r.Notation, "@'My Profile'::Tracked {")
	wantLine(t, r.Notation, "ticket = \"T-42\";")
	wantLine(t, r.Notation, "package Customizations;")
	wantLine(t, r.Notation, "package Mockups {")
	wantLine(t, r.Notation, "part def Clock {")
	wantLine(t, r.Notation, "exhibit state ticking : Ticking;")
	wantLine(t, r.Notation, "/* applied stereotype «Frame»: title = Clock */")
	wantLine(t, r.Notation, "part def Gauge {")
	wantLine(t, r.Notation, "@Simulation::Configuration;")
	wantNoLine(t, r.Notation, "Pump customization")
	wantNoLine(t, r.Notation, "Start button")
	wantNoLine(t, r.Notation, "Status label")
	wantNoLine(t, r.Notation, "Sequence generator")
	wantNoLine(t, r.Notation, "owning system")
	skipped := map[string]string{
		"_c_pump":     "specification-dialog customization",
		"_pump_owner": "specification-dialog customization",
		"_ui_group":   "UI prototyping mockup",
		"_ui_button":  "UI prototyping mockup",
		"_ui_label":   "UI prototyping mockup",
		"_seq":        "simulation tool's configuration",
	}
	for id, reason := range skipped {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict != migrate.Skipped || !strings.Contains(es[0].Note, reason) {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
	for _, id := range []string{"_my", "_st_tracked", "_pump", "_cfg", "_ui_clock", "_ui_clock_sm", "_ui_gauge"} {
		if es := entriesFor(r, id); len(es) != 1 || es[0].Verdict == migrate.Skipped || es[0].Verdict == migrate.Unmapped {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}
