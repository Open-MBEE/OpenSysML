package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const renderViewFormSource = `package FormDemo {
    private import Views::*;
    part def Wheel;
    part def Vehicle {
        part wheel : Wheel;
    }
    view structure {
        expose Vehicle;
        expose Wheel;
        render asTreeDiagram;
    }
    view parts {
        expose Vehicle;
        render asElementTable;
    }
}
`

// renderViewFormEngine parses the source into a JSON engine beside the service.
func renderViewFormEngine(t *testing.T) (*engine.Engine, string) {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	params, err := json.Marshal(map[string]any{
		"documents": []map[string]string{{"name": "form.sysml", "content": renderViewFormSource}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := eng.Call(context.Background(), "ParseSources", params)
	if err != nil {
		t.Fatalf("engine ParseSources: %v", err)
	}
	var parsed engine.JParseSourcesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode engine ParseSources: %v", err)
	}
	return eng, parsed.ModelHash
}

func TestRenderViewFormMatchesEngineJSON(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	parsed, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{{
		Source: &pb.SourceDocument_Content{Content: renderViewFormSource}, Name: "form.sysml",
	}}})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	eng, engineHash := renderViewFormEngine(t)

	requests := []*pb.RenderViewRequest{
		{View: "FormDemo::structure", Form: "dot", Palette: "okabe-ito", Style: "cameo", Lines: "ortho", Direction: "LR", Unplaced: "strip"},
		{View: "FormDemo::structure", Form: "dot", Link: "https://example.test/{file}#L{line}"},
		{View: "FormDemo::structure", Palette: "okabe-ito"},
	}
	for _, form := range view.ViewForms() {
		target := "FormDemo::structure"
		if form == view.FormCSV || form == view.FormTSV || form == view.FormMarkdown {
			target = "FormDemo::parts"
		}
		requests = append(requests, &pb.RenderViewRequest{View: target, Form: string(form)})
	}
	for _, req := range requests {
		t.Run(req.View+" "+req.Form, func(t *testing.T) {
			req.ModelHash = parsed.ModelHash
			want, err := srv.RenderView(ctx, req)
			if err != nil {
				t.Fatalf("gRPC RenderView: %v", err)
			}
			if (req.Form == "") != (want.Text == "") {
				t.Fatalf("form %q wrote text %q", req.Form, want.Text)
			}
			req.ModelHash = engineHash
			params, err := protojson.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := eng.Call(ctx, "RenderView", params)
			if err != nil {
				t.Fatalf("engine RenderView: %v", err)
			}
			var got pb.RenderViewResponse
			if err := protojson.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode engine RenderView as protojson: %v\n%s", err, body)
			}
			if !proto.Equal(want, &got) {
				t.Errorf("engine RenderView JSON differs from the gRPC answer:\nengine: %s\ngRPC: %s", body, protojson.Format(want))
			}
		})
	}
}

func TestRenderViewFormRefusalsMatchEngineJSON(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	parsed, _ := parseContent(t, srv, renderViewFormSource)
	eng, engineHash := renderViewFormEngine(t)

	for _, test := range []struct {
		name string
		req  *pb.RenderViewRequest
		want string
	}{
		{"unknown form", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "png"},
			`unknown rendering form "png"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"run-only form", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "svg"},
			`unknown rendering form "svg"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"graph form of a table", &pb.RenderViewRequest{View: "FormDemo::parts", Form: "dot"},
			"FormDemo::parts: a table rendering is not written as dot; ask for text, markdown, csv or tsv"},
		{"table form of a graph", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "csv"}, "is not written as csv"},
		{"unknown palette", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "dot", Palette: "neon"}, `unknown palette "neon"`},
		{"unknown style", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "dot", Style: "neon"}, `"neon"`},
		{"unknown lines", &pb.RenderViewRequest{View: "FormDemo::structure", Lines: "zigzag"}, `"zigzag"`},
		{"unknown direction", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "dot", Direction: "lr"}, `unknown direction "lr"`},
		{"unknown unplaced", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "dot", Unplaced: "pile"}, `"pile"`},
		{"unknown link placeholder", &pb.RenderViewRequest{View: "FormDemo::structure", Form: "dot", Link: "{nothing}"}, "nothing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.req.ModelHash = parsed.ModelHash
			_, err := srv.RenderView(ctx, test.req)
			if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("RenderView error = %v, want INVALID_ARGUMENT containing %q", err, test.want)
			}
			test.req.ModelHash = engineHash
			params, marshalErr := protojson.Marshal(test.req)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			_, engineErr := eng.Call(ctx, "RenderView", params)
			var refusal *engine.Error
			var status *connect.Error
			if !errors.As(engineErr, &refusal) || !errors.As(err, &status) ||
				refusal.Code != uint32(connect.CodeInvalidArgument) || refusal.Message != status.Message() {
				t.Errorf("engine error = %v, want the service's %v", engineErr, err)
			}
		})
	}
}

// A model read from a file has sites, so a link template is filled and
// draw.io records each element's file and line; inline content has none.
func TestRenderViewFormLinksTheSourceOfAFile(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "form.sysml")
	if err := os.WriteFile(path, []byte(renderViewFormSource), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	linked, err := srv.RenderView(ctx, &pb.RenderViewRequest{
		ModelHash: parsed.ModelHash, View: "FormDemo::structure", Form: "dot", Link: "https://example.test/{qname}#L{line}:{col}",
	})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if want := `URL="https://example.test/FormDemo::Vehicle#L4:5"`; !strings.Contains(linked.Text, want) {
		t.Errorf("dot lacks %s:\n%s", want, linked.Text)
	}
	drawio, err := srv.RenderView(ctx, &pb.RenderViewRequest{ModelHash: parsed.ModelHash, View: "FormDemo::structure", Form: "drawio"})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if want := `fqn="FormDemo::Vehicle::wheel" kind="part" view="FormDemo::structure" link="` + filepath.ToSlash(path) + `:5"`; !strings.Contains(drawio.Text, want) {
		t.Errorf("draw.io lacks %s:\n%s", want, drawio.Text)
	}

	inline, _ := parseContent(t, srv, renderViewFormSource)
	unlinked, err := srv.RenderView(ctx, &pb.RenderViewRequest{
		ModelHash: inline.ModelHash, View: "FormDemo::structure", Form: "dot", Link: "https://example.test/{qname}#L{line}:{col}",
	})
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if strings.Contains(unlinked.Text, "URL=") {
		t.Errorf("inline content has no file to link, yet the dot links:\n%s", unlinked.Text)
	}
}
