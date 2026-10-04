package view

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// renderSource renders view from one in-memory document.
func renderSource(t *testing.T, view, text string) *Rendering {
	t.Helper()
	r, idx := loadSources(t, []string{"inline.sysml"}, [][]byte{[]byte(text)})
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s: %v", view, err)
	}
	return rendering
}

const selectionModel = `package Model {
	private import StandardViewDefinitions::*;
	metadata def Safety;
	package Inner {
		part def Engine;
		requirement def Limit;
		requirement limit : Limit;
	}
	view def RequirementGraph :> GeneralView {
		filter @SysML::RequirementUsage;
	}
	view selecting : %s {
		%s
		expose Inner::*;
	}
}`

// A GeneralView's filters select the graph it is drawn as only when every
// condition is a metaclass classification or a disjunction of them; any other
// condition, and none, leaves today's tree.
func TestGeneralViewSpecializationSelection(t *testing.T) {
	cases := []struct {
		name, viewDef, members string
		want                   Kind
	}{
		{"no filter", "GeneralView", "", KindTree},
		{"requirement usage", "GeneralView", "filter @SysML::RequirementUsage;", KindRequirement},
		{"requirement definition", "GeneralView", "filter @SysML::RequirementDefinition;", KindRequirement},
		{"meta-classification", "GeneralView", "filter @@SysML::RequirementUsage;", KindRequirement},
		{"standard requirement view", "GeneralView", "filter @SysML::RequirementDefinition or @SysML::RequirementUsage or @SysML::Specialization or @SysML::FeatureTyping or @SysML::SatisfyRequirementUsage or @SysML::AllocationDefinition or @SysML::AllocationUsage;", KindRequirement},
		{"requirement beats definition", "GeneralView", "filter @SysML::Definition; filter @SysML::RequirementUsage;", KindRequirement},
		{"package", "GeneralView", "filter @SysML::Package or @SysML::Import;", KindPackage},
		{"package beats definition", "GeneralView", "filter @SysML::Package or @SysML::Definition;", KindPackage},
		{"definition and usage", "GeneralView", "filter @SysML::Definition or @SysML::Usage or @SysML::Specialization or @SysML::FeatureTyping;", KindDefinition},
		{"part definition", "GeneralView", "filter @SysML::PartDefinition;", KindDefinition},
		{"KerML metaclass", "GeneralView", "filter @KerML::Package;", KindPackage},
		{"inherited filter", "RequirementGraph", "", KindRequirement},
		{"relationships alone", "GeneralView", "filter @SysML::Specialization or @SysML::FeatureTyping;", KindTree},
		{"conjunction", "GeneralView", "filter @SysML::PartDefinition and @SysML::Definition;", KindTree},
		{"negation", "GeneralView", "filter not @SysML::Package;", KindTree},
		{"user metadata", "GeneralView", "filter @Safety;", KindTree},
		{"one unrecognized among recognized", "GeneralView", "filter @SysML::RequirementUsage; filter @Safety;", KindTree},
		{"other standard view", "InterconnectionView", "filter @SysML::RequirementUsage;", KindInterconnection},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := renderSource(t, "Model::selecting", fmt.Sprintf(selectionModel, tc.viewDef, tc.members))
			if rendering.Kind != tc.want {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.want)
			}
		})
	}
}

// A filtered expose selects a graph as a filter member does.
func TestGeneralViewFilteredExposeSelects(t *testing.T) {
	rendering := renderSource(t, "Model::packages", `package Model {
	private import StandardViewDefinitions::*;
	package Inner {
		package Deeper;
	}
	view packages : GeneralView {
		expose Model::**[@SysML::Package];
	}
}`)
	if rendering.Kind != KindPackage {
		t.Fatalf("kind = %q, want %q", rendering.Kind, KindPackage)
	}
	text := rendering.Text()
	for _, want := range []string{"package Model::Inner", "package Model::Inner::Deeper", "Model::Inner +-- Model::Inner::Deeper"} {
		if !strings.Contains(text, want) {
			t.Errorf("package rendering lacks %q:\n%s", want, text)
		}
	}
}

