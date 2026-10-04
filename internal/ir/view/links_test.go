package view

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParseLinkTemplate(t *testing.T) {
	for _, template := range []string{
		"https://example.com/{file}#L{line}:{col}?name={qname}&id={id}",
		"",
	} {
		if err := ParseLinkTemplate(template); err != nil {
			t.Errorf("ParseLinkTemplate(%q) = %v", template, err)
		}
	}
	for _, template := range []string{"https://example.com/{unknown}", "https://example.com/{"} {
		if err := ParseLinkTemplate(template); err == nil {
			t.Errorf("ParseLinkTemplate(%q) succeeded", template)
		}
	}
}

func TestLinkURLPercentEncodesSubstitutionsAndLiteralDelimiters(t *testing.T) {
	site := Site{
		File:          `dir/a b]\"#{}%é漢.sysml`,
		Line:          7,
		Col:           11,
		QualifiedName: `Pkg::'Pum p]\"#{}%é漢'`,
		ID:            `id /:#?&%é漢`,
	}
	links := Links{
		Template: "https://example.com/src/{file}#L{line}:{col}?name={qname}&id={id}%#?'[]",
		Sites: func(Origin) (Site, bool) {
			return site, true
		},
	}
	got, ok := links.URL(Origin{Doc: "model.sysml", Span: source.Span{Offset: 1, Len: 1}})
	if !ok {
		t.Fatal("URL returned no link")
	}
	want := "https://example.com/src/dir/a%20b%5D%5C%22%23%7B%7D%25%C3%A9%E6%BC%A2.sysml#L7:11?name=Pkg::%27Pum%20p%5D%5C%22%23%7B%7D%25%C3%A9%E6%BC%A2%27&id=id%20/:%23%3F%26%25%C3%A9%E6%BC%A2%#?'%5B%5D"
	if got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
	for _, forbidden := range []string{` `, `"`, `\`, `]`, `{`, `}`} {
		if strings.Contains(got, forbidden) {
			t.Errorf("URL contains forbidden %q: %s", forbidden, got)
		}
	}
}

func TestLinksDeclineUnlocatedAndMissingSites(t *testing.T) {
	located := Origin{Doc: "model.sysml", Span: source.Span{Offset: 1, Len: 1}}
	for name, links := range map[string]Links{
		"zero origin":  {Template: "https://example.com/{file}", Sites: func(Origin) (Site, bool) { return Site{}, true }},
		"nil sites":    {Template: "https://example.com/{file}"},
		"site miss":    {Template: "https://example.com/{file}", Sites: func(Origin) (Site, bool) { return Site{}, false }},
		"bad template": {Template: "https://example.com/{unknown}", Sites: func(Origin) (Site, bool) { return Site{}, true }},
	} {
		t.Run(name, func(t *testing.T) {
			origin := located
			if name == "zero origin" {
				origin = Origin{}
			}
			if url, ok := links.URL(origin); ok || url != "" {
				t.Errorf("URL = %q, %t; want no link", url, ok)
			}
		})
	}
}

func TestRendererSitesMemoizesAndUsesEffectiveIdentity(t *testing.T) {
	renderer, idx := loadFixtures(t, "tree.sysml")
	rendering, err := renderer.Render(lookup(t, idx, "VehicleViews::vehicleView"))
	if err != nil {
		t.Fatal(err)
	}
	origin := rendering.Roots[0].Origin
	calls := 0
	sites := renderer.Sites(func(got Origin) (string, source.Pos, bool) {
		calls++
		if got != origin {
			t.Errorf("locator origin = %+v, want %+v", got, origin)
		}
		return filepath.Join("workspace", got.Doc), source.Pos{Line: 3, Col: 5}, true
	})
	first, ok := sites(origin)
	if !ok {
		t.Fatal("Sites returned no source site")
	}
	second, ok := sites(origin)
	if !ok {
		t.Fatal("memoized Sites returned no source site")
	}
	if calls != 1 {
		t.Errorf("locator called %d times, want once per origin", calls)
	}
	if first != second {
		t.Errorf("memoized site = %+v, want %+v", second, first)
	}
	if first.File != "workspace/tree.sysml" || first.Line != 3 || first.Col != 5 {
		t.Errorf("site = %+v, want slash-normalized file and source position", first)
	}
	if first.QualifiedName == "" {
		t.Error("site has no qualified name")
	}
	sym := renderer.resolver.Index().DocumentRoot(origin.Doc).DeclaredAt(origin.Span)
	info, hasIdentity := identity.Of(renderer.model, renderer.resolver, sym)
	if !hasIdentity || info == nil {
		t.Fatal("identity.Of returned no effective identity for the rendered declaration")
	}
	if first.ID != info.EffectiveID {
		t.Errorf("ID = %q, want effective identity %q", first.ID, info.EffectiveID)
	}
}

func TestRendererSitesMemoizesConcurrentLookups(t *testing.T) {
	renderer, index := loadFixtures(t, "tree.sysml")
	rendering, err := renderer.Render(lookup(t, index, "VehicleViews::vehicleView"))
	if err != nil {
		t.Fatal(err)
	}
	origin := rendering.Roots[0].Origin
	var calls atomic.Int32
	sites := renderer.Sites(func(got Origin) (string, source.Pos, bool) {
		calls.Add(1)
		return got.Doc, source.Pos{Line: 1, Col: 1}, true
	})

	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, ok := sites(origin); !ok {
				t.Error("Sites returned no source site")
			}
		}()
	}
	workers.Wait()
	if calls.Load() != 1 {
		t.Errorf("locator called %d times, want once for concurrent lookups", calls.Load())
	}
}

func TestRendererSitesLeaveAnonymousQualifiedNameEmpty(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
	if err != nil {
		t.Fatal(err)
	}
	var origin Origin
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeFlow {
			origin = edge.Origin
			break
		}
	}
	if !origin.Located() {
		t.Fatal("flow edge has no located origin")
	}
	sites := renderer.Sites(func(got Origin) (string, source.Pos, bool) {
		return got.Doc, source.Pos{Line: 20, Col: 3}, true
	})
	site, ok := sites(origin)
	if !ok {
		t.Fatal("Sites returned no source site for the anonymous flow")
	}
	if site.QualifiedName != "" {
		t.Errorf("anonymous flow qualified name = %q, want empty", site.QualifiedName)
	}
	artifact, err := rendering.WriteWith(FormDot, Options{Links: Links{
		Template: "https://example.test/{file}#L{line}:{col}",
		Sites:    sites,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `tooltip="interconnection.sysml:20:3"`; !strings.Contains(artifact, want) {
		t.Errorf("anonymous flow tooltip does not fall back to its source position %q:\n%s", want, artifact)
	}
}

func TestFileLocatorRequiresOnDiskFileAndLineIndex(t *testing.T) {
	renderer, idx := loadFixtures(t, "tree.sysml")
	rendering, err := renderer.Render(lookup(t, idx, "VehicleViews::vehicleView"))
	if err != nil {
		t.Fatal(err)
	}
	origin := rendering.Roots[0].Origin
	lineIndex := fixtureText(t, "tree.sysml").Lines()
	locator := FileLocator(renderer.model, func(doc string) *source.LineIndex {
		if doc != origin.Doc {
			t.Errorf("line index requested for %q, want %q", doc, origin.Doc)
		}
		return lineIndex
	})
	file, pos, ok := locator(origin)
	if !ok || file != origin.Doc || pos != lineIndex.PosAt(origin.Span.Offset) {
		t.Errorf("FileLocator = %q, %+v, %t", file, pos, ok)
	}
	if _, _, ok := FileLocator(renderer.model, nil)(origin); ok {
		t.Error("FileLocator succeeded without line index")
	}
	renderer.model.SetSourceFile(func(string, source.Span) string { return "" })
	if _, _, ok := FileLocator(renderer.model, func(string) *source.LineIndex {
		return lineIndex
	})(origin); ok {
		t.Error("FileLocator succeeded without an on-disk source file")
	}
}

func TestWriteWithNoSitesIsByteIdentical(t *testing.T) {
	cases := []struct {
		file, name string
	}{
		{"interconnection.sysml", "PlantViews::loopView"},
		{"tree.sysml", "VehicleViews::vehicleView"},
		{"state.sysml", "MachineViews::vehicleStates"},
		{"action.sysml", "FlowViews::driveView"},
		{"sequence.sysml", "SequenceViews::pubSubView"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.name)
			for _, form := range rendering.Kind.SupportedForms() {
				plain, err := rendering.Write(form)
				if err != nil {
					t.Fatal(err)
				}
				for name, options := range map[string]Options{
					"zero links":             {},
					"template without sites": {Links: Links{Template: "https://example.com/{file}"}},
				} {
					got, err := rendering.WriteWith(form, options)
					if err != nil {
						t.Fatalf("%s, %s: %v", form, name, err)
					}
					if got != plain {
						t.Errorf("%s, %s changed output without source sites", form, name)
					}
				}
			}
		})
	}
}

func TestWriteWithRejectsInvalidLinkTemplate(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	_, err := rendering.WriteWith(FormMermaid, Options{Links: Links{Template: "https://example.com/{"}})
	if err == nil {
		t.Fatal("WriteWith succeeded with an unclosed link placeholder")
	}
	if !strings.Contains(err.Error(), "unclosed") {
		t.Errorf("error = %v, want an unclosed-placeholder error", err)
	}
}

func TestLinkedDiagramGoldens(t *testing.T) {
	cases := []struct {
		name, file, view string
		forms            []Form
	}{
		{"interconnection", "interconnection.sysml", "PlantViews::loopView", []Form{FormMermaid, FormDot, FormPlantUML}},
		{"tree", "tree.sysml", "VehicleViews::vehicleView", []Form{FormMermaid, FormDot, FormPlantUML}},
		{"state", "state.sysml", "MachineViews::vehicleStates", []Form{FormMermaid, FormDot, FormPlantUML}},
		{"action", "action.sysml", "FlowViews::driveView", []Form{FormMermaid, FormDot, FormPlantUML}},
		{"sequence", "sequence.sysml", "SequenceViews::pubSubView", []Form{FormMermaid, FormPlantUML}},
		{"case", "case.sysml", "CaseExamples::caseDiagram", []Form{FormMermaid, FormDot, FormPlantUML}},
		{"mixed", "mixed.sysml", "MixedExamples::mixedDiagram", []Form{FormMermaid, FormDot, FormPlantUML}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			renderer, index := loadSources(t, []string{tc.file}, [][]byte{content})
			rendering, err := renderer.Render(lookup(t, index, tc.view))
			if err != nil {
				t.Fatal(err)
			}
			lines := map[string]*source.LineIndex{tc.file: source.NewLineIndex(content)}
			sites := renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
				return lines[doc]
			}))
			options := Options{Links: Links{
				Template: "https://example.test/src/{file}#L{line}",
				Sites:    sites,
			}}
			for _, form := range tc.forms {
				got, err := rendering.WriteWith(form, options)
				if err != nil {
					t.Fatalf("write %s: %v", form, err)
				}
				path := filepath.Join("testdata", fmt.Sprintf("links-%s-%s.golden", tc.name, form))
				checkGolden(t, path, got)
				if form == FormMermaid {
					assertMermaidClickTargetsDeclared(t, got)
				}
				if tc.name == "case" {
					assertLinkedDiagramNode(t, rendering, got, form, options.Links, func(node *Node) bool {
						return caseNodeKind(node.Kind)
					}, "provide transportation")
					assertLinkedDiagramNode(t, rendering, got, form, options.Links, func(node *Node) bool {
						return node.Kind == "actor"
					}, "driver")
					if form == FormPlantUML {
						assertLinkedObjectiveNote(t, rendering, got, options.Links)
					}
				} else if tc.name == "mixed" {
					assertLinkedDiagramNode(t, rendering, got, form, options.Links, func(node *Node) bool {
						return node.Kind == "part"
					}, "b")
					if form == FormPlantUML {
						assertLinkedDiagramNode(t, rendering, got, form, options.Links, func(node *Node) bool {
							return node.Kind == "initial"
						}, "")
					}
				}
			}
		})
	}
}

func assertLinkedObjectiveNote(t *testing.T, rendering *Rendering, diagram string, links Links) {
	t.Helper()
	var objective *Node
	var walk func(*Node)
	walk = func(node *Node) {
		if objective != nil {
			return
		}
		if node.Kind == "objective" {
			objective = node
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if objective == nil {
		t.Fatal("no objective node in case rendering")
	}
	url, ok := links.URL(objective.Origin)
	if !ok {
		t.Fatal("objective node has no source URL")
	}
	lines := strings.Split(diagram, "\n")
	for i, line := range lines {
		if !strings.Contains(strings.TrimSpace(line), "note as "+objective.ID) {
			continue
		}
		for _, body := range lines[i+1:] {
			if strings.TrimSpace(body) == "end note" {
				break
			}
			if strings.Contains(body, "[["+url+" ") {
				return
			}
		}
		t.Errorf("PlantUML objective note has no linked body for %q:\n%s", url, diagram)
		return
	}
	t.Errorf("PlantUML has no note for objective %q:\n%s", objective.ID, diagram)
}

func assertLinkedDiagramNode(t *testing.T, rendering *Rendering, diagram string, form Form, links Links, matchesKind func(*Node) bool, namePart string) {
	t.Helper()
	namePart = strings.ToLower(namePart)
	var found *Node
	var walk func(*Node)
	walk = func(node *Node) {
		if found != nil {
			return
		}
		if matchesKind(node) && strings.Contains(strings.ToLower(node.Name), namePart) {
			found = node
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if found == nil {
		t.Fatalf("no rendered node matching %q", namePart)
	}
	url, ok := links.URL(found.Origin)
	if !ok {
		t.Fatalf("node %q has no source URL", found.Name)
	}
	switch form {
	case FormMermaid:
		if !strings.Contains(diagram, fmt.Sprintf(`click %s href "%s"`, found.ID, url)) {
			t.Errorf("Mermaid has no click link for %q (%s):\n%s", found.Name, found.ID, diagram)
		}
	case FormDot:
		for _, line := range strings.Split(diagram, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, `"`+found.ID+`" [`) && strings.Contains(line, `URL="`+url+`"`) && strings.Contains(line, `tooltip="`) {
				return
			}
		}
		t.Errorf("DOT has no URL and tooltip for %q (%s):\n%s", found.Name, found.ID, diagram)
	case FormPlantUML:
		for _, line := range strings.Split(diagram, "\n") {
			if strings.Contains(line, " as "+found.ID+" ") && strings.Contains(line, "[["+url+"]]") {
				return
			}
		}
		t.Errorf("PlantUML has no source link for %q (%s):\n%s", found.Name, found.ID, diagram)
	}
}

