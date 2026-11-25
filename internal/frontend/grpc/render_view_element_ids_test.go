package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestRenderViewCarriesDurableElementIDs pins the structured form's element id
// to the EffectiveID an identity lookup reports for the same declaration.
func TestRenderViewCarriesDurableElementIDs(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "conformance", "fixtures", "views.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, cached := parseContent(t, srv, string(content))

	worker, release := cached.worker()
	defer release()
	sem, res := worker.Model.Semantics(), worker.Model.Resolver()
	// A wire span names a declaration by its start; the identity builder needs
	// the declaration's own span, which DeclaredFrom finds from that offset.
	wantAt := func(span *pb.Span) string {
		t.Helper()
		if span == nil || span.File == "" {
			return ""
		}
		sf := cached.document(span.File)
		if sf == nil {
			t.Fatalf("no source for %q", span.File)
		}
		offset := sf.Lines().OffsetAt(source.Pos{Line: int(span.StartLine), Col: int(span.StartCol)})
		root := cached.Index.DocumentRoot(span.File)
		if root == nil {
			t.Fatalf("no index root for %q", span.File)
		}
		sym := root.DeclaredFrom(offset)
		if sym == nil {
			t.Fatalf("no declaration at %s:%d:%d", span.File, span.StartLine, span.StartCol)
		}
		return effectiveID(t, sem, res, sym)
	}
	wantNamed := func(fqn string) string {
		t.Helper()
		syms := cached.Index.LookupQualified(fqn)
		if len(syms) == 0 {
			t.Fatalf("symbol %q not indexed", fqn)
		}
		return effectiveID(t, sem, res, syms[0])
	}

	rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "RenderViewDemo::connections", Ports: "full",
	})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	// The three ports the view draws are inherited from the two part definitions.
	portOwners := map[string]string{"sender": "Sender", "receiver": "Receiver"}
	var nodes, ports, edges int
	for _, node := range rendered.Nodes {
		nodes++
		if node.ElementId == "" || node.ElementId == node.Id {
			t.Errorf("node %q has element id %q, render-local id %q; want a durable identity", node.Name, node.ElementId, node.Id)
		}
		if got := wantAt(node.Origin); got != node.ElementId {
			t.Errorf("node %q element id = %q, want EffectiveID %q", node.Name, node.ElementId, got)
		}
		for _, port := range node.Ports {
			ports++
			owner, ok := portOwners[node.Name]
			if !ok {
				t.Fatalf("no port owner known for node %q; update the fixture expectation", node.Name)
			}
			want := wantNamed("RenderViewDemo::" + owner + "::" + port.Name)
			if port.ElementId == "" || port.ElementId == port.Id {
				t.Errorf("port %q of %q has element id %q, render-local id %q", port.Name, node.Name, port.ElementId, port.Id)
			}
			if port.ElementId != want {
				t.Errorf("port %q of %q element id = %q, want EffectiveID %q", port.Name, node.Name, port.ElementId, want)
			}
		}
	}
	for _, edge := range rendered.Edges {
		edges++
		if edge.ElementId == "" {
			t.Errorf("edge %q has no element id", edge.Kind)
		}
		if got := wantAt(edge.Origin); got != edge.ElementId {
			t.Errorf("edge %q element id = %q, want EffectiveID %q", edge.Kind, edge.ElementId, got)
		}
	}
	if nodes == 0 || ports == 0 || edges == 0 {
		t.Fatalf("fixture rendered %d nodes, %d ports, %d edges; want all three", nodes, ports, edges)
	}

	// The engine's protojson-shaped answer carries the same fields as the service.
	eng, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(map[string]any{
		"documents": []map[string]string{{"name": "views.sysml", "content": string(content)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := eng.Call(context.Background(), "ParseSources", params)
	if err != nil {
		t.Fatalf("engine ParseSources: %v", err)
	}
	var parsedEngine engine.JParseSourcesResponse
	if err := json.Unmarshal(body, &parsedEngine); err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(map[string]any{
		"modelHash": parsedEngine.ModelHash, "view": "RenderViewDemo::connections", "ports": "full",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err = eng.Call(context.Background(), "RenderView", request)
	if err != nil {
		t.Fatalf("engine RenderView: %v", err)
	}
	var fromEngine pb.RenderViewResponse
	if err := protojson.Unmarshal(body, &fromEngine); err != nil {
		t.Fatalf("engine RenderView is not protojson: %v\n%s", err, body)
	}
	if len(fromEngine.Nodes) != len(rendered.Nodes) || len(fromEngine.Edges) != len(rendered.Edges) {
		t.Fatalf("engine answered %d nodes and %d edges, want %d and %d",
			len(fromEngine.Nodes), len(fromEngine.Edges), len(rendered.Nodes), len(rendered.Edges))
	}
	for i, node := range fromEngine.Nodes {
		if node.ElementId != rendered.Nodes[i].ElementId {
			t.Errorf("engine node %d element id = %q, want %q", i, node.ElementId, rendered.Nodes[i].ElementId)
		}
		for j, port := range node.Ports {
			if port.ElementId != rendered.Nodes[i].Ports[j].ElementId {
				t.Errorf("engine port %d:%d element id = %q, want %q", i, j, port.ElementId, rendered.Nodes[i].Ports[j].ElementId)
			}
		}
	}
	for i, edge := range fromEngine.Edges {
		if edge.ElementId != rendered.Edges[i].ElementId {
			t.Errorf("engine edge %d element id = %q, want %q", i, edge.ElementId, rendered.Edges[i].ElementId)
		}
	}
}

func effectiveID(t *testing.T, sem *semantics.Model, res *resolve.Resolver, sym *symbols.Symbol) string {
	t.Helper()
	info, ok := identity.Of(sem, res, sym)
	if !ok || info == nil {
		t.Fatalf("no identity for %s", sym.Name)
	}
	return info.EffectiveID
}
