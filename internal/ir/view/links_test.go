package view

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			}
		})
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
			links := Links{
				Template: "https://example.test/src/{file}#L{line}",
				Sites: renderer.Sites(func(origin Origin) (string, source.Pos, bool) {
					return origin.Doc, source.Pos{Line: 2, Col: 1}, origin.Located()
				}),
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
		})
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