func TestPlantUMLCaseObjectiveNoteLinkRendersAsSVG(t *testing.T) {
	renderer, index := loadFixtures(t, "case.sysml")
	rendering, err := renderer.Render(lookup(t, index, "CaseExamples::caseDiagram"))
	if err != nil {
		t.Fatal(err)
	}
	lineIndex := fixtureText(t, "case.sysml").Lines()
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
			if doc != "case.sysml" {
				return nil
			}
			return lineIndex
		})),
	}
	var objective *Node
	var walk func(*Node)
	walk = func(node *Node) {
		if objective != nil {
			return
		}
		if node.Kind == "objective" {
			objective = node
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if objective == nil {
		t.Fatal("no objective node in case rendering")
	}
	url, ok := links.URL(objective.Origin)
	if !ok {
		t.Fatal("objective node has no source URL")
	}
	input, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	svg := renderLinkedSVG(t, FormPlantUML, input)
	if !svgAnchorContainsText(svg, url, "Transport") {
		t.Errorf("PlantUML SVG has no objective-text anchor for %q:\n%s", url, svg)
	}
	if !svgAnchorContainsText(svg, url, "objective") {
		t.Errorf("PlantUML SVG has no objective-title anchor for %q:\n%s", url, svg)
	}
	if !svgAnchorContainsBoldText(svg, url, "objective") {
		t.Errorf("PlantUML SVG does not render the objective title in bold inside %q:\n%s", url, svg)
	}
	if bytes.Contains(svg, []byte("[[")) {
		t.Errorf("PlantUML SVG contains literal Creole link syntax:\n%s", svg)
	}
	if bytes.Contains(svg, []byte("**")) {
		t.Errorf("PlantUML SVG contains literal bold syntax:\n%s", svg)
	}
}

func TestPlantUMLLinkedObjectiveNotePreservesParagraphBreaks(t *testing.T) {
	const model = `package ObjectiveParagraphExamples {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import OpenSysMLRenderings::*;

	use case def Sample {
		objective {
			doc /* First step.

			Second step. */
		}
	}

	view diagram {
		expose Sample;
		render asCaseDiagram;
	}
}`
	renderer, index := loadSources(t, []string{"paragraphs.sysml"}, [][]byte{[]byte(model)})
	rendering, err := renderer.Render(lookup(t, index, "ObjectiveParagraphExamples::diagram"))
	if err != nil {
		t.Fatal(err)
	}
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(origin Origin) (Site, bool) {
			return Site{File: origin.Doc, Line: 1, Col: 1}, true
		},
	}
	diagram, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	const url = "https://example.test/src/paragraphs.sysml#L1"
	want := fmt.Sprintf("[[%s First step.]]\n  \n  [[%s Second step.]]", url, url)
	if !strings.Contains(diagram, want) {
		t.Fatalf("PlantUML objective note lost its paragraph break:\n%s", diagram)
	}
	svg := renderLinkedSVG(t, FormPlantUML, diagram)
	firstY, ok := svgAnchorTextY(svg, url, "First")
	if !ok {
		t.Fatalf("PlantUML SVG has no linked first paragraph:\n%s", svg)
	}
	secondY, ok := svgAnchorTextY(svg, url, "Second")
	if !ok {
		t.Fatalf("PlantUML SVG has no linked second paragraph:\n%s", svg)
	}
	if secondY-firstY < 25 {
		t.Errorf("PlantUML SVG did not render a paragraph gap: first y=%g, second y=%g", firstY, secondY)
	}
}

