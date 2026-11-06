package symbolfacts

import (
	"strings"
	"testing"
)

// memberModel exercises own, inherited, redefined, private and anonymous
// members, and a usage inheriting through its typing.
const memberModel = `
package Demo {
    port def CmdPort;
    part def Logic { port cmdIn : CmdPort; }
    part def Base {
        attribute gain : ScalarValues::Real;
        private attribute secret : ScalarValues::Real;
        part logic : Logic;
    }
    part def Controller :> Base {
        port cmdIn : CmdPort;
        binding delegate bind cmdIn = logic.cmdIn;
        bind cmdIn = logic.cmdIn;
    }
    part system {
        part controller : Controller;
        part tuned : Controller { attribute :>> gain = 2.0; }
    }
}
`

func memberIDs(members []*Member) string {
	var ids []string
	for _, m := range members {
		ids = append(ids, m.Id)
	}
	return strings.Join(ids, ",")
}

func memberFacts(t *testing.T, id string) ([]*Member, int32) {
	t.Helper()
	idx, sc := modelWithLibrary(t, memberModel)
	syms := LookupNamed(idx, id)
	if len(syms) != 1 {
		t.Fatalf("%s resolves to %d symbols", id, len(syms))
	}
	info := Of(syms[0], sc)
	return info.Members, info.WithheldLibraryMembers
}

// Own members come first in declaration order, the anonymous binding among
// them; inherited members follow, each naming the type declaring it, a
// private member of the supertype is not inherited, and the library members
// every part has are withheld and counted rather than dropped.
func TestMembersAreOwnThenInheritedWithProvenance(t *testing.T) {
	members, withheld := memberFacts(t, "Demo::Controller")
	want := "Demo::Controller::cmdIn,Demo::Controller::delegate,Demo::Controller::,Demo::Base::gain,Demo::Base::logic"
	if got := memberIDs(members); got != want {
		t.Errorf("members = %s, want %s", got, want)
	}
	for _, m := range members {
		inherited := strings.HasPrefix(m.Id, "Demo::Base::")
		if m.Inherited != inherited {
			t.Errorf("%s: inherited = %v, want %v", m.Id, m.Inherited, inherited)
		}
		from := ""
		if inherited {
			from = "Demo::Base"
		}
		if m.InheritedFrom != from {
			t.Errorf("%s: inherited_from = %q, want %q", m.Id, m.InheritedFrom, from)
		}
		if m.Library {
			t.Errorf("%s is reported as a library member", m.Id)
		}
	}
	if withheld == 0 {
		t.Error("the library members a part inherits are absent but not counted, which is silence")
	}
}

// A usage typed by a definition inherits the definition's members through the
// typing, bindings included, while its child ids stay its own members.
func TestUsageMembersIncludeWhatItsTypeDeclares(t *testing.T) {
	idx, sc := modelWithLibrary(t, memberModel)
	info := Of(LookupNamed(idx, "Demo::system::controller")[0], sc)
	if len(info.ChildIds) != 0 {
		t.Errorf("child ids = %v, want none: the usage declares nothing", info.ChildIds)
	}
	want := "Demo::Controller::cmdIn,Demo::Controller::delegate,Demo::Controller::,Demo::Base::gain,Demo::Base::logic"
	if got := memberIDs(info.Members); got != want {
		t.Errorf("members = %s, want %s", got, want)
	}
	for _, m := range info.Members {
		if !m.Inherited {
			t.Errorf("%s is reported as own", m.Id)
		}
	}
	if from := info.Members[0].InheritedFrom; from != "Demo::Controller" {
		t.Errorf("cmdIn inherited from %q, want Demo::Controller", from)
	}
	if from := info.Members[3].InheritedFrom; from != "Demo::Base" {
		t.Errorf("gain inherited from %q, want Demo::Base", from)
	}
}

// A redefinition masks the member it redefines: the redefining member is the
// usage's own, and the redefined one is not inherited.
func TestRedefinitionMasksTheRedefinedMember(t *testing.T) {
	members, _ := memberFacts(t, "Demo::system::tuned")
	want := "Demo::system::tuned::gain,Demo::Controller::cmdIn,Demo::Controller::delegate,Demo::Controller::,Demo::Base::logic"
	if got := memberIDs(members); got != want {
		t.Errorf("members = %s, want %s", got, want)
	}
	if members[0].Inherited || members[0].InheritedFrom != "" {
		t.Errorf("the redefining gain is reported as inherited from %q", members[0].InheritedFrom)
	}
}

// Asked for, the library members are listed and marked, and none is counted
// as withheld.
func TestLibraryMembersAreListedOnRequest(t *testing.T) {
	idx, sc := modelWithLibrary(t, memberModel)
	sym := LookupNamed(idx, "Demo::Controller")[0]
	withheld := Of(sym, sc).WithheldLibraryMembers
	info := OfWith(sym, sc, Options{IncludeLibraryMembers: true})
	if info.WithheldLibraryMembers != 0 {
		t.Errorf("withheld = %d with library members included", info.WithheldLibraryMembers)
	}
	var library int32
	for _, m := range info.Members {
		if m.Library {
			library++
			if !m.Inherited || m.InheritedFrom == "" {
				t.Errorf("library member %s has no provenance", m.Id)
			}
		}
	}
	if library != withheld {
		t.Errorf("%d library members listed, %d withheld before", library, withheld)
	}
}

// A library element asked about directly reports what it declares, as its own.
func TestOwnMembersOfALibraryElementAreReported(t *testing.T) {
	members, _ := memberFacts(t, "Parts::Part")
	own := 0
	for _, m := range members {
		if !m.Inherited {
			own++
			if !m.Library {
				t.Errorf("%s is not marked a library member", m.Id)
			}
		}
	}
	if own == 0 {
		t.Error("Parts::Part reports none of the members it declares")
	}
}
