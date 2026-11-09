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
