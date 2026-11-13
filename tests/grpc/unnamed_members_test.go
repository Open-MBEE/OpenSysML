package grpc_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// unnamedMembersModel declares one unnamed member of each listable kind in a
// package and in a part definition, and a usage that reads the latter by typing.
const unnamedMembersModel = `package U {
	private import ScalarValues::*;
	metadata def M;
	part def Unit { out item o; in item i; }
	part u1 : Unit;
	part u2 : Unit;
	part : Unit;
	attribute : Real;
	bind u1.o = u2.o;
	connect u1 to u2;
	flow from u1.o to u2.i;
	@M;
	part def Holder {
		part x : Unit;
		part y : Unit;
		part : Unit;
		attribute : Real;
		bind x.o = y.o;
		connect x to y;
		flow from x.o to y.i;
		@M;
	}
	part holder : Holder;
	part def Base { part a; attribute x; }
}`

var unnamedMemberKinds = []string{
	"partUsage", "attributeUsage", "bindingConnectorAsUsage", "connectionUsage", "flowUsage", "metadataUsage",
}

func unnamedMembersSymbol(t *testing.T, srv *grpc.Service, hash, id string, library bool) *pb.SymbolInfo {
	t.Helper()
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: hash, SymbolId: id, IncludeLibraryMembers: library})
	if err != nil || resp.GetError() != "" || resp.GetSymbol() == nil {
		t.Fatalf("GetSymbol(%s): %v %s", id, err, resp.GetError())
	}
	return resp.GetSymbol()
}

func parseUnnamedMembers(t *testing.T) (*grpc.Service, string) {
	t.Helper()
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: unnamedMembersModel}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	return srv, parsed.ModelHash
}

// Every unnamed member a package or a definition owns is a child under a positional
// id that GetSymbol resolves back to it, with the kind of its metaclass.
func TestGRPCUnnamedOwnMembersAreListedAndRoundTrip(t *testing.T) {
	srv, hash := parseUnnamedMembers(t)
	for _, owner := range []string{"U", "U::Holder"} {
		sym := unnamedMembersSymbol(t, srv, hash, owner, false)
		var kinds []string
		for _, id := range sym.GetChildIds() {
			if !strings.Contains(id, "::@") {
				continue
			}
			child := unnamedMembersSymbol(t, srv, hash, id, false)
			if child.GetId() != id || child.GetName() != "" {
				t.Errorf("GetSymbol(%s) = id %q name %q, want the same id and no name", id, child.GetId(), child.GetName())
			}
			kinds = append(kinds, child.GetKind())
		}
		if !slices.Equal(kinds, unnamedMemberKinds) {
			t.Errorf("%s unnamed children kinds = %v, want %v", owner, kinds, unnamedMemberKinds)
		}
	}
}

// A usage owns none of its type's unnamed members (KerML 8.3.2.4) and lists each as
// an inherited member under the id its definition lists it by (KerML 8.3.3.1).
func TestGRPCUnnamedMembersAreInheritedThroughTyping(t *testing.T) {
	srv, hash := parseUnnamedMembers(t)
	holder := unnamedMembersSymbol(t, srv, hash, "U::holder", false)
	if len(holder.GetChildIds()) != 0 {
		t.Errorf("U::holder child_ids = %v, want none", holder.GetChildIds())
	}
	var ids, kinds []string
	for _, member := range holder.GetMembers() {
		if member.GetName() != "" {
			continue
		}
		if !member.GetInherited() || member.GetInheritedFrom() != "U::Holder" {
			t.Errorf("member %s inherited=%v from %q, want inherited from U::Holder", member.GetId(), member.GetInherited(), member.GetInheritedFrom())
		}
		ids = append(ids, member.GetId())
		kinds = append(kinds, member.GetKind())
	}
	if !slices.Equal(kinds, unnamedMemberKinds) {
		t.Errorf("U::holder unnamed inherited kinds = %v, want %v", kinds, unnamedMemberKinds)
	}
	var owned []string
	for _, id := range unnamedMembersSymbol(t, srv, hash, "U::Holder", false).GetChildIds() {
		if strings.Contains(id, "::@") {
			owned = append(owned, id)
		}
	}
	if !slices.Equal(ids, owned) {
		t.Errorf("U::holder unnamed inherited ids = %v, want those U::Holder owns %v", ids, owned)
	}
}

// Unnamed inherited library features are members under unique positional ids of the declaring
// library type that GetSymbol resolves, and are counted as withheld when not asked for.
func TestGRPCUnnamedLibraryMembersAreInheritedWithPositionalIDs(t *testing.T) {
	srv, hash := parseUnnamedMembers(t)
	base := unnamedMembersSymbol(t, srv, hash, "U::Base", true)
	if base.GetWithheldLibraryMembers() != 0 {
		t.Errorf("withheld library members = %d with library members listed", base.GetWithheldLibraryMembers())
	}
	seen := map[string]bool{}
	features := map[string]int{}
	library := 0
	for _, member := range base.GetMembers() {
		if seen[member.GetId()] {
			t.Errorf("member id %s is listed twice", member.GetId())
		}
		seen[member.GetId()] = true
		if member.GetLibrary() {
			library++
		}
		if member.GetName() != "" || !member.GetInherited() {
			continue
		}
		if !member.GetLibrary() || !strings.HasPrefix(member.GetId(), member.GetInheritedFrom()+"::@") {
			t.Errorf("unnamed member %s (library=%v) inherited from %q, want a positional id of that library type", member.GetId(), member.GetLibrary(), member.GetInheritedFrom())
		}
		resolved := unnamedMembersSymbol(t, srv, hash, member.GetId(), false)
		if resolved.GetId() != member.GetId() || resolved.GetKind() != member.GetKind() || resolved.GetName() != "" {
			t.Errorf("GetSymbol(%s) = %s %s %q, want the member listed", member.GetId(), resolved.GetId(), resolved.GetKind(), resolved.GetName())
		}
		if strings.HasSuffix(member.GetKind(), "Usage") {
			features[member.GetKind()]++
		}
	}
	// Occurrences::Occurrence declares these 18; Parts::Part reaches them through Items::Item.
	want := map[string]int{"connectionUsage": 7, "constraintUsage": 8, "bindingConnectorAsUsage": 3}
	for kind, n := range want {
		if features[kind] != n {
			t.Errorf("unnamed inherited library %s = %d, want %d (all: %v)", kind, features[kind], n, features)
		}
	}
	if len(features) != len(want) {
		t.Errorf("unnamed inherited library features = %v, want kinds %v", features, want)
	}
	if !seen["Occurrences::Occurrence::@47"] {
		t.Error("U::Base does not list the library binding Occurrences::Occurrence::@47")
	}
	withheld := unnamedMembersSymbol(t, srv, hash, "U::Base", false)
	if got := int(withheld.GetWithheldLibraryMembers()); got != library {
		t.Errorf("withheld library members = %d, want the %d listed on request", got, library)
	}
}