func TestPlantUMLMixedInitialCircleLinkRendersAsSVG(t *testing.T) {
	renderer, index := loadFixtures(t, "mixed.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedExamples::mixedDiagram"))
	if err != nil {
		t.Fatal(err)
	}
	lineIndex := fixtureText(t, "mixed.sysml").Lines()
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
			if doc != "mixed.sysml" {
				return nil
			}
			return lineIndex
		})),
	}
	var initial *Node
	var walk func(*Node)
	walk = func(node *Node) {
		if initial != nil {
			return
		}
		if node.Kind == "initial" {
			initial = node
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if initial == nil {
		t.Fatal("no initial control node in mixed rendering")
	}
	url, ok := links.URL(initial.Origin)
	if !ok {
		t.Fatal("mixed initial control node has no source URL")
	}
	if !strings.HasSuffix(url, "#L35") {
		t.Fatalf("mixed initial control node URL = %q, want source line 35", url)
	}
	input, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	svg := renderLinkedSVG(t, FormPlantUML, input)
	if !svgAnchorHasHref(svg, url) {
		t.Errorf("PlantUML SVG has no anchor for mixed initial control URL %q:\n%s", url, input)
	}
}

func TestMermaidFlowchartDoesNotLinkUsedPortSubgraph(t *testing.T) {
	renderer, index := loadFixtures(t, "action.sysml")
	rendering, err := renderer.Render(lookup(t, index, "FlowViews::driveView"))
	if err != nil {
		t.Fatal(err)
	}
	used := rendering.usedPorts(rendering.portView(PortsMinimal))
	var target *Node
	var find func(*Node)
	find = func(node *Node) {
		if target != nil {
			return
		}
		if len(node.Children) == 0 && rendering.hasUsedPorts(node, used) {
			target = node
			return
		}
		for _, child := range node.Children {
			find(child)
		}
	}
	for _, root := range rendering.Roots {
		find(root)
	}
	if target == nil {
		t.Fatal("action fixture has no childless node with a used port")
	}

	lineIndex := fixtureText(t, "action.sysml").Lines()
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
			if doc != "action.sysml" {
				return nil
			}
			return lineIndex
		})),
	}
	if _, ok := links.URL(target.Origin); !ok {
		t.Fatalf("used-port action %q has no source URL", target.ID)
	}
	input, err := rendering.WriteWith(FormMermaid, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	if want := "subgraph " + target.ID + " ["; !strings.Contains(input, want) {
		t.Errorf("used-port action %q is not drawn as a subgraph:\n%s", target.ID, input)
	}
	if unexpected := "click " + target.ID + " "; strings.Contains(input, unexpected) {
		t.Errorf("Mermaid flowchart linked subgraph ID %q:\n%s", target.ID, input)
	}
}

