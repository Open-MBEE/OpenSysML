package grpc

import "testing"

// A body's result expression and a `filter` member answer under the ids the conversion writes:
// the result's referent Membership beside the result, the ElementFilterMembership at the member.
func TestElementPropertiesReadBodyResultsAndFiltersAtTheConversionIDs(t *testing.T) {
	r := parseForElementProperties(t, `package P {
	calc def F;
	part def M { attribute mass; calc e : F { mass } }
	package Q { private import P::*; attribute keep : ScalarValues::Boolean; filter keep; }
}`)
	if got := propertyElementIDs(r.property("P__M__e___400", "ownedMembership").Values); len(got) == 0 || got[0] != "P__M__e___400_referent" {
		t.Errorf("the result's ownedMembership = %v, want its referent Membership P__M__e___400_referent first", got)
	}
	memberships := r.property("P__Q", "ownedMembership").Values
	var filters []string
	for _, value := range memberships {
		if value.GetElement().GetMetaclass() == "ElementFilterMembership" {
			filters = append(filters, value.GetElement().GetId())
		}
	}
	if len(filters) != 1 || filters[0] != "P__Q___402" {
		t.Errorf("Q's ElementFilterMemberships = %v, want the one filter member P__Q___402", filters)
	}
}
