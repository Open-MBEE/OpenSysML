package grpc_test

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// memberDirectionModel reaches a port definition's features through ordinary and conjugated
// typings, a specializing definition, redefining and subsetting ports, and a nested port.
const memberDirectionModel = `package D {
	private import ScalarValues::*;
	port def CmdPort {
		out attribute cmd : Integer;
		in attribute ack : Boolean;
		inout attribute sync : Boolean;
		attribute note : String;
	}
	port def SubPort :> CmdPort;
	port def Outer { port inner : CmdPort; }
	part def Device {
		port pOut : CmdPort;
		port pIn : ~CmdPort;
		port pSub : ~SubPort;
		port pSubset :> pIn;
		port pOuter : ~Outer;
	}
	part def SubDevice :> Device { port pRedefined :>> pIn; }
	part def Rack { part slot : Device; }
	part device : Device;
	part sub : SubDevice;
	part rack : Rack;
}`

func memberDirection(t *testing.T, srv *grpc.Service, modelHash, id, memberOf string) string {
	t.Helper()
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: modelHash, SymbolId: id, MemberOf: memberOf})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("GetSymbol(%s, member_of=%s): %v %s", id, memberOf, err, resp.GetError())
	}
	return resp.GetSymbol().GetMetadata()["direction"]
}

func memberIDs(t *testing.T, srv *grpc.Service, modelHash, id string) []string {
	t.Helper()
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: modelHash, SymbolId: id})
	if err != nil || resp.GetError() != "" {
		t.Fatalf("GetSymbol(%s): %v %s", id, err, resp.GetError())
	}
	var ids []string
	for _, member := range resp.GetSymbol().GetMembers() {
		ids = append(ids, member.GetId())
	}
	return ids
}

// A member's Symbol direction is Type::directionOf relative to the type it is read through
// (KerML 8.3.3.1.10, SysML v2 7.13.3), the answer Type::input and Type::output give.
func TestGRPCMemberDirectionFollowsConjugation(t *testing.T) {
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: memberDirectionModel}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	hash := parsed.ModelHash
	const cmd, ack, sync, note = "D::CmdPort::cmd", "D::CmdPort::ack", "D::CmdPort::sync", "D::CmdPort::note"

	original := map[string]string{cmd: "out", ack: "in", sync: "inout", note: ""}
	conjugated := map[string]string{cmd: "in", ack: "out", sync: "inout", note: ""}
	owners := []struct {
		id   string
		want map[string]string
	}{
		{"D::CmdPort", original},
		{"D::CmdPort::'~CmdPort'", conjugated},
		{"D::SubPort", original},
		{"D::Device::pOut", original},
		{"D::Device::pIn", conjugated},
		{"D::Device::pSub", conjugated},
		// A port subsetting or redefining a conjugated port inherits the
		// features with the directions they have relative to that port.
		{"D::Device::pSubset", conjugated},
		{"D::SubDevice::pRedefined", conjugated},
		{"D::Outer::inner", original},
	}
	for _, owner := range owners {
		listed := memberIDs(t, srv, hash, owner.id)
		inputs := elementPropertyIDs(t, srv, hash, owner.id, "input")
		outputs := elementPropertyIDs(t, srv, hash, owner.id, "output")
		for feature, want := range owner.want {
			if !slices.Contains(listed, feature) {
				t.Errorf("%s members = %v, want %s among them", owner.id, listed, feature)
				continue
			}
			got := memberDirection(t, srv, hash, feature, owner.id)
			if got != want {
				t.Errorf("%s as a member of %s: direction = %q, want %q", feature, owner.id, got, want)
			}
			in, out := slices.Contains(inputs, feature), slices.Contains(outputs, feature)
			properties := map[[2]bool]string{{true, false}: "in", {false, true}: "out", {true, true}: "inout"}[[2]bool{in, out}]
			if got != properties {
				t.Errorf("%s as a member of %s: Symbol direction %q, element properties %q", feature, owner.id, got, properties)
			}
		}
	}

	// Read by itself a feature keeps its own Feature::direction.
	for feature, want := range original {
		if got := memberDirection(t, srv, hash, feature, ""); got != want {
			t.Errorf("%s: direction = %q, want %q", feature, got, want)
		}
	}
	// A port has no direction of its own, whatever it is a member of, and
	// conjugating its owner leaves the features of its own type alone.
	for _, step := range [][2]string{
		{"D::Device::pIn", "D::device"}, {"D::Device::pIn", "D::sub"}, {"D::Device::pIn", "D::Rack::slot"},
		{"D::Outer::inner", "D::Device::pOuter"},
	} {
		if got := memberDirection(t, srv, hash, step[0], step[1]); got != "" {
			t.Errorf("%s as a member of %s: direction = %q, want none", step[0], step[1], got)
		}
	}

	resp, err := srv.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: hash, SymbolId: cmd, MemberOf: "D::Missing"})
	if err != nil || resp.GetError() != "symbol not found: D::Missing" {
		t.Errorf("GetSymbol(member_of=D::Missing) = %q, %v, want the unknown type reported", resp.GetError(), err)
	}
}

// A service without symbol_member_direction ignores member_of, which is why a
// client must not ask it.
func TestGRPCMemberDirectionWithheldWithoutCapability(t *testing.T) {
	srv, err := grpc.NewServiceWithUnavailableCapabilitiesForTesting(4, "test", []string{grpc.CapabilitySymbolMemberDirection})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: memberDirectionModel}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	if got := memberDirection(t, srv, parsed.ModelHash, "D::CmdPort::cmd", "D::Device::pIn"); got != "out" {
		t.Errorf("direction = %q, want the feature's own out", got)
	}
}

// A conjugated typing reverses what the port inherits, not the features the port
// declares itself: Type::directionOf answers those with their own direction.
func TestGRPCMemberDirectionKeepsOwnedFeatureOfConjugatedPort(t *testing.T) {
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	const model = `package O {
	private import ScalarValues::*;
	port def CmdPort {
		out attribute cmd : Integer;
		in attribute ack : Boolean;
	}
	part def Device {
		port pOwn : ~CmdPort {
			in attribute :>> cmd;
			out attribute extra : Integer;
		}
	}
}`
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: model}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	hash := parsed.ModelHash
	const owner = "O::Device::pOwn"
	inputs := elementPropertyIDs(t, srv, hash, owner, "input")
	outputs := elementPropertyIDs(t, srv, hash, owner, "output")
	for feature, want := range map[string]string{owner + "::cmd": "in", owner + "::extra": "out", "O::CmdPort::ack": "out"} {
		if got := memberDirection(t, srv, hash, feature, owner); got != want {
			t.Errorf("%s as a member of %s: direction = %q, want %q", feature, owner, got, want)
		}
		if in, out := slices.Contains(inputs, feature), slices.Contains(outputs, feature); in != (want == "in") || out != (want == "out") {
			t.Errorf("%s: input = %v, output = %v, want %s only", feature, inputs, outputs, want)
		}
	}
}
