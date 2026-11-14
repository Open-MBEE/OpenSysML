package grpc

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const unnamedChildrenModel = `package UnnamedChildren {
	port def CmdPort { in attribute cmd : ScalarValues::Integer; }
	part def Logic { port cmdIn : CmdPort; port cmdOut : ~CmdPort; }
	metadata def Tag;
	part def Controller {
		port cmdIn : CmdPort;
		part logic : Logic;
		part monitor : Logic;
		bind cmdIn = logic.cmdIn;
		binding toMonitor bind cmdIn = monitor.cmdIn;
		bind logic.cmdOut = monitor.cmdIn;
		connect logic.cmdOut to monitor.cmdIn;
		part : Logic { part inner : Logic; }
		@Tag;
	}
	#Tag part def Tagged;
}`

func getSymbol(t *testing.T, srv *Service, hash, id string) *pb.SymbolInfo {
	t.Helper()
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: hash, SymbolId: id})
	if err != nil {
		t.Fatalf("GetSymbol(%s): %v", id, err)
	}
	if resp.Symbol == nil {
		t.Fatalf("GetSymbol(%s): %s", id, resp.Error)
	}
	return resp.Symbol
}

// An unnamed member is listed in child_ids under the positional id the query
// API reports, and GetSymbol resolves it, so children agree with the query.
func TestChildIDsNameUnnamedMembersPositionally(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, unnamedChildrenModel)
	controller := getSymbol(t, srv, hash, "UnnamedChildren::Controller")
	want := []string{
		"UnnamedChildren::Controller::cmdIn", "UnnamedChildren::Controller::logic", "UnnamedChildren::Controller::monitor",
		"UnnamedChildren::Controller::@3", "UnnamedChildren::Controller::toMonitor", "UnnamedChildren::Controller::@5",
		"UnnamedChildren::Controller::@6", "UnnamedChildren::Controller::@7", "UnnamedChildren::Controller::@8",
	}
	if !slices.Equal(controller.ChildIds, want) {
		t.Fatalf("child_ids = %v, want %v", controller.ChildIds, want)
	}

	resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{Scope: []string{"UnnamedChildren::Controller"}}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var queried []string
	for _, element := range resp.Elements {
		if element.Properties[QueryPropOwner] == "UnnamedChildren::Controller" {
			queried = append(queried, element.Id)
		}
	}
	if !slices.Equal(queried, want) {
		t.Errorf("query members = %v, want the child_ids %v", queried, want)
	}

	kinds := map[string]string{
		"UnnamedChildren::Controller::@3": "bindingConnectorAsUsage",
		"UnnamedChildren::Controller::@5": "bindingConnectorAsUsage",
		"UnnamedChildren::Controller::@6": "connectionUsage",
		"UnnamedChildren::Controller::@7": "partUsage",
		"UnnamedChildren::Controller::@8": "metadataUsage",
	}
	for id, kind := range kinds {
		sym := getSymbol(t, srv, hash, id)
		if sym.Id != id || sym.Kind != kind || sym.Name != "" {
			t.Errorf("GetSymbol(%s) = %s %s %q, want the anonymous %s under that id", id, sym.Id, sym.Kind, sym.Name, kind)
		}
	}
	anonymous := getSymbol(t, srv, hash, "UnnamedChildren::Controller::@7")
	if !slices.Equal(anonymous.ChildIds, []string{"UnnamedChildren::Controller::@7::inner"}) {
		t.Errorf("anonymous part child_ids = %v, want its named member under the positional path", anonymous.ChildIds)
	}
	inner := getSymbol(t, srv, hash, "UnnamedChildren::Controller::@7::inner")
	if inner.Name != "inner" || inner.Kind != "partUsage" {
		t.Errorf("GetSymbol(@7::inner) = %s %q, want the part inner", inner.Kind, inner.Name)
	}
	if resp, _ := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: hash, SymbolId: "UnnamedChildren::Controller::"}); resp.Symbol != nil || resp.Error == "" {
		t.Errorf("GetSymbol(Controller::) = %v, want an error: the malformed id names nothing", resp)
	}
	if resp, _ := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{ModelHash: hash, SymbolId: "UnnamedChildren::Controller::@9"}); resp.Symbol != nil || resp.Error == "" {
		t.Errorf("GetSymbol(Controller::@9) = %v, want an error: no ninth member", resp)
	}
}

