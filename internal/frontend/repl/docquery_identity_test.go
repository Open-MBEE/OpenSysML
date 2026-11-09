package repl

import (
	"strings"
	"testing"

	_ "github.com/Open-MBEE/OpenSysML/internal/doc/queryexec/elementids"
)

// metadataBodyQueryModel restates a metadata definition's feature in an annotation body —
// an element with a name but no qualifiedName, since the usage owning it is anonymous.
const metadataBodyQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	metadata def Heritage { attribute mission : String; }
	part def Telescope;
	part telescope : Telescope;
	@Heritage about telescope { mission = "Hubble"; }
	calc def BodyFeatures :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "ReferenceUsage", exact = true),
			properties = ("elementId", "owningRelationship")
		)
	}
	calc def BodyMemberships :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "FeatureMembership", exact = true),
			properties = ("elementId", "memberElement", "target")
		)
	}
}`

// TestRunQueryTableSpellsAnElementWithoutAQualifiedNameByItsElementID: the table spells a
// metadata body feature — named, but owned by an anonymous usage — by its elementId, as the
// row's own key and as the end of the membership that owns it; the conversion knows no other.
func TestRunQueryTableSpellsAnElementWithoutAQualifiedNameByItsElementID(t *testing.T) {
	s := NewSession()
	if res := s.Submit(metadataBodyQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	features := s.RunDocumentQuery("BodyFeatures root=Observatory")
	if !features.Holds() || features.Table == nil {
		t.Fatalf("BodyFeatures verdict = %s: %v", features.Status, features.Lines)
	}
	var featureID, membershipID string
	for _, row := range features.Table.Rows {
		id := strings.Trim(cellText(row.Cells, 0), `"`)
		if !strings.HasSuffix(id, "__mission") {
			continue
		}
		featureID, membershipID = id, cellText(row.Cells, 1)
		if row.Element != featureID {
			t.Errorf("body feature row keyed %q, want its elementId %q", row.Element, featureID)
		}
	}
	if featureID == "" || !strings.Contains(featureID, "___") {
		t.Fatalf("no body feature row with a positional elementId; rows %v", features.Table.Rows)
	}
	if membershipID != featureID+"_om" {
		t.Fatalf("body feature owningRelationship %q, want %q", membershipID, featureID+"_om")
	}
	memberships := s.RunDocumentQuery("BodyMemberships root=Observatory")
	if !memberships.Holds() || memberships.Table == nil {
		t.Fatalf("BodyMemberships verdict = %s: %v", memberships.Status, memberships.Lines)
	}
	checked := false
	for _, row := range memberships.Table.Rows {
		if strings.Trim(cellText(row.Cells, 0), `"`) != membershipID {
			continue
		}
		checked = true
		for i, property := range []string{"memberElement", "target"} {
			if got := cellText(row.Cells, i+1); got != featureID {
				t.Errorf("%s of %s = %q, want the body feature's elementId %q", property, membershipID, got, featureID)
			}
		}
	}
	if !checked {
		t.Fatalf("no row for %s; rows %v", membershipID, memberships.Table.Rows)
	}
}

func cellText(cells [][]string, i int) string {
	if i >= len(cells) || len(cells[i]) != 1 {
		return ""
	}
	return cells[i][0]
}

// effectiveNameQueryModel has two elements the conversion names through an effective name: a calc
// def's nameless `return` parameter (`result`, the feature it implicitly redefines) and the
// requirement a nameless objective (`obj`) verifies, owned under it.
const effectiveNameQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def ExposureTime { in flux : Real; return : Real = 1.0 / flux; }
	requirement massRequirement;
	verification def ThroughputTest { objective { verify massRequirement; } }
	part { part wheel : Wheel; }
	part def Wheel;
	calc def Typings :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "FeatureTyping", exact = true),
			properties = ("elementId", "typedFeature")
		)
	}
	calc def Verifications :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "RequirementVerificationMembership", exact = true),
			properties = ("elementId", "memberElement", "owningRelatedElement")
		)
	}
}`

// TestRunQueryTableSpellsAnEffectivelyNamedElementByItsQualifiedName: the table spells an element
// by the qualified name the conversion derives for it — through an effective name as through a
// declared one, as the RDF writer states its qualifiedName — and by elementId only where the
// conversion addresses it by position.
func TestRunQueryTableSpellsAnEffectivelyNamedElementByItsQualifiedName(t *testing.T) {
	s := NewSession()
	if res := s.Submit(effectiveNameQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	typings := s.RunDocumentQuery("Typings root=Observatory")
	if !typings.Holds() || typings.Table == nil {
		t.Fatalf("Typings verdict = %s: %v", typings.Status, typings.Lines)
	}
	checked, wheel := false, false
	for _, row := range typings.Table.Rows {
		switch id := strings.Trim(cellText(row.Cells, 0), `"`); {
		case id == "Observatory__ExposureTime__result_ft0":
			checked = true
			if got := cellText(row.Cells, 1); got != "Observatory::ExposureTime::result" {
				t.Errorf("typedFeature of the return parameter's typing = %q, want its qualified name through the effective name result", got)
			}
		case strings.HasSuffix(id, "__wheel_ft0"):
			// A named part under an anonymous part has no qualifiedName: the conversion addresses it by
			// position, and the table keeps spelling it by that elementId, never as `Observatory::wheel`.
			wheel = true
			if got, want := cellText(row.Cells, 1), strings.TrimSuffix(id, "_ft0"); got != want || !strings.Contains(got, "___") {
				t.Errorf("typedFeature of wheel's typing = %q, want its positional elementId %q", got, want)
			}
		}
	}
	if !checked || !wheel {
		t.Fatalf("missing the return parameter's (%v) or wheel's (%v) FeatureTyping row; rows %v", checked, wheel, typings.Table.Rows)
	}
	verifications := s.RunDocumentQuery("Verifications root=Observatory")
	if !verifications.Holds() || verifications.Table == nil {
		t.Fatalf("Verifications verdict = %s: %v", verifications.Status, verifications.Lines)
	}
	if n := len(verifications.Table.Rows); n != 1 {
		t.Fatalf("RequirementVerificationMembership rows = %d, want 1: %v", n, verifications.Table.Rows)
	}
	row := verifications.Table.Rows[0]
	if got := cellText(row.Cells, 1); got != "Observatory::ThroughputTest::obj::massRequirement" {
		t.Errorf("memberElement = %q, want the verified requirement under the objective's effective name obj", got)
	}
	if got := cellText(row.Cells, 2); got != "Observatory::ThroughputTest::obj" {
		t.Errorf("owningRelatedElement = %q, want the objective by its effective name", got)
	}
}
