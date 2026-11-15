package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

const viewFormSource = `package FormDemo {
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

func TestRenderViewWritesTheFormAsked(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, viewFormSource)
	ctx := context.Background()
	for form, test := range map[string]struct{ view, want string }{
		opensysml.ViewFormText:     {"FormDemo::structure", "FormDemo::structure - tree rendering (render asTreeDiagram)\n"},
		opensysml.ViewFormMermaid:  {"FormDemo::structure", "flowchart TD\n"},
		opensysml.ViewFormDOT:      {"FormDemo::structure", "digraph \"FormDemo::structure\" {\n"},
		opensysml.ViewFormPlantUML: {"FormDemo::structure", "@startuml\n"},
		opensysml.ViewFormD2:       {"FormDemo::structure", "n0 <- n1: { class: composition }\n"},
		opensysml.ViewFormDrawio:   {"FormDemo::structure", "<mxfile host=\"OpenSysML\""},
		opensysml.ViewFormMarkdown: {"FormDemo::parts", "| wheel | part | Wheel | FormDemo::Vehicle |\n"},
		opensysml.ViewFormCSV:      {"FormDemo::parts", "wheel,part,Wheel,FormDemo::Vehicle\n"},
		opensysml.ViewFormTSV:      {"FormDemo::parts", "wheel\tpart\tWheel\tFormDemo::Vehicle\n"},
	} {
		rendered, err := client.RenderView(ctx, model, test.view, opensysml.WithForm(form))
		if err != nil {
			t.Errorf("RenderView as %s: %v", form, err)
			continue
		}
		if !strings.Contains(rendered.Text, test.want) || !strings.HasSuffix(rendered.Text, "\n") || strings.HasSuffix(rendered.Text, "\n\n") {
			t.Errorf("%s text lacks %q or does not end in one newline:\n%s", form, test.want, rendered.Text)
		}
		if rendered.View != test.view || len(rendered.Nodes)+len(rendered.Rows) == 0 {
			t.Errorf("%s answered no data beside its text: %+v", form, rendered)
		}
	}
	if len(opensysml.ViewForms()) != 9 {
		t.Errorf("ViewForms() = %v, want the nine forms tested here", opensysml.ViewForms())
	}
	data, err := client.RenderView(ctx, model, "FormDemo::structure")
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if data.Text != "" || len(data.Nodes) != 3 {
		t.Errorf("no form: text %q, %d nodes; want no text and the three nodes", data.Text, len(data.Nodes))
	}
}

func TestRenderViewWritesUnderItsWriterOptions(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, viewFormSource)
	rendered, err := client.RenderView(context.Background(), model, "FormDemo::structure",
		opensysml.WithForm(opensysml.ViewFormDOT), opensysml.WithPalette("okabe-ito"), opensysml.WithStyle("cameo"),
		opensysml.WithLines("ortho"), opensysml.WithDirection("LR"), opensysml.WithUnplaced("strip"),
		opensysml.WithLink("https://example.test/{qname}"))
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	for _, want := range []string{
		`graph [fontname="Arial", fontcolor="#424242", rankdir=LR, splines=ortho];`,
		`label=<<b>bdd</b> [Block] Vehicle [ structure ]>;`,
		`"n0" [fillcolor="#E69F00", color="#E69F00",`,
	} {
		if !strings.Contains(rendered.Text, want) {
			t.Errorf("dot lacks %s:\n%s", want, rendered.Text)
		}
	}
}

func TestRenderViewRefusesAFormItCannotWrite(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, viewFormSource)
	ctx := context.Background()
	for _, test := range []struct {
		name, view string
		options    []opensysml.RenderViewOption
		want       string
	}{
		{"unknown form", "FormDemo::structure", []opensysml.RenderViewOption{opensysml.WithForm("png")},
			`unknown rendering form "png"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"run-only form", "FormDemo::structure", []opensysml.RenderViewOption{opensysml.WithForm("svg")},
			`unknown rendering form "svg"; the forms are text, mermaid, markdown, dot, plantuml, d2, drawio, csv and tsv`},
		{"graph form of a table", "FormDemo::parts", []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormDOT)},
			"FormDemo::parts: a table rendering is not written as dot; ask for text, markdown, csv or tsv"},
		{"unknown palette", "FormDemo::structure", []opensysml.RenderViewOption{opensysml.WithPalette("neon")}, `unknown palette "neon"`},
		{"unknown direction", "FormDemo::structure", []opensysml.RenderViewOption{opensysml.WithDirection("lr")}, `unknown direction "lr"`},
	} {
		_, err := client.RenderView(ctx, model, test.view, test.options...)
		var status *opensysml.StatusError
		if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument || !strings.Contains(status.Message, test.want) {
			t.Errorf("%s: err = %v, want CodeInvalidArgument containing %q", test.name, err, test.want)
		}
	}
}