// A relationship end that resolves to nothing draws no edge and no node, is
// reported, and leaves the rest of the graph drawn.
func TestGeneralViewUnresolvedRelationshipEnds(t *testing.T) {
	rendering := renderSource(t, "Model::requirements", `package Model {
	private import StandardViewDefinitions::*;
	requirement def Limit :> MissingLimit;
	requirement limit : Limit;
	part vehicle {
		satisfy missingRequirement by vehicle;
		satisfy limit by missingPart;
	}
	view requirements : GeneralView {
		filter @SysML::RequirementUsage or @SysML::RequirementDefinition;
		expose Model::*;
	}
}`)
	if rendering.Kind != KindRequirement {
		t.Fatalf("kind = %q, want %q", rendering.Kind, KindRequirement)
	}
	text := rendering.Text()
	for _, want := range []string{"requirement def Model::Limit", "requirement Model::limit : Limit", "Model::limit ..|> Model::Limit"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"MissingLimit", "missingRequirement", "missingPart"} {
		if strings.Contains(strings.Split(text, "not represented:")[0], unwanted) {
			t.Errorf("rendering draws the unresolved %s:\n%s", unwanted, text)
		}
	}
	if len(rendering.Notices) == 0 {
		t.Errorf("no notice reports the unresolved ends:\n%s", text)
	}
	t.Log(strings.Join(rendering.Notices, "\n"))
}

// fixedVerdicts answers each requirement with verdicts of every kind, in a fixed
// order, so the overlay is checked without running a verification case.
func fixedVerdicts(req *symbols.Symbol) []Verdict {
	switch req.Name {
	case "vehicleMass":
		return []Verdict{{Case: "Cases::light", Kind: "pass"}, {Case: "Cases::heavy", Kind: "fail", Detail: "mass 2500"}}
	case "chassisMass":
		return []Verdict{{Case: "Cases::chassis", Kind: "pass"}}
	case "emergencyStop":
		return []Verdict{{Case: "Cases::stop", Kind: "inconclusive", Detail: "no verdict bound"}, {Case: "Cases::brake", Kind: "error", Detail: "division\nby zero"}}
	}
	return nil
}

// The verdict overlay colours each verified requirement by its worst verdict and
// labels it with every case's, in each form, under both drawing styles.
func TestGoldenVerdictOverlay(t *testing.T) {
	r, idx := loadFixtures(t, "general.sysml")
	r.SetVerdicts(fixedVerdicts)
	rendering, err := r.Render(lookup(t, idx, "GeneralViews::requirementView"))
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, filepath.Join("testdata", "general-verdicts.text.golden"), rendering.Text())
	for _, form := range []Form{FormMermaid, FormDot, FormPlantUML} {
		for _, style := range []DrawingStyle{StylePilot, StyleCameo} {
			artifact, err := rendering.WriteWith(form, Options{Style: style})
			if err != nil {
				t.Fatalf("write %s %s: %v", form, style, err)
			}
			checkGolden(t, filepath.Join("testdata", "general-verdicts-"+string(style)+"."+string(form)+".golden"), artifact)
		}
	}
	data := rendering.Data()
	verdicts := map[string]string{}
	for _, node := range data.Nodes {
		if node.Verdict != "" {
			verdicts[node.Name] = node.Verdict
		}
	}
	want := map[string]string{"VehicleRequirements::vehicleMass": "fail", "VehicleRequirements::chassisMass": "pass", "VehicleRequirements::emergencyStop": "error"}
	if fmt.Sprint(verdicts) != fmt.Sprint(want) {
		t.Errorf("node verdicts = %v, want %v", verdicts, want)
	}
}

// Without verdicts asked for, a requirement rendering is purely structural.
func TestRequirementRenderingWithoutOverlayHasNoVerdicts(t *testing.T) {
	rendering := render(t, "general.sysml", "GeneralViews::requirementView")
	for _, node := range rendering.Data().Nodes {
		if node.Verdict != "" {
			t.Errorf("%s has verdict %q without an overlay", node.Name, node.Verdict)
		}
	}
	if strings.Contains(rendering.Text(), "verdict") {
		t.Errorf("text mentions a verdict:\n%s", rendering.Text())
	}
}

