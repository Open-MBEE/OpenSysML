package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// docRenderModel declares a document over a small part tree: a titled report
// with a query-backed table and a numbered list.
const docRenderModel = docQueryModel + `package Reports {
	private import DocumentQueries::*;
	private import Observatory::*;

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup for the telescope assembly.";
		}

		part breakdown : Section {
			attribute redefines title = "Heavy Subsystems";

			part masses : Table {
				attribute redefines caption = "Heavy subsystems by mass";
				calc rows : HeavySubsystems {
					in root = telescope;
				}
			}

			part items : List {
				attribute redefines style = "number";
				calc entries : HeavySubsystems {
					in root = telescope;
				}
			}
		}
	}
}
`

// docDiagramModel declares a view over a connected part tree and a report
// whose one diagram draws it next to a query table.
const docDiagramModel = `package Imaging {
	private import Views::*;
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	port def DataPort;
	part def Camera { attribute mass : Real; port output : DataPort; }
	part def Recorder { attribute mass : Real; port input : DataPort; }

	part imagingChain {
		part camera : Camera { attribute redefines mass = 2.5; }
		part recorder : Recorder { attribute redefines mass = 4.0; }
		connection link connect camera.output to recorder.input;
	}

	view chainView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(
			source = WhereType(source = OwnedElements(source = root), type = "PartUsage"),
			properties = ("name", "mass")
		)
	}

	part def ChainReport :> Document {
		attribute redefines title = "Imaging Chain";

		part chain : Diagram {
			attribute redefines caption = "The imaging chain";
			ref redefines source = chainView;
		}

		part masses : Table {
			calc rows : Parts {
				in root = imagingChain;
			}
		}
	}
}
`

func docRenderSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(docRenderModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

func TestRenderDocumentPrintsMarkdown(t *testing.T) {
	s := docRenderSession(t)
	got := run(t, s, "%render-document Reports::MassReport")
	wants(t, got,
		"# Telescope Mass Report",
		"Mass rollup for the telescope assembly.",
		"## Heavy Subsystems",
		"<!-- caption -->\n*Heavy subsystems by mass*",
		"| name | mass |",
		"| --- | --- |",
		"| mount | 15 |",
		"| segmentControl | 20 |",
	)
	if strings.Index(got, "mount") > strings.Index(got, "segmentControl") {
		t.Errorf("rows are not in the query's order:\n%s", got)
	}
}

func TestRenderDocumentUsageAndErrors(t *testing.T) {
	s := docRenderSession(t)
	wants(t, run(t, s, "%render-document"), renderDocumentUsage)
	wants(t, run(t, s, "%render-document NoSuchDocument"), "error:", "NoSuchDocument")
	wants(t, run(t, s, "%render-document Observatory::HeavySubsystems"),
		"error:", "not a document", "DocumentQueries::Document")
	wants(t, run(t, s, "%render-document Reports::MassReport root=telescope"),
		"error:", "binds its queries' parameters in the model")
	wants(t, run(t, s, "%render-document Reports::MassReport svg"),
		"error:", `"svg" is not a diagram form (mermaid, dot)`)
	wants(t, run(t, s, "%render-document Reports::MassReport dot extra"), renderDocumentUsage)
}

// TestRenderDocumentDiagramForm writes the document's graph-shaped diagram as
// Mermaid by default and as DOT when asked; the table stays a pipe table.
func TestRenderDocumentDiagramForm(t *testing.T) {
	s := docRenderSession(t)
	if res := s.Submit(docDiagramModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	mermaid := run(t, s, "%render-document Imaging::ChainReport")
	wants(t, mermaid, "```mermaid\n", "flowchart", "| name | mass |")
	if strings.Contains(mermaid, "```dot") {
		t.Errorf("default rendering is DOT:\n%s", mermaid)
	}
	wants(t, run(t, s, "%render-document Imaging::ChainReport mermaid"), "```mermaid\n")
	dot := run(t, s, "%render-document Imaging::ChainReport dot")
	wants(t, dot,
		"```dot\n// view: Imaging::chainView\n// kind: interconnection\n",
		"digraph \"Imaging::chainView\" {",
		`"n1" -> "n3" [label="link", arrowhead=none, ltail="cluster_n1", lhead="cluster_n3"];`,
		"| camera | 2.5 |",
		"| name | mass |",
	)
	if strings.Contains(dot, "```mermaid") {
		t.Errorf("a diagram is still Mermaid under dot:\n%s", dot)
	}
	markdown, err := s.RenderDocumentMarkdown("Imaging::ChainReport", docrender.MarkdownOptions{DiagramForm: view.FormDot})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(markdown, "```dot\n") {
		t.Errorf("API rendering does not write DOT:\n%s", markdown)
	}
}

func TestRenderDocumentMarkdownAPI(t *testing.T) {
	s := docRenderSession(t)
	markdown, err := s.RenderDocumentMarkdown("Reports::MassReport", docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(markdown, "# Telescope Mass Report\n") {
		t.Errorf("markdown does not open with the title heading:\n%s", markdown)
	}
	if !strings.HasSuffix(markdown, "\n") {
		t.Errorf("markdown does not end with a newline")
	}
}

func TestRenderDocumentListedInHelpAndCompletion(t *testing.T) {
	s := docRenderSession(t)
	wants(t, run(t, s, "%help"), "%render-document <name> [mermaid|dot]", "Graphviz DOT")
	comp := s.Complete("%render-doc", len("%render-doc"))
	found := false
	for _, cand := range comp.Candidates {
		if cand == "%render-document" {
			found = true
		}
	}
	if !found {
		t.Errorf("%%render-document is not completed: %v", comp.Candidates)
	}
}