// A `#M` prefix is a member of the annotated element, listed and resolvable
// like the body spelling.
func TestChildIDsListPrefixMetadata(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, unnamedChildrenModel)
	tagged := getSymbol(t, srv, hash, "UnnamedChildren::Tagged")
	if !slices.Equal(tagged.ChildIds, []string{"UnnamedChildren::Tagged::@0"}) {
		t.Fatalf("Tagged child_ids = %v, want its #Tag at @0", tagged.ChildIds)
	}
	tag := getSymbol(t, srv, hash, "UnnamedChildren::Tagged::@0")
	if tag.Kind != "metadataUsage" || tag.TypeInfo == nil || tag.TypeInfo.ResolvedId != "UnnamedChildren::Tag" {
		t.Errorf("GetSymbol(Tagged::@0) = %s typed %v, want a metadataUsage typed by UnnamedChildren::Tag", tag.Kind, tag.TypeInfo)
	}
}

// The root's child_ids and an Evaluate context name an unnamed element the same way.
func TestPositionalIDsReachTheRootAndEvaluate(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, "part def P { attribute x = 2; } part : P { attribute y = 3; }")
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: "part def P { attribute x = 2; } part : P { attribute y = 3; }"}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if parsed.ModelHash != hash {
		t.Fatalf("the same content hashes differently: %s vs %s", parsed.ModelHash, hash)
	}
	if !slices.Equal(parsed.Root.ChildIds, []string{"P", "@1"}) {
		t.Fatalf("root child_ids = %v, want [P @1]", parsed.Root.ChildIds)
	}
	eval, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: hash, Expression: "y + 1", ContextSymbolId: "@1"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if eval.Error != "" || eval.Result == nil || eval.Result.GetIntValue() != 4 {
		t.Errorf("Evaluate(y + 1 in @1) = %v %q, want 4", eval.Result, eval.Error)
	}
}

// The annotated element owns its `#M` prefix, so its type resolves in that
// element's own scope as the pilot resolves it: a nested namesake shadows.
func TestPrefixMetadataTypeResolvesInTheAnnotatedElement(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, `package Shadow {
	metadata def Tag;
	#Tag part def A { metadata def Tag; }
}`)
	a := getSymbol(t, srv, hash, "Shadow::A")
	if !slices.Equal(a.ChildIds, []string{"Shadow::A::Tag", "Shadow::A::@1"}) {
		t.Fatalf("A child_ids = %v, want the nested Tag then the #Tag prefix at @1", a.ChildIds)
	}
	tag := getSymbol(t, srv, hash, "Shadow::A::@1")
	if tag.TypeInfo == nil || tag.TypeInfo.ResolvedId != "Shadow::A::Tag" {
		t.Errorf("GetSymbol(A::@1) typed %v, want Shadow::A::Tag, the definition the prefix's owner declares", tag.TypeInfo)
	}
}

// The members a symbol enumerates with provenance name unnamed members and a
// `#M` prefix by the same ids child_ids does, then the inherited ones.
func TestMembersListPositionalAndPrefixIdsWithProvenance(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, unnamedChildrenModel)
	for _, id := range []string{"UnnamedChildren::Controller", "UnnamedChildren::Tagged"} {
		info := getSymbol(t, srv, hash, id)
		var own []string
		for _, m := range info.Members {
			if !m.Inherited {
				own = append(own, m.Id)
			}
		}
		if !slices.Equal(own, info.ChildIds) {
			t.Errorf("%s own members = %v, want child_ids %v", id, own, info.ChildIds)
		}
	}
	controller := getSymbol(t, srv, hash, "UnnamedChildren::Controller")
	if controller.WithheldLibraryMembers == 0 {
		t.Errorf("Controller withholds no library members, want the Parts::Part ones counted")
	}
	for _, m := range controller.Members {
		if m.Inherited && m.InheritedFrom == "" {
			t.Errorf("inherited member %s names no declaring type", m.Id)
		}
	}
}