func TestMermaidStateLinksSkipImplicitPseudostates(t *testing.T) {
	origin := Origin{Doc: "machine.sysml", Span: source.Span{Offset: 1, Len: 1}}
	start := &Node{ID: "n1", Kind: startKind, Origin: origin}
	active := &Node{ID: "n2", Kind: "state", Name: "active", Origin: origin}
	final := &Node{ID: "n3", Kind: "final", Origin: origin}
	rendering := &Rendering{
		Kind: KindState,
		Roots: []*Node{{
			ID: "n0", Kind: "state", Name: "machine", Origin: origin,
			Children: []*Node{start, active, final},
		}},
		Edges: []Edge{
			{From: start.ID, To: active.ID, Kind: EdgeTransition},
			{From: active.ID, To: final.ID, Kind: EdgeTransition},
		},
	}
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(got Origin) (Site, bool) {
			return Site{File: got.Doc, Line: 1, Col: 1}, got.Located()
		},
	}
	got, err := rendering.WriteWith(FormMermaid, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "[*] --> "+active.ID) || !strings.Contains(got, active.ID+" --> [*]") {
		t.Errorf("test start and final were not converted to [*]:\n%s", got)
	}
	for _, id := range []string{start.ID, final.ID} {
		if unexpected := "click " + id + " "; strings.Contains(got, unexpected) {
			t.Errorf("Mermaid state linked undeclared pseudostate %q:\n%s", id, got)
		}
	}
	if expected := "click " + active.ID + " "; !strings.Contains(got, expected) {
		t.Errorf("Mermaid state omitted the declared state's link %q:\n%s", active.ID, got)
	}
	assertMermaidClickTargetsDeclared(t, got)
}