// A service without render_view_form would drop the form and its options
// unanswered, so the client refuses to send one; the data alone is still asked.
func TestRenderViewFormNeedsItsCapability(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityRenderViewForm})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := dialClient(t, server.URL)
	ctx := context.Background()
	model := parse(t, client, viewFormSource)
	for name, option := range map[string]opensysml.RenderViewOption{
		"form": opensysml.WithForm(opensysml.ViewFormDOT), "palette": opensysml.WithPalette("okabe-ito"),
		"style": opensysml.WithStyle("cameo"), "lines": opensysml.WithLines("ortho"),
		"direction": opensysml.WithDirection("LR"), "link": opensysml.WithLink("{file}"),
		"unplaced": opensysml.WithUnplaced("strip"),
	} {
		_, err := client.RenderView(ctx, model, "FormDemo::structure", option)
		wantCapabilityRefusal(t, "RenderView with a "+name, err, opensysml.CapabilityRenderViewForm)
	}
	if data, err := client.RenderView(ctx, model, "FormDemo::structure"); err != nil || len(data.Nodes) != 3 {
		t.Errorf("RenderView without a form = %+v, %v; want the three nodes", data, err)
	}
}

func TestRenderDocumentAnswersTheFormAsked(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	ctx := context.Background()
	for form, want := range map[string]string{
		opensysml.DocumentFormMarkdown: "# Telescope Mass Report",
		opensysml.DocumentFormHTML:     "<h1",
		opensysml.DocumentFormAsciiDoc: "= Telescope Mass Report",
	} {
		rendered, err := client.RenderDocument(ctx, model, "Observatory::MassReport", opensysml.WithDocumentForm(form))
		if err != nil {
			t.Errorf("RenderDocument as %s: %v", form, err)
			continue
		}
		if !strings.Contains(rendered, want) || !strings.Contains(rendered, "Telescope Mass Report") {
			t.Errorf("%s document lacks %q:\n%s", form, want, rendered)
		}
	}
	_, err := client.RenderDocument(ctx, model, "Observatory::MassReport", opensysml.WithDocumentForm("pdf"))
	var status *opensysml.StatusError
	if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument {
		t.Errorf("RenderDocument as pdf: err = %v, want CodeInvalidArgument", err)
	}
}

func TestRenderDocumentRefusesAnUnknownFormBeforeTheCall(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, documentSource)
	for _, form := range []string{"pdf", "HTML", "md"} {
		rendered, err := client.RenderDocument(context.Background(), model, "Observatory::MassReport", opensysml.WithDocumentForm(form))
		var status *opensysml.StatusError
		if !errors.As(err, &status) || status.Code != opensysml.CodeInvalidArgument ||
			!strings.Contains(status.Message, "is not one RenderDocument renders") || rendered != "" {
			t.Errorf("RenderDocument as %s = %q, %v, want the client's CodeInvalidArgument", form, rendered, err)
		}
	}
}

func TestRenderDocumentFormsNeedTheirCapabilities(t *testing.T) {
	for form, capability := range map[string]string{
		opensysml.DocumentFormHTML:     opensysml.CapabilityRenderDocumentHTML,
		opensysml.DocumentFormAsciiDoc: opensysml.CapabilityRenderDocumentAsciiDoc,
	} {
		svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{capability})
		if err != nil {
			t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
		}
		t.Cleanup(svc.Close)
		mux := http.NewServeMux()
		mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)

		client := dialClient(t, server.URL)
		ctx := context.Background()
		model := parse(t, client, documentSource)
		_, err = client.RenderDocument(ctx, model, "Observatory::MassReport", opensysml.WithDocumentForm(form))
		wantCapabilityRefusal(t, "RenderDocument as "+form, err, capability)
		if markdown, err := client.RenderDocument(ctx, model, "Observatory::MassReport"); err != nil || !strings.Contains(markdown, "Telescope Mass Report") {
			t.Errorf("RenderDocument as Markdown without %s = %q, %v", capability, markdown, err)
		}
	}
}