// Only a requirement rendering takes the verdict overlay.
func TestOverlaySupport(t *testing.T) {
	for _, kind := range Kinds() {
		if got, want := kind.SupportsOverlay(OverlayVerdicts), kind == KindRequirement; got != want {
			t.Errorf("%s.SupportsOverlay(verdicts) = %v, want %v", kind, got, want)
		}
		if !kind.SupportsOverlay("") {
			t.Errorf("%s refuses no overlay", kind)
		}
	}
	if o, ok := ParseOverlay("verdicts"); !ok || o != OverlayVerdicts {
		t.Errorf("ParseOverlay(verdicts) = %q, %v", o, ok)
	}
	if _, ok := ParseOverlay("colours"); ok {
		t.Error("ParseOverlay accepted an unknown overlay")
	}
}

// A verification usage verifies what the objective it inherits names, unless it
// restates that objective.
func TestGeneralViewInheritedObjectiveVerifies(t *testing.T) {
	text := renderSource(t, "Model::requirements", `package Model {
	private import StandardViewDefinitions::*;
	requirement r;
	requirement s;
	verification def Check {
		objective {
			verify r;
		}
	}
	verification check : Check;
	verification recheck : Check {
		objective {
			verify s;
		}
	}
	view requirements : GeneralView {
		filter @SysML::RequirementUsage;
		expose Model::*;
	}
}`).Text()
	for _, want := range []string{"Model::Check ..> Model::r: verify", "Model::check ..> Model::r: verify", "Model::recheck ..> Model::s: verify"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Model::recheck ..> Model::r") {
		t.Errorf("recheck verifies the objective it restates:\n%s", text)
	}
}

// A feature subsetting the inherited feature of its own name draws its edge to
// that feature.
func TestGeneralViewInheritedSubsetting(t *testing.T) {
	text := renderSource(t, "Model::definitions", `package Model {
	private import StandardViewDefinitions::*;
	part def Base {
		part p;
	}
	part def Derived :> Base {
		part p :> p;
	}
	view definitions : GeneralView {
		filter @SysML::PartDefinition or @SysML::PartUsage;
		expose Model::**;
	}
}`).Text()
	if want := "Model::Derived::p --|> Model::Base::p: subsets"; !strings.Contains(text, want) {
		t.Errorf("rendering lacks %q:\n%s", want, text)
	}
}

// Each import of a member of a drawn package draws its own edge, named by the
// member.
func TestGeneralViewMemberImportsStayDistinct(t *testing.T) {
	rendering := renderSource(t, "Model::packages", `package Model {
	private import StandardViewDefinitions::*;
	package Q {
		part def A;
		part def B;
	}
	package P {
		import Q::A;
		import Q::B;
	}
	view packages : GeneralView {
		filter @SysML::Package;
		expose Model::**;
	}
}`)
	text := rendering.Text()
	for _, want := range []string{"Model::P ..> Model::Q: import ::A", "Model::P ..> Model::Q: import ::B"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendering lacks %q:\n%s", want, text)
		}
	}
	origins := map[Origin]bool{}
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeImport {
			origins[edge.Origin] = true
		}
	}
	if len(origins) != 2 {
		t.Errorf("import edges have %d distinct origins, want one per import declaration", len(origins))
	}
}

// A verdict recolours a requirement the view styles, keeping the rest of its
// Style and leaving the declared Style unchanged.
func TestVerdictOverlayRecoloursStyledRequirement(t *testing.T) {
	declared := &Style{Fill: "#0000FF", Line: "#0000FF", Text: "#FFFFFF", Font: "Courier"}
	node := &Node{Style: declared}
	g := &generalGraph{r: &Renderer{verdicts: fixedVerdicts}}
	g.overlayVerdicts(&symbols.Symbol{Name: "vehicleMass"}, node)
	want := Style{Fill: paletteFill("#D55E00", true), Line: "#D55E00", Text: "#FFFFFF", Font: "Courier"}
	if node.Verdict != "fail" || *node.Style != want {
		t.Errorf("verdict %q, style %+v; want fail, %+v", node.Verdict, *node.Style, want)
	}
	if node.verdictStyled {
		t.Error("a declared Style is marked as the verdict's")
	}
	if declared.Fill != "#0000FF" || declared.Line != "#0000FF" {
		t.Errorf("the declared Style changed: %+v", *declared)
	}
}
