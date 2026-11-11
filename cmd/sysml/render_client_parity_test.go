package main

import (
	"context"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// renderParityModel declares a tree view, an interconnection view with ports
// and a table view, so every form has a kind it is written in.
const renderParityModel = `package Demo {
    private import OpenSysMLRenderings::*;
    port def Power;
    part def Wheel { port hub : Power; }
    part def Engine { port shaft : Power; port spare : Power; }
    part def Vehicle {
        attribute mass = 1200;
        part engine : Engine;
        part wheel : Wheel;
        connect engine.shaft to wheel.hub;
    }
    view overview { expose Demo::Vehicle; expose Demo::Engine; expose Demo::Wheel; }
    view wiring { expose Demo::Vehicle; render Views::asInterconnectionDiagram; }
    view parts { expose Demo::Vehicle; render Views::asElementTable; }
}
`

// The client's WithForm text is what the CLI writes for the same view, form
// and options, byte for byte, whatever the form and however it is optioned.
func TestRenderViewFormMatchesTheCLI(t *testing.T) {
	binary := buildCLI(t)
	path := writeModel(t, t.TempDir(), "parity.sysml", renderParityModel)
	client, err := opensysml.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	model, err := client.ParseFile(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	link := "https://example.test/src/{file}#L{line}:{col}?{qname}&{id}"
	cases := []struct {
		name    string
		view    string
		flags   []string
		options []opensysml.RenderViewOption
	}{
		{"text", "Demo::overview", []string{"-render-form", "text"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormText)}},
		{"mermaid", "Demo::overview", []string{"-render-form", "mermaid"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormMermaid)}},
		{"dot", "Demo::overview", []string{"-render-form", "dot"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormDOT)}},
		{"plantuml", "Demo::overview", []string{"-render-form", "plantuml"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormPlantUML)}},
		{"d2", "Demo::overview", []string{"-render-form", "d2"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormD2)}},
		{"drawio", "Demo::overview", []string{"-render-form", "drawio"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormDrawio)}},
		{"markdown", "Demo::parts", []string{"-render-form", "markdown"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormMarkdown)}},
		{"csv", "Demo::parts", []string{"-render-form", "csv"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormCSV)}},
		{"tsv", "Demo::parts", []string{"-render-form", "tsv"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormTSV)}},
		{"table text", "Demo::parts", []string{"-render-form", "text"}, []opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormText)}},
		{
			"dot optioned", "Demo::overview",
			[]string{
				"-render-form", "dot", "-render-palette", "okabe-ito", "-render-style", "cameo", "-render-lines", "ortho",
				"-render-link", link, "-render-show", "keywords,typings", "-render-compartments", "most", "-render-depth", "1",
			},
			[]opensysml.RenderViewOption{
				opensysml.WithForm(opensysml.ViewFormDOT), opensysml.WithPalette("okabe-ito"), opensysml.WithStyle("cameo"),
				opensysml.WithLines("ortho"), opensysml.WithLink(link), opensysml.WithShow("keywords", "typings"),
				opensysml.WithCompartments("most"), opensysml.WithDepth(1),
			},
		},
		{
			"dot interconnection", "Demo::wiring",
			[]string{"-render-form", "dot", "-render-ports", "full", "-render-unplaced", "strip", "-render-lines", "polyline"},
			[]opensysml.RenderViewOption{
				opensysml.WithForm(opensysml.ViewFormDOT), opensysml.WithFullPorts(), opensysml.WithUnplaced("strip"),
				opensysml.WithLines("polyline"),
			},
		},
		{
			"mermaid optioned", "Demo::overview",
			[]string{"-render-form", "mermaid", "-render-palette", "viridis", "-render-link", link, "-render-style", "cameo"},
			[]opensysml.RenderViewOption{
				opensysml.WithForm(opensysml.ViewFormMermaid), opensysml.WithPalette("viridis"), opensysml.WithLink(link),
				opensysml.WithStyle("cameo"),
			},
		},
		{
			"plantuml optioned", "Demo::wiring",
			[]string{"-render-form", "plantuml", "-render-lines", "ortho", "-render-palette", "tol-bright", "-render-link", link},
			[]opensysml.RenderViewOption{
				opensysml.WithForm(opensysml.ViewFormPlantUML), opensysml.WithLines("ortho"), opensysml.WithPalette("tol-bright"),
				opensysml.WithLink(link),
			},
		},
		{
			"csv optioned", "Demo::parts",
			[]string{"-render-form", "csv", "-render-show", "all", "-render-link", link},
			[]opensysml.RenderViewOption{opensysml.WithForm(opensysml.ViewFormCSV), opensysml.WithShow("all"), opensysml.WithLink(link)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := runFiles(t, binary, []string{path}, append([]string{"-render", tc.view}, tc.flags...)...)
			if cli.status != exitHolds {
				t.Fatalf("exit status = %d, want %d\n%s", cli.status, exitHolds, cli.output())
			}
			rendered, err := client.RenderView(ctx, model, tc.view, tc.options...)
			if err != nil {
				t.Fatal(err)
			}
			if rendered.Text == "" || rendered.Text != cli.stdout {
				t.Errorf("RenderView text differs from the CLI's\nclient:\n%s\nCLI:\n%s", rendered.Text, cli.stdout)
			}
		})
	}
}