func assertMermaidClickTargetsDeclared(t *testing.T, diagram string) {
	t.Helper()
	flowchartNode := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*(\[|\(|\{|@)`)
	stateNode := regexp.MustCompile(`^state[[:space:]]+".*"[[:space:]]+as[[:space:]]+([A-Za-z_][A-Za-z0-9_]*)`)
	declared := map[string]bool{}
	for _, line := range strings.Split(diagram, "\n") {
		line = strings.TrimSpace(line)
		if match := stateNode.FindStringSubmatch(line); len(match) > 1 {
			declared[match[1]] = true
		} else if match := flowchartNode.FindStringSubmatch(line); len(match) > 1 {
			declared[match[1]] = true
		}
	}
	for _, line := range strings.Split(diagram, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "click" && !declared[fields[1]] {
			t.Errorf("Mermaid click targets undeclared node %q in:\n%s", fields[1], diagram)
		}
	}
}

func TestPlantUMLLinksPrecedeColorsAndExcludePorts(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection-ports.sysml")
	rendering, err := renderer.Render(lookup(t, index, "ToasterViews::toasterView"))
	if err != nil {
		t.Fatal(err)
	}
	options := Options{
		Palette: PaletteOkabeIto,
		Links: Links{
			Template: "https://example.test/{file}#L{line}",
			Sites: renderer.Sites(func(origin Origin) (string, source.Pos, bool) {
				return origin.Doc, source.Pos{Line: 4, Col: 2}, origin.Located()
			}),
		},
	}
	got, err := rendering.WriteWith(FormPlantUML, options)
	if err != nil {
		t.Fatal(err)
	}
	foundLink, foundColor := false, false
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "port ") && strings.Contains(line, "[[") {
			t.Errorf("PlantUML port carries a link: %s", line)
		}
		link := strings.Index(line, " [[")
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "rectangle ")) && link >= 0 {
			foundLink = true
			stereotype := strings.Index(line, ">>")
			if stereotype < 0 || stereotype > link {
				t.Errorf("link is not after stereotypes: %s", line)
			}
			if color := strings.Index(line, " #"); color >= 0 {
				foundColor = true
				if link > color {
					t.Errorf("link follows color: %s", line)
				}
			}
		}
	}
	if !foundLink || !foundColor {
		t.Errorf("PlantUML output did not exercise a linked colored node:\n%s", got)
	}
}

func TestPlantUMLPseudostateLinksAreOmittedWhenSVGDropsThem(t *testing.T) {
	origin := Origin{Doc: "model.sysml", Span: source.Span{Offset: 1, Len: 1}}
	pseudostates := []struct {
		kind, stereotype string
	}{
		{"initial", "start"},
		{"fork", "fork"},
		{"join", "join"},
		{"final", "end"},
		{"decision", "choice"},
		{"choice", "choice"},
		{"merge", "choice"},
		{"junction", "choice"},
		{"shallow history", "history"},
		{"deep history", "history*"},
	}
	for _, diagramKind := range []Kind{KindState, KindAction} {
		t.Run(string(diagramKind), func(t *testing.T) {
			writer := plantumlWriter{kind: diagramKind, links: Links{
				Template: "https://example.test/{file}",
				Sites: func(Origin) (Site, bool) {
					return Site{File: "model.sysml", Line: 1, Col: 1}, true
				},
			}}
			for _, tc := range pseudostates {
				t.Run(tc.kind, func(t *testing.T) {
					got := writer.decoration(&Node{Kind: tc.kind, Origin: origin})
					if want := " <<" + tc.stereotype + ">>"; got != want {
						t.Errorf("decoration = %q, want unlinked pseudostate %q", got, want)
					}
				})
			}
		})
	}
}

func TestPlantUMLNoteLinkTextEscapesClosingBracketsInSVG(t *testing.T) {
	const text = "part]name"
	if got, want := plantumlNoteLinkText(text), "part~]name"; got != want {
		t.Fatalf("plantumlNoteLinkText(%q) = %q, want %q", text, got, want)
	}
	const url = "https://example.test/note"
	input := fmt.Sprintf("@startuml\nnote as n1\n  [[%s %s]]\nend note\n@enduml\n", url, plantumlNoteLinkText(text))
	svg := renderLinkedSVG(t, FormPlantUML, input)
	if !svgAnchorContainsText(svg, url, text) {
		t.Errorf("PlantUML SVG has no link for note text %q:\n%s", text, svg)
	}
	if bytes.Contains(svg, []byte("[[")) {
		t.Errorf("PlantUML SVG contains literal Creole link syntax:\n%s", svg)
	}
}

func TestLinkWritersDoNotLinkZeroOrigins(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
	if err != nil {
		t.Fatal(err)
	}
	zero := Origin{}
	links := Links{
		Template: "https://example.test/{file}",
		Sites: func(Origin) (Site, bool) {
			return Site{File: "model.sysml"}, true
		},
	}
	if url, ok := links.URL(zero); ok || url != "" {
		t.Fatalf("zero origin URL = %q, %t", url, ok)
	}
	copy := *rendering
	copy.Roots = []*Node{{ID: "n0", Kind: "part", Name: "synthetic", Origin: zero}}
	copy.Edges = []Edge{{From: "n0", To: "n0", Kind: EdgeConnection, Origin: zero}}
	for _, form := range []Form{FormMermaid, FormDot, FormPlantUML} {
		got, err := copy.WriteWith(form, Options{Links: links})
		if err != nil {
			t.Errorf("%s: %v", form, err)
			continue
		}
		if strings.Contains(got, "https://example.test/") {
			t.Errorf("%s linked a zero origin:\n%s", form, got)
		}
	}
}

func TestLinkedFormsRenderAsSVG(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
	if err != nil {
		t.Fatal(err)
	}
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(func(origin Origin) (string, source.Pos, bool) {
			return origin.Doc, source.Pos{Line: 2, Col: 1}, origin.Located()
		}),
	}
	for _, form := range []Form{FormDot, FormPlantUML, FormMermaid} {
		t.Run(string(form), func(t *testing.T) {
			input, err := rendering.WriteWith(form, Options{Links: links})
			if err != nil {
				t.Fatal(err)
			}
			svg := renderLinkedSVG(t, form, input)
			svgText := string(svg)
			if !strings.Contains(svgText, "xlink:href=") || strings.Contains(svgText, "Syntax Error") {
				t.Errorf("%s SVG has no linked anchor:\nsource:\n%s\nsvg:\n%s", form, input, svg)
			}
		})
	}
}

func TestDOTCompositeLinkRendersAsVisibleClusterAnchor(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	rendering, err := renderer.Render(lookup(t, index, "PlantViews::loopView"))
	if err != nil {
		t.Fatal(err)
	}
	cluster := rendering.Roots[0]
	if len(cluster.Children) == 0 {
		t.Fatal("fixture root is not a composite cluster")
	}
	lineIndex := fixtureText(t, "interconnection.sysml").Lines()
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
			if doc != "interconnection.sysml" {
				return nil
			}
			return lineIndex
		})),
	}
	clusterURL, ok := links.URL(cluster.Origin)
	if !ok {
		t.Fatal("composite cluster has no source URL")
	}
	input, err := rendering.WriteWith(FormDot, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	svg := renderLinkedSVG(t, FormDot, input)
	if !svgAnchorContainsElement(svg, clusterURL, "polygon") {
		t.Errorf("composite URL %q is not an SVG anchor around visible cluster content:\n%s", clusterURL, svg)
	}
}

func TestPlantUMLLinkedArrowFormsRenderAsSVG(t *testing.T) {
	for _, tc := range []struct {
		file, name string
	}{
		{"interconnection-ports.sysml", "ToasterViews::toasterView"},
		{"state.sysml", "MachineViews::vehicleStates"},
		{"action.sysml", "FlowViews::driveView"},
		{"sequence.sysml", "SequenceViews::pubSubView"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			renderer, index := loadFixtures(t, tc.file)
			rendering, err := renderer.Render(lookup(t, index, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			lineIndex := fixtureText(t, tc.file).Lines()
			links := Links{
				Template: "https://example.test/src/{file}#L{line}",
				Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
					if doc != tc.file {
						return nil
					}
					return lineIndex
				})),
			}
			input, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
			if err != nil {
				t.Fatal(err)
			}
			svg := renderLinkedSVG(t, FormPlantUML, input)
			svgText := string(svg)
			if !strings.Contains(svgText, "xlink:href=") || strings.Contains(svgText, "Syntax Error") {
				t.Errorf("PlantUML SVG has no linked anchor:\nsource:\n%s\nsvg:\n%s", input, svg)
			}
			if tc.file == "state.sysml" || tc.file == "action.sysml" {
				for _, url := range plantUMLLinkURLs(input, false) {
					if !svgAnchorHasHref(svg, url) {
						t.Errorf("PlantUML SVG has no anchor for emitted link %q", url)
					}
				}
				for _, url := range plantUMLLinkURLs(input, true) {
					if !svgAnchorHasHref(svg, url) {
						t.Errorf("PlantUML SVG swallowed a child's own link %q inside a linked composite", url)
					}
				}
			}
		})
	}
}

func TestMermaidSequenceLinkFragmentsAreDroppedInSVG(t *testing.T) {
	renderer, index := loadFixtures(t, "sequence.sysml")
	rendering, err := renderer.Render(lookup(t, index, "SequenceViews::pubSubView"))
	if err != nil {
		t.Fatal(err)
	}
	lineIndex := fixtureText(t, "sequence.sysml").Lines()
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: renderer.Sites(FileLocator(renderer.model, func(doc string) *source.LineIndex {
			if doc != "sequence.sysml" {
				return nil
			}
			return lineIndex
		})),
	}
	sourceURL, ok := links.URL(rendering.Roots[0].Origin)
	if !ok || !strings.Contains(sourceURL, "#L") {
		t.Fatalf("sequence participant source URL = %q, %t; want URL with a line fragment", sourceURL, ok)
	}
	input, err := rendering.WriteWith(FormMermaid, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	svg := renderLinkedSVG(t, FormMermaid, input)
	expected := strings.SplitN(sourceURL, "#", 2)[0]
	hrefs := svgAnchorHrefs(svg)
	found := false
	for _, href := range hrefs {
		if href == expected {
			found = true
		}
		if strings.HasPrefix(href, expected+"#L") {
			t.Errorf("Mermaid sequence SVG retained the fragment unexpectedly: %q", href)
		}
	}
	if !found {
		t.Errorf("Mermaid sequence SVG does not contain fragmentless href %q; got %q", expected, hrefs)
	}
}

func plantUMLLinkURLs(input string, nodesOnly bool) []string {
	pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	var urls []string
	for _, line := range strings.Split(input, "\n") {
		if nodesOnly && !strings.HasPrefix(strings.TrimSpace(line), "state ") {
			continue
		}
		for _, match := range pattern.FindAllStringSubmatch(line, -1) {
			urls = append(urls, match[1])
		}
	}
	return urls
}

func svgAnchorHrefs(svg []byte) []string {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	var hrefs []string
	for {
		token, err := decoder.Token()
		if err != nil {
			return hrefs
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local != "a" {
			continue
		}
		for _, attr := range element.Attr {
			if attr.Name.Local == "href" {
				hrefs = append(hrefs, attr.Value)
			}
		}
	}
}

func svgAnchorHasHref(svg []byte, href string) bool {
	for _, got := range svgAnchorHrefs(svg) {
		if got == href {
			return true
		}
	}
	return false
}

func svgAnchorContainsElement(svg []byte, href, child string) bool {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	inTargetAnchor := false
	anchorDepth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch element := token.(type) {
		case xml.StartElement:
			if inTargetAnchor {
				if element.Name.Local == child {
					return true
				}
				if element.Name.Local == "a" {
					anchorDepth++
				}
				continue
			}
			if element.Name.Local != "a" {
				continue
			}
			for _, attr := range element.Attr {
				if attr.Name.Local == "href" && attr.Value == href {
					inTargetAnchor = true
					anchorDepth = 1
					break
				}
			}
		case xml.EndElement:
			if inTargetAnchor && element.Name.Local == "a" {
				anchorDepth--
				if anchorDepth == 0 {
					inTargetAnchor = false
				}
			}
		}
	}
}

func svgAnchorContainsText(svg []byte, href, text string) bool {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	inTargetAnchor := false
	anchorDepth := 0
	var content strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch element := token.(type) {
		case xml.StartElement:
			if inTargetAnchor {
				if element.Name.Local == "a" {
					anchorDepth++
				}
				continue
			}
			if element.Name.Local != "a" {
				continue
			}
			for _, attr := range element.Attr {
				if attr.Name.Local == "href" && attr.Value == href {
					inTargetAnchor = true
					anchorDepth = 1
					content.Reset()
					break
				}
			}
		case xml.CharData:
			if inTargetAnchor {
				content.Write([]byte(element))
				if strings.Contains(content.String(), text) {
					return true
				}
			}
		case xml.EndElement:
			if inTargetAnchor && element.Name.Local == "a" {
				anchorDepth--
				if anchorDepth == 0 {
					inTargetAnchor = false
					content.Reset()
				}
			}
		}
	}
}

func svgAnchorContainsBoldText(svg []byte, href, text string) bool {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	inTargetAnchor := false
	inText := false
	bold := false
	var content strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch element := token.(type) {
		case xml.StartElement:
			if !inTargetAnchor && element.Name.Local == "a" {
				for _, attr := range element.Attr {
					if attr.Name.Local == "href" && attr.Value == href {
						inTargetAnchor = true
						break
					}
				}
			} else if inTargetAnchor && element.Name.Local == "text" {
				inText = true
				bold = false
				content.Reset()
				for _, attr := range element.Attr {
					switch attr.Name.Local {
					case "font-weight":
						bold = attr.Value == "700" || attr.Value == "bold"
					case "style":
						style := strings.ToLower(attr.Value)
						bold = bold || strings.Contains(style, "font-weight:700") || strings.Contains(style, "font-weight:bold")
					}
				}
			}
		case xml.CharData:
			if inText {
				content.Write([]byte(element))
			}
		case xml.EndElement:
			if inTargetAnchor && inText && element.Name.Local == "text" {
				if bold && strings.Contains(content.String(), text) {
					return true
				}
				inText = false
			}
			if inTargetAnchor && element.Name.Local == "a" {
				inTargetAnchor = false
			}
		}
	}
}

func svgAnchorTextY(svg []byte, href, text string) (float64, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	inTargetAnchor := false
	inText := false
	var y float64
	hasY := false
	var content strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return 0, false
		}
		switch element := token.(type) {
		case xml.StartElement:
			if !inTargetAnchor && element.Name.Local == "a" {
				for _, attr := range element.Attr {
					if attr.Name.Local == "href" && attr.Value == href {
						inTargetAnchor = true
						break
					}
				}
			} else if inTargetAnchor && element.Name.Local == "text" {
				inText = true
				hasY = false
				content.Reset()
				for _, attr := range element.Attr {
					if attr.Name.Local == "y" {
						var err error
						y, err = strconv.ParseFloat(attr.Value, 64)
						if err != nil {
							return 0, false
						}
						hasY = true
						break
					}
				}
			}
		case xml.CharData:
			if inText {
				content.Write([]byte(element))
			}
		case xml.EndElement:
			if inTargetAnchor && inText && element.Name.Local == "text" {
				if strings.Contains(content.String(), text) {
					return y, hasY
				}
				inText = false
			}
			if inTargetAnchor && element.Name.Local == "a" {
				inTargetAnchor = false
			}
		}
	}
}

func renderLinkedSVG(t *testing.T, form Form, input string) []byte {
	t.Helper()
	switch form {
	case FormDot:
		binary, err := exec.LookPath("dot")
		if err != nil {
			t.Skip("Graphviz dot is not installed")
		}
		command := exec.Command(binary, "-Tsvg")
		command.Stdin = strings.NewReader(input)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("dot -Tsvg: %v\n%s", err, output)
		}
		return output
	case FormPlantUML:
		jar := os.Getenv("OPENSYSML_PLANTUML_JAR")
		if jar == "" {
			jar = filepath.Join("..", "..", "..", "build", "doc-pdf", "plantuml", "plantuml-1.2026.8.jar")
		}
		if _, err := os.Stat(jar); err != nil {
			t.Skip("PlantUML jar is not installed")
		}
		java, err := exec.LookPath("java")
		if err != nil {
			t.Skip("java is not installed")
		}
		command := exec.Command(java, "-Djava.awt.headless=true", "-jar", jar, "-tsvg", "-pipe")
		command.Stdin = strings.NewReader(input)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("PlantUML SVG: %v\n%s", err, output)
		}
		return output
	case FormMermaid:
		binary := os.Getenv("OPENSYSML_MMDC")
		if binary == "" {
			var err error
			binary, err = exec.LookPath("mmdc")
			if err != nil {
				t.Skip("Mermaid CLI is not installed")
			}
		}
		dir := t.TempDir()
		inputPath, outputPath := filepath.Join(dir, "diagram.mmd"), filepath.Join(dir, "diagram.svg")
		if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-i", inputPath, "-o", outputPath}
		if puppeteer := os.Getenv("OPENSYSML_MMDC_PUPPETEER"); puppeteer != "" {
			args = append(args, "-p", puppeteer)
		}
		command := exec.Command(binary, args...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("mmdc: %v\n%s", err, output)
		}
		svg, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		return svg
	default:
		t.Fatalf("unsupported SVG form %s", form)
		return nil
	}
}
