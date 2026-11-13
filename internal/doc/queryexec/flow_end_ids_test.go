package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestFlowEndsHaveTheirOwnElementIDs: a `flow`'s FlowEnds, their EndFeatureMemberships and a
// chained end's ReferenceSubsetting are identified as the conversion mints them (end0/end1).
func TestFlowEndsHaveTheirOwnElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def PX { attribute x; }
	part p1 : PX;
	part p2 : PX;
	flow f from p1.x to p2.x;
	flow g from p1 to p2;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	for _, flow := range []string{"f", "g"} {
		ends, _ := fixture.model.ReflectiveElements(fixture.symbol(t, flow), "ownedEndFeature")
		if len(ends) != 2 {
			t.Fatalf("%s: ownedEndFeature = %d ends, want 2", flow, len(ends))
		}
		for i, end := range ends {
			slot := "_pend" + string(rune('0'+i))
			if id := elementID(end); !strings.HasSuffix(id, "__"+flow+slot) {
				t.Errorf("%s end %d id = %q, want its flow's at %s", flow, i, id, slot)
			}
			memberships, _ := fixture.model.ReflectiveElements(end, "owningMembership")
			if len(memberships) != 1 || elementID(memberships[0]) != elementID(end)+"_om" {
				t.Errorf("%s end %d owning membership = %v, want %s_om", flow, i, memberships, elementID(end))
			}
			relationships := fixture.model.ImplicitRelationships(end)
			switch {
			case flow == "g" && len(relationships) != 0:
				t.Errorf("g end %d owns %d relationships, want none for a bare name", i, len(relationships))
			case flow == "f" && (len(relationships) != 1 || elementID(relationships[0]) != elementID(end)+"_prs"):
				t.Errorf("f end %d relationships = %v, want its ReferenceSubsetting at %s_prs", i, relationships, elementID(end))
			}
		}
	}
}

// TestFilterConditionHasItsMembersElementID: a `filter` condition is identified under the
// ElementFilterMembership that owns it, at `filter`, and the membership as that member.
func TestFilterConditionHasItsMembersElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	metadata def M;
	filter @M;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	pkg := symbols.PreferDeclared(fixture.index.LookupQualified("Observatory"))
	if len(pkg) != 1 {
		t.Fatalf("lookup Observatory: got %d symbols", len(pkg))
	}
	members, _ := fixture.model.ReflectiveElements(pkg[0], "ownedMember")
	var condition *symbols.Symbol
	for _, member := range members {
		if member.Expr != nil && member.Expr.Role == symbols.ExpressionFilterCondition {
			condition = member
		}
	}
	if condition == nil {
		t.Fatalf("Observatory owns no filter condition among %d members", len(members))
	}
	id, err := ElementIDOf(read, fixture.index, condition)
	if err != nil || !strings.HasSuffix(id, "_pfilter") {
		t.Fatalf("condition id = %q, %v; want the member's at _pfilter", id, err)
	}
	memberships, _ := fixture.model.ReflectiveElements(condition, "owningMembership")
	if len(memberships) != 1 {
		t.Fatalf("condition owning membership = %v, want one", memberships)
	}
	if got, _ := ElementIDOf(read, fixture.index, memberships[0]); got != strings.TrimSuffix(id, "_pfilter") {
		t.Errorf("ElementFilterMembership id = %q, want %q", got, strings.TrimSuffix(id, "_pfilter"))
	}
}
