package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// memberModel exercises a usage inheriting members through its typing, a
// private member, a redefinition and a named binding.
const memberModel = `
package Demo {
    port def CmdPort;
    part def Logic { port cmdIn : CmdPort; }
    part def Controller {
        attribute gain : ScalarValues::Real;
        private attribute secret : ScalarValues::Real;
        port cmdIn : CmdPort;
        part logic : Logic;
        binding delegate bind cmdIn = logic.cmdIn;
    }
    part system {
        part controller : Controller;
        part tuned : Controller { attribute :>> gain = 2.0; }
    }
}
`

func memberIDs(members []*pb.MemberInfo) string {
	var ids []string
	for _, m := range members {
		ids = append(ids, m.GetId())
	}
	return strings.Join(ids, ",")
}

// TestMembersAreReportedWithProvenance verifies a usage reports the members
// its type declares as inherited from it, its private member left out, while
// child_ids stays the owned members, and the library members are counted.
func TestMembersAreReportedWithProvenance(t *testing.T) {
	get := parseForFacts(t, memberModel)
	controller := get("Demo::system::controller")
	if len(controller.ChildIds) != 0 {
		t.Errorf("child_ids = %v, want none", controller.ChildIds)
	}
	want := "Demo::Controller::gain,Demo::Controller::cmdIn,Demo::Controller::logic,Demo::Controller::delegate"
	if got := memberIDs(controller.Members); got != want {
		t.Errorf("members = %s, want %s", got, want)
	}
	for _, m := range controller.Members {
		if !m.Inherited || m.InheritedFrom != "Demo::Controller" || m.Library {
			t.Errorf("%s: inherited %v from %q library %v", m.Id, m.Inherited, m.InheritedFrom, m.Library)
		}
	}
	if controller.WithheldLibraryMembers == 0 {
		t.Error("library members are absent but not counted")
	}

	definition := get("Demo::Controller")
	for _, m := range definition.Members {
		if strings.HasPrefix(m.Id, "Demo::Controller::") && (m.Inherited || m.InheritedFrom != "") {
			t.Errorf("own member %s reported as inherited from %q", m.Id, m.InheritedFrom)
		}
	}
	if got := memberIDs(definition.Members); !strings.Contains(got, "Demo::Controller::secret") {
		t.Errorf("the definition's own private member is missing: %s", got)
	}

	tuned := get("Demo::system::tuned")
	want = "Demo::system::tuned::gain,Demo::Controller::cmdIn,Demo::Controller::logic,Demo::Controller::delegate"
	if got := memberIDs(tuned.Members); got != want {
		t.Errorf("tuned members = %s, want %s", got, want)
	}
}

// TestLibraryMembersListedOnRequest verifies include_library_members lists
// what is otherwise counted, each marked library with its provenance.
func TestLibraryMembersListedOnRequest(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: memberModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	get := func(include bool) *pb.SymbolInfo {
		resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
			ModelHash: parsed.ModelHash, SymbolId: "Demo::system::controller", IncludeLibraryMembers: include,
		})
		if err != nil || resp.Symbol == nil {
			t.Fatalf("GetSymbol: %v %s", err, resp.GetError())
		}
		return resp.Symbol
	}
	withheld := get(false).WithheldLibraryMembers
	full := get(true)
	if full.WithheldLibraryMembers != 0 {
		t.Errorf("withheld = %d with library members included", full.WithheldLibraryMembers)
	}
	var library int32
	for _, m := range full.Members {
		if m.Library {
			library++
			if !m.Inherited || m.InheritedFrom == "" {
				t.Errorf("library member %s without provenance", m.Id)
			}
		}
	}
	if library != withheld {
		t.Errorf("%d library members listed, %d were withheld", library, withheld)
	}
}

// TestMembersWithheldWithoutCapability verifies a build without symbol_members
// reports neither the members nor the count, so a client cannot mistake the
// absence for an element without members.
func TestMembersWithheldWithoutCapability(t *testing.T) {
	srv, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", []string{CapabilitySymbolMembers})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: memberModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
		ModelHash: parsed.ModelHash, SymbolId: "Demo::system::controller",
	})
	if err != nil || resp.Symbol == nil {
		t.Fatalf("GetSymbol: %v %s", err, resp.GetError())
	}
	if len(resp.Symbol.Members) != 0 || resp.Symbol.WithheldLibraryMembers != 0 {
		t.Errorf("members = %v (withheld %d), want none without the capability", resp.Symbol.Members, resp.Symbol.WithheldLibraryMembers)
	}
}

// TestResolveNameIgnoredWithoutCapability verifies a build without symbol_name_resolution reads
// symbol_id as a reported id, as a service predating resolve_name does, and does not advertise it.
func TestResolveNameIgnoredWithoutCapability(t *testing.T) {
	const model = `package Vis { package Lib { part def Pump; alias PumpAlias for Pump; } }`
	for _, tc := range []struct {
		srv  *Service
		want string
	}{
		{mustNewService(t, 10), "Vis::Lib::Pump"},
		{mustNewServiceWithout(t, CapabilitySymbolNameResolution), "Vis::Lib::PumpAlias"},
	} {
		parsed, err := tc.srv.ParseFile(context.Background(), &pb.ParseFileRequest{
			Source: &pb.ParseFileRequest_Content{Content: model},
		})
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		resp, err := tc.srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
			ModelHash: parsed.ModelHash, SymbolId: "Vis::Lib::PumpAlias", ResolveName: true,
		})
		if err != nil || resp.Symbol == nil {
			t.Fatalf("GetSymbol: %v %s", err, resp.GetError())
		}
		if resp.Symbol.Id != tc.want {
			t.Errorf("GetSymbol(Vis::Lib::PumpAlias, resolve_name) = %s, want %s", resp.Symbol.Id, tc.want)
		}
	}
}

// TestResolveNameAnswersGeneratedIDs verifies resolve_name still reads the ids no name spells:
// a positional id and an unnamed connector end's element id, as a client following child_ids sends.
func TestResolveNameAnswersGeneratedIDs(t *testing.T) {
	const model = `package Gen { part def V { part q; part r; connection c connect q to r; bind q = r; } }`
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, tc := range []struct{ id, kind string }{
		{"Gen::V::@3", "bindingConnectorAsUsage"},
		{"Gen__V__c_pend0", "connectorEnd"},
		{"Gen__V___403_pend1", "connectorEnd"},
	} {
		for _, resolve := range []bool{false, true} {
			resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
				ModelHash: parsed.ModelHash, SymbolId: tc.id, ResolveName: resolve,
			})
			if err != nil || resp.Symbol == nil {
				t.Fatalf("GetSymbol(%s, resolve_name=%t): %v %s", tc.id, resolve, err, resp.GetError())
			}
			if resp.Symbol.Id != tc.id || resp.Symbol.Kind != tc.kind {
				t.Errorf("GetSymbol(%s, resolve_name=%t) = %s %s, want kind %s",
					tc.id, resolve, resp.Symbol.Id, resp.Symbol.Kind, tc.kind)
			}
		}
	}
}
