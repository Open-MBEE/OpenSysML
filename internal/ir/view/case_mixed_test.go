package view

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestCaseAndMixedKindsSupportGraphForms(t *testing.T) {
	for _, kind := range []Kind{KindCase, KindMixed} {
		if !kind.Supported() || !kind.SupportsDirection() {
			t.Errorf("%s is not a supported directional kind", kind)
		}
		listed := false
		for _, candidate := range Kinds() {
			listed = listed || candidate == kind
		}
		if !listed {
			t.Errorf("%s is absent from Kinds()", kind)
		}
		if kind.MachineForm() != FormMermaid {
			t.Errorf("%s machine form = %s, want mermaid", kind, kind.MachineForm())
		}
		for _, form := range []Form{FormText, FormMermaid, FormDot, FormPlantUML} {
			if !kind.SupportsForm(form) {
				t.Errorf("%s does not support %s", kind, form)
			}
		}
		for _, form := range []Form{FormMarkdown, FormCSV, FormTSV} {
			if kind.SupportsForm(form) {
				t.Errorf("%s supports %s", kind, form)
			}
			rendering := &Rendering{Kind: kind}
			if _, err := rendering.Write(form); !errors.Is(err, ErrWrongForm) {
				t.Errorf("%s written as %s: error = %v, want ErrWrongForm", kind, form, err)
			}
		}
	}
	if got := string(KindCase); got != "case" {
		t.Errorf("KindCase = %q", got)
	}
	if got := string(KindMixed); got != "mixed" {
		t.Errorf("KindMixed = %q", got)
	}
	for kind, want := range map[EdgeKind]string{
		EdgeComposition: "composition", EdgeAssociation: "association", EdgeInclude: "include",
		EdgeAnchor: "anchor", EdgeTyping: "typing", EdgeSpecialization: "specialization",
		EdgeReference: "reference",
	} {
		if got := kind.String(); got != want {
			t.Errorf("%v.String() = %q, want %q", kind, got, want)
		}
	}
}

func TestCaseViewSelectionAndRelationships(t *testing.T) {
	rendering := render(t, "case.sysml", "CaseExamples::caseDiagram")
	if rendering.Kind != KindCase || rendering.View != "CaseExamples::caseDiagram" {
		t.Fatalf("rendering = kind %q view %q", rendering.Kind, rendering.View)
	}
	names := map[string][]*Node{}
	var visit func(*Node)
	visit = func(node *Node) {
		names[node.Name] = append(names[node.Name], node)
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, root := range rendering.Roots {
		visit(root)
	}
	for _, name := range []string{"CaseExamples::'Provide Transportation'", "CaseExamples::'Add Fuel'",
		"'enter vehicle'", "'Drive Vehicle'", "CaseExamples::FuelAnalysis", "CaseExamples::FuelVerification"} {
		if len(names[name]) == 0 {
			t.Errorf("case rendering has no node named %q; got %v", name, mapNodeNames(names))
		}
	}
	if len(names["CaseExamples::'Add Fuel'"]) != 1 {
		t.Errorf("included target appears %d times, want once", len(names["CaseExamples::'Add Fuel'"]))
	}
	var objectives []*Node
	for _, nodes := range names {
		for _, node := range nodes {
			if node.Kind == "objective" {
				objectives = append(objectives, node)
			}
		}
	}
	hasObjectiveDocumentation := false
	for _, objective := range objectives {
		hasObjectiveDocumentation = hasObjectiveDocumentation || strings.Contains(objective.Detail, "Transport the vehicle")
	}
	if !hasObjectiveDocumentation {
		t.Errorf("objective documentation is not on its node: %+v", objectives)
	}
	if len(rendering.Notes) != 0 {
		t.Errorf("objective documentation was put in layout notes: %+v", rendering.Notes)
	}
	edges := map[EdgeKind][]Edge{}
	for _, edge := range rendering.Edges {
		edges[edge.Kind] = append(edges[edge.Kind], edge)
	}
	for _, kind := range []EdgeKind{EdgeComposition, EdgeAssociation, EdgeInclude, EdgeAnchor} {
		if len(edges[kind]) == 0 {
			t.Errorf("case rendering has no %s edge", kind)
		}
	}
	var includedReference, includedDeclaration bool
	for _, edge := range edges[EdgeInclude] {
		if edge.Label != "«include»" {
			t.Errorf("include label = %q", edge.Label)
		}
		if len(names["CaseExamples::'Add Fuel'"]) == 1 && names["CaseExamples::'Add Fuel'"][0].ID == edge.To {
			includedReference = true
		}
		if len(names["'enter vehicle'"]) == 1 && names["'enter vehicle'"][0].ID == edge.To {
			includedDeclaration = true
		}
	}
	if !includedReference || !includedDeclaration {
		t.Errorf("include edges: reference=%t declaration=%t", includedReference, includedDeclaration)
	}
	for _, edge := range edges[EdgeAssociation] {
		if strings.Contains(edge.Label, "subject") && edge.Label != "«subject»" {
			t.Errorf("subject association label = %q", edge.Label)
		}
	}
	if !strings.Contains(rendering.Mermaid(), "flowchart LR\n") {
		t.Errorf("case Mermaid does not default to LR:\n%s", rendering.Mermaid())
	}
}

func TestCaseViewDefinitionSelectionAndEmptyNotice(t *testing.T) {
	rendering := render(t, "case.sysml", "CaseExamples::caseDefinition")
	if rendering.Kind != KindCase {
		t.Fatalf("CaseView selected kind %q, want case", rendering.Kind)
	}
	found := false
	for _, root := range rendering.Roots {
		if root.Name == "CaseExamples::'Provide Transportation'" {
			found = true
		}
	}
	if !found {
		t.Fatalf("CaseView has no case root: %+v", rendering.Roots)
	}

	r, idx := loadFixture(t, "case.sysml")
	tool := lookup(t, idx, "CaseExamples::Tool")
	empty, err := r.RenderExposed([]*symbols.Symbol{tool}, KindCase, "#case")
	if err != nil {
		t.Fatalf("RenderExposed: %v", err)
	}
	if len(empty.Notices) != 1 || !strings.Contains(empty.Notices[0], "holds no case; a case rendering does not show it") {
		t.Errorf("notice = %v", empty.Notices)
	}
}

func TestViewsDemoSelectsCaseAndMixedRenderings(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "views-demo.sysml"))
	if err != nil {
		t.Fatalf("read views demo: %v", err)
	}
	r, idx := loadSources(t, []string{"views-demo.sysml"}, [][]byte{content})
	for _, selection := range []struct {
		view string
		kind Kind
	}{
		{view: "LanderViews::useCases", kind: KindCase},
		{view: "LanderViews::mixedOverview", kind: KindMixed},
	} {
		view := lookup(t, idx, selection.view)
		kind, _, err := r.KindOf(view)
		if err != nil {
			t.Errorf("KindOf(%s): %v", selection.view, err)
		} else if kind != selection.kind {
			t.Errorf("KindOf(%s) = %s, want %s", selection.view, kind, selection.kind)
		}
		rendering, err := r.Render(view)
		if err != nil {
			t.Errorf("Render(%s): %v", selection.view, err)
			continue
		}
		assertRenderingEdgeEndpoints(t, rendering)
	}
}

func TestMixedViewContainsSharedKindsAndReferenceEdges(t *testing.T) {
	rendering := render(t, "mixed.sysml", "MixedExamples::mixedDiagram")
	if rendering.Kind != KindMixed || rendering.View != "MixedExamples::mixedDiagram" {
		t.Fatalf("rendering = kind %q view %q", rendering.Kind, rendering.View)
	}
	names := map[string][]*Node{}
	var visit func(*Node)
	visit = func(node *Node) {
		names[node.Name] = append(names[node.Name], node)
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, root := range rendering.Roots {
		visit(root)
	}
	for _, name := range []string{"MixedExamples::Plant", "pump", "MixedExamples::Modes",
		"MixedExamples::Operate", "Inspection", "operator", "MixedExamples::Ready", "ready"} {
		if len(names[name]) == 0 {
			t.Errorf("mixed rendering has no node named %q; got %v", name, mapNodeNames(names))
		}
	}
	kinds := map[EdgeKind]bool{}
	labels := map[string]bool{}
	for _, edge := range rendering.Edges {
		kinds[edge.Kind] = true
		labels[edge.Label] = true
	}
	for _, kind := range []EdgeKind{EdgeConnection, EdgeTyping, EdgeSpecialization, EdgeReference, EdgeAssociation, EdgeAnchor} {
		if !kinds[kind] {
			t.Errorf("mixed rendering has no %s edge", kind)
		}
	}
	for _, label := range []string{"«perform»", "«exhibit»", "«specializes»"} {
		if !labels[label] {
			t.Errorf("mixed rendering has no %s edge label", label)
		}
	}
	for _, usage := range []struct {
		name  string
		label string
	}{
		{name: "MixedExamples::Inspection::run", label: "«perform»"},
		{name: "MixedExamples::Inspection::mode", label: "«exhibit»"},
	} {
		nodes := names[usage.name]
		if len(nodes) == 0 {
			t.Errorf("mixed rendering has no reference usage %q; got %v", usage.name, mapNodeNames(names))
			continue
		}
		found := false
		for _, edge := range rendering.Edges {
			if edge.Kind == EdgeReference && edge.From == nodes[0].ID && edge.Label == usage.label {
				found = true
			}
			if edge.Kind == EdgeTyping && edge.From == nodes[0].ID {
				t.Errorf("%s usage has a duplicate typing edge: %+v", usage.name, edge)
			}
		}
		if !found {
			t.Errorf("%s usage has no %s reference edge", usage.name, usage.label)
		}
	}
	if len(names["ready"]) == 1 && len(names["MixedExamples::Ready"]) == 1 {
		from, to := names["ready"][0], names["MixedExamples::Ready"][0]
		found := false
		for _, edge := range rendering.Edges {
			if edge.Kind == EdgeTyping && edge.From == from.ID && edge.To == to.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("nested state usage ready has no typing edge to its drawn definition")
		}
	}
	if !strings.Contains(rendering.Mermaid(), "flowchart TD\n") {
		t.Errorf("mixed Mermaid does not default to TD:\n%s", rendering.Mermaid())
	}
	focusedRenderer, focusedIndex := loadFixture(t, "mixed.sysml")
	pump := lookup(t, focusedIndex, "MixedExamples::Plant::pump")
	focused, err := focusedRenderer.RenderExposed([]*symbols.Symbol{pump}, KindMixed, "#mixed")
	if err != nil {
		t.Fatalf("RenderExposed(pump): %v", err)
	}
	for _, edge := range focused.Edges {
		if edge.Kind == EdgeTyping {
			t.Errorf("mixed rendering invented a typing edge without a drawn type: %+v", edge)
		}
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, "rankdir") {
		t.Errorf("mixed DOT states a default rankdir:\n%s", dot)
	}
}

func TestCaseAndMixedLayoutRoutesReachDrawnElements(t *testing.T) {
	caseRenderer, caseIndex := loadFixture(t, "case.sysml")
	caseView := lookup(t, caseIndex, "CaseExamples::caseLayout")
	caseElement := lookup(t, caseIndex, "CaseExamples::Provide Transportation")
	caseActor := lookup(t, caseIndex, "CaseExamples::Provide Transportation::driver")
	caseInclude := lookup(t, caseIndex, "CaseExamples::Provide Transportation::Add Fuel")
	caseDrawn, err := caseRenderer.DrawnIn(caseView)
	if err != nil {
		t.Fatalf("DrawnIn(caseLayout): %v", err)
	}
	if !caseDrawn.Node(caseElement) || !caseDrawn.Node(caseActor) ||
		caseDrawn.Node(caseInclude) || !caseDrawn.Edge(caseInclude) {
		t.Errorf("case drawing membership: case=%t actor=%t include-node=%t include-edge=%t",
			caseDrawn.Node(caseElement), caseDrawn.Node(caseActor),
			caseDrawn.Node(caseInclude), caseDrawn.Edge(caseInclude))
	}
	caseRendering, err := caseRenderer.Render(caseView)
	if err != nil {
		t.Fatalf("Render(caseLayout): %v", err)
	}
	if got := findNode(t, caseRendering.Roots, "CaseExamples::'Provide Transportation'").Geometry; got == nil || got.X != 40 || got.Y != 60 {
		t.Errorf("case layout geometry = %+v, want (40, 60)", got)
	}
	if got := routedEdges(caseRendering); len(got) != 1 || got[0].Kind != EdgeInclude ||
		!reflect.DeepEqual(got[0].Route, []Point{{60, 20}, {100, 60}}) {
		t.Errorf("case routes = %+v, want the included-case route", got)
	}

	mixedRenderer, mixedIndex := loadFixture(t, "mixed.sysml")
	mixedView := lookup(t, mixedIndex, "MixedExamples::mixedLayout")
	mixedCase := lookup(t, mixedIndex, "MixedExamples::Inspection")
	mixedActor := lookup(t, mixedIndex, "MixedExamples::Inspection::operator")
	mixedTyping := lookup(t, mixedIndex, "MixedExamples::Plant::pump")
	mixedDrawn, err := mixedRenderer.DrawnIn(mixedView)
	if err != nil {
		t.Fatalf("DrawnIn(mixedLayout): %v", err)
	}
	if !mixedDrawn.Node(mixedCase) || !mixedDrawn.Node(mixedActor) || !mixedDrawn.Edge(mixedTyping) {
		t.Errorf("mixed drawing membership: case=%t actor=%t typing=%t",
			mixedDrawn.Node(mixedCase), mixedDrawn.Node(mixedActor), mixedDrawn.Edge(mixedTyping))
	}
	mixedRendering, err := mixedRenderer.Render(mixedView)
	if err != nil {
		t.Fatalf("Render(mixedLayout): %v", err)
	}
	if got := findNode(t, mixedRendering.Roots, "Inspection").Geometry; got == nil || got.X != 50 || got.Y != 30 {
		t.Errorf("mixed case geometry = %+v, want (50, 30)", got)
	}
	var typingRoute []Point
	for _, edge := range mixedRendering.Edges {
		if edge.Kind == EdgeTyping && edge.From == findNode(t, mixedRendering.Roots, "pump").ID {
			typingRoute = edge.Route
		}
	}
	if !reflect.DeepEqual(typingRoute, []Point{{100, 40}, {140, 80}}) {
		t.Errorf("mixed typing route = %+v, want (100, 40) to (140, 80)", typingRoute)
	}
}

func TestMixedDrawsCaseRolesAsNodesAndEdges(t *testing.T) {
	renderer, index := loadFixture(t, "case.sysml")
	caseElement := lookup(t, index, "CaseExamples::Provide Transportation")
	objective := (*symbols.Symbol)(nil)
	for _, member := range caseElement.Scope.AllMembers() {
		if semantics.IsObjectiveUsage(member) {
			objective = member
			break
		}
	}
	if objective == nil {
		t.Fatal("case fixture has no objective")
	}
	for _, sym := range []*symbols.Symbol{
		lookup(t, index, "CaseExamples::Provide Transportation::driver"),
		lookup(t, index, "CaseExamples::Provide Transportation::vehicle"),
		objective,
	} {
		node, edge := renderer.draws(KindMixed, sym)
		if !node || !edge {
			t.Errorf("mixed draws %s: node=%t edge=%t, want both", sym.Name, node, edge)
		}
	}
	var documentation *symbols.Symbol
	objective.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolDocumentation {
			documentation = member
			return false
		}
		return true
	})
	if documentation == nil {
		t.Fatal("objective has no documentation member")
	}
	if node, edge := renderer.draws(KindMixed, documentation); node || edge {
		t.Errorf("mixed draws objective documentation: node=%t edge=%t, want neither", node, edge)
	}
}

func TestCaseObjectiveDocumentationUsesRendererSourceText(t *testing.T) {
	renderer, index := loadFixture(t, "case.sysml")
	if renderer.model.SourceText() != nil {
		t.Fatal("NewRenderer set source text on the semantic model")
	}
	rendering, err := renderer.Render(lookup(t, index, "CaseExamples::caseDiagram"))
	if err != nil {
		t.Fatalf("Render(caseDiagram): %v", err)
	}
	found := false
	var visit func(*Node)
	visit = func(node *Node) {
		found = found || node.Kind == "objective" && strings.Contains(node.Detail, "Transport the vehicle")
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, root := range rendering.Roots {
		visit(root)
	}
	if !found {
		t.Fatal("objective documentation was not read from the renderer source lookup")
	}
	if renderer.model.SourceText() != nil {
		t.Fatal("rendering mutated semantic model source text")
	}
}

func TestCaseShapesAndEdgeLabelsAreKindScoped(t *testing.T) {
	for _, nodeKind := range []string{"use case", "actor", "subject", "objective"} {
		node := &Node{ID: "n0", Kind: nodeKind, Name: "Example"}
		for _, kind := range []Kind{KindTree, KindInterconnection} {
			labels := labelsOf([]*Node{node}, false, nil)
			shape := mermaidNodeShape(kind, node, labels, Options{})
			if strings.Contains(shape, "([") || strings.Contains(shape, "notch-rect") {
				t.Errorf("%s Mermaid node kind %q uses a case-only shape: %s", kind, nodeKind, shape)
			}
			rendering := &Rendering{Kind: kind, Roots: []*Node{node}}
			dot, err := rendering.DOT()
			if err != nil {
				t.Fatalf("%s DOT: %v", kind, err)
			}
			var nodeAttributes string
			for _, line := range strings.Split(dot, "\n") {
				if strings.Contains(line, `"n0" [`) {
					nodeAttributes = line
					break
				}
			}
			for _, shape := range []string{"shape=ellipse", "shape=note", "shape=box"} {
				if strings.Contains(nodeAttributes, shape) {
					t.Errorf("%s DOT node kind %q uses case-only %s", kind, nodeKind, shape)
				}
			}
		}
	}
	for _, kind := range []Kind{KindCase, KindMixed} {
		for _, tc := range []struct {
			nodeKind string
			shape    string
		}{{"use case", "shape=ellipse"}, {"actor", "shape=box"}, {"subject", "shape=box"}, {"objective", "shape=note"}} {
			node := &Node{ID: "n0", Kind: tc.nodeKind, Name: "Example"}
			labels := labelsOf([]*Node{node}, false, nil)
			if shape := mermaidNodeShape(kind, node, labels, Options{}); tc.nodeKind == "use case" && !strings.HasPrefix(shape, "([") ||
				tc.nodeKind == "objective" && !strings.Contains(shape, "notch-rect") {
				t.Errorf("%s Mermaid node kind %q has unexpected shape %q", kind, tc.nodeKind, shape)
			}
			rendering := &Rendering{Kind: kind, Roots: []*Node{node}}
			dot, err := rendering.DOT()
			if err != nil {
				t.Fatalf("%s DOT: %v", kind, err)
			}
			var nodeAttributes string
			for _, line := range strings.Split(dot, "\n") {
				if strings.Contains(line, `"n0" [`) {
					nodeAttributes = line
					break
				}
			}
			if !strings.Contains(nodeAttributes, tc.shape) {
				t.Errorf("%s DOT node kind %q attributes = %q, want %s", kind, tc.nodeKind, nodeAttributes, tc.shape)
			}
		}
	}
	if got := mermaidEdgeLabel(KindTree, Edge{Kind: EdgeComposition}); got != "" {
		t.Errorf("tree composition label = %q, want empty", got)
	}
	if got := mermaidEdgeLabel(KindCase, Edge{Kind: EdgeComposition}); got != "«composition»" {
		t.Errorf("case composition label = %q", got)
	}
	if got := mermaidEdgeLabel(KindTree, Edge{Kind: EdgeTransition, Label: "ready"}); got != "ready" {
		t.Errorf("tree explicit edge label = %q, want ready", got)
	}
}

func TestCaseRenderingIncludesInheritedVisibleRoles(t *testing.T) {
	renderer, index := loadFixture(t, "view-role-inheritance.sysml")
	base := lookup(t, index, "RoleInheritance::Base")
	driver := lookup(t, index, "RoleInheritance::Base::driver")
	subject := lookup(t, index, "RoleInheritance::Base::s")
	ownedObjectives, inheritedObjectives := renderer.model.ObjectivesOf(base)
	if len(inheritedObjectives) != 0 || len(ownedObjectives) != 1 {
		t.Fatalf("Base objectives = %v, %v; want one owned objective", ownedObjectives, inheritedObjectives)
	}
	objective := ownedObjectives[0]

	rendering, err := renderer.Render(lookup(t, index, "RoleInheritance::trip"))
	if err != nil {
		t.Fatalf("Render(trip): %v", err)
	}
	tripNode := findNode(t, rendering.Roots, "RoleInheritance::Trip")
	driverNode := findNodeByOrigin(t, rendering.Roots, driver.Origin())
	subjectNode := findNodeByOrigin(t, rendering.Roots, subject.Origin())
	objectiveNode := findNodeByOrigin(t, rendering.Roots, objective.Origin())
	if driverNode.Kind != "actor" || subjectNode.Kind != "subject" || objectiveNode.Kind != "objective" {
		t.Fatalf("inherited role nodes = %q, %q, %q; want actor, subject, objective",
			driverNode.Kind, subjectNode.Kind, objectiveNode.Kind)
	}
	if !strings.Contains(objectiveNode.Detail, "Arrive safely.") {
		t.Errorf("inherited objective detail = %q, want its documentation", objectiveNode.Detail)
	}
	for _, want := range []struct {
		from, to string
		kind     EdgeKind
	}{
		{driverNode.ID, tripNode.ID, EdgeAssociation},
		{tripNode.ID, subjectNode.ID, EdgeAssociation},
		{tripNode.ID, objectiveNode.ID, EdgeAnchor},
	} {
		if !hasRenderingEdge(rendering, want.kind, want.from, want.to) {
			t.Errorf("rendering has no %s edge from %s to %s", want.kind, want.from, want.to)
		}
	}

	redefined, err := renderer.Render(lookup(t, index, "RoleInheritance::trip2"))
	if err != nil {
		t.Fatalf("Render(trip2): %v", err)
	}
	var drivers []*Node
	var visit func([]*Node)
	visit = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Kind == "actor" && node.Name == "driver" {
				drivers = append(drivers, node)
			}
			visit(node.Children)
		}
	}
	visit(redefined.Roots)
	if len(drivers) != 1 {
		t.Errorf("Trip2 has %d driver nodes, want one: %+v", len(drivers), drivers)
	}

	plain, err := renderer.Render(lookup(t, index, "RoleInheritance::x"))
	if err != nil {
		t.Fatalf("Render(x): %v", err)
	}
	var plainRoleNodes []*Node
	var collectPlainRoles func([]*Node)
	collectPlainRoles = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Kind == "objective" || node.Kind == "subject" {
				plainRoleNodes = append(plainRoleNodes, node)
			}
			collectPlainRoles(node.Children)
		}
	}
	collectPlainRoles(plain.Roots)
	if len(plainRoleNodes) != 0 {
		t.Errorf("role-free X has subject or objective nodes: %+v", plainRoleNodes)
	}
}

func TestMixedKeepsDeferredMembersUnderStructuralOwnerInSourceOrder(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::mixed"))
	if err != nil {
		t.Fatalf("Render(mixed): %v", err)
	}
	vehicle := findNode(t, rendering.Roots, "MixedReview::P::Vehicle")
	var childNames []string
	for _, child := range vehicle.Children {
		childNames = append(childNames, child.Name)
	}
	if want := []string{"MixedReview::P::Vehicle::Drive", "Wheel", "MixedReview::P::Vehicle::Brake"}; !reflect.DeepEqual(childNames, want) {
		t.Errorf("Vehicle children = %v, want source order %v", childNames, want)
	}
}

func TestMixedKeepsCaseRolesUnderStructuralOwner(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::caseOwnerView"))
	if err != nil {
		t.Fatalf("Render(caseOwnerView): %v", err)
	}
	vehicleSym := lookup(t, index, "MixedReview::CaseOwners::Vehicle")
	driveSym := lookup(t, index, "MixedReview::CaseOwners::Vehicle::Drive")
	driverSym := lookup(t, index, "MixedReview::CaseOwners::Vehicle::Drive::driver")
	subjectSym := lookup(t, index, "MixedReview::CaseOwners::Vehicle::Drive::v")
	nestedSym := lookup(t, index, "MixedReview::CaseOwners::Vehicle::Drive::nested")
	vehicle := findNodeByOrigin(t, rendering.Roots, symbolOrigin(vehicleSym))
	drive := findDirectChildByOrigin(t, vehicle, symbolOrigin(driveSym))
	driver := findDirectChildByOrigin(t, vehicle, symbolOrigin(driverSym))
	subject := findDirectChildByOrigin(t, vehicle, symbolOrigin(subjectSym))
	nested := findDirectChildByOrigin(t, vehicle, symbolOrigin(nestedSym))
	for _, edge := range []struct {
		kind     EdgeKind
		from, to *Node
	}{
		{EdgeAssociation, driver, drive},
		{EdgeAssociation, drive, subject},
		{EdgeComposition, drive, nested},
	} {
		if !hasRenderingEdge(rendering, edge.kind, edge.from.ID, edge.to.ID) {
			t.Errorf("rendering has no %s edge from %q to %q: %+v",
				edge.kind, edge.from.Name, edge.to.Name, rendering.Edges)
		}
	}
	assertRenderingEdgeEndpoints(t, rendering)
}

func TestMixedKeepsIncludedTargetInItsPackage(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::mixed"))
	if err != nil {
		t.Fatalf("Render(mixed): %v", err)
	}
	packageNode := findNode(t, rendering.Roots, "MixedReview::P")
	from := findNode(t, rendering.Roots, "A")
	to := findNode(t, rendering.Roots, "B")
	var targetIsChild bool
	for _, child := range packageNode.Children {
		targetIsChild = targetIsChild || child == to
	}
	if !targetIsChild {
		t.Error("included case B is not a child of its package P node")
	}
	if !hasRenderingEdge(rendering, EdgeInclude, from.ID, to.ID) {
		t.Errorf("rendering has no include edge from A to B: %+v", rendering.Edges)
	}
}

func TestMixedResolvesTransitiveIncludes(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::transitiveIncludes"))
	if err != nil {
		t.Fatalf("Render(transitiveIncludes): %v", err)
	}
	a := findNodeByOrigin(t, rendering.Roots, symbolOrigin(lookup(t, index, "MixedReview::P::A")))
	b := findNodeByOrigin(t, rendering.Roots, symbolOrigin(lookup(t, index, "MixedReview::P::B")))
	c := findNodeByOrigin(t, rendering.Roots, symbolOrigin(lookup(t, index, "MixedReview::P::C")))
	if !hasRenderingEdge(rendering, EdgeInclude, a.ID, b.ID) {
		t.Errorf("rendering has no include edge A→B: %+v", rendering.Edges)
	}
	if !hasRenderingEdge(rendering, EdgeInclude, b.ID, c.ID) {
		t.Errorf("rendering has no include edge B→C: %+v", rendering.Edges)
	}
	assertRenderingEdgeEndpoints(t, rendering)
}

func TestMixedDispatchesFallbackMembersToTheirBuilders(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::behaviorView"))
	if err != nil {
		t.Fatalf("Render(behaviorView): %v", err)
	}
	requirement := findNode(t, rendering.Roots, "MixedReview::Requirement")
	action := findNode(t, requirement.Children, "MixedReview::Requirement::a")
	start := findNode(t, action.Children, "start")
	step := findNode(t, action.Children, "'step'")
	if !hasRenderingEdge(rendering, EdgeSuccession, start.ID, step.ID) {
		t.Errorf("rendering has no start-to-step succession edge: %+v", rendering.Edges)
	}
}

func TestMixedBuildsStructuresQueuedByDeferredMembers(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-review.sysml")
	rendering, err := renderer.Render(lookup(t, index, "MixedReview::nestedStructuresView"))
	if err != nil {
		t.Fatalf("Render(nestedStructuresView): %v", err)
	}
	vehicle := findNodeByOrigin(t, rendering.Roots, symbolOrigin(lookup(t, index, "MixedReview::NestedStructures::Vehicle")))
	requirement := findDirectChildByOrigin(t, vehicle, symbolOrigin(lookup(t, index, "MixedReview::NestedStructures::Vehicle::R")))
	findDirectChildByOrigin(t, requirement, symbolOrigin(lookup(t, index, "MixedReview::NestedStructures::Vehicle::R::Inner")))
	assertRenderingEdgeEndpoints(t, rendering)
}

func TestMixedConnectsStructuresAcrossDeferredMembers(t *testing.T) {
	renderer, index := loadFixture(t, "mixed-deferred-connector.sysml")
	rendering, err := renderer.Render(lookup(t, index, "DeferredConnector::connectorView"))
	if err != nil {
		t.Fatalf("Render(connectorView): %v", err)
	}
	vehicle := findNodeByOrigin(t, rendering.Roots, symbolOrigin(lookup(t, index, "DeferredConnector::Vehicle")))
	a := findDirectChildByOrigin(t, vehicle, symbolOrigin(lookup(t, index, "DeferredConnector::Vehicle::a")))
	requirement := findDirectChildByOrigin(t, vehicle, symbolOrigin(lookup(t, index, "DeferredConnector::Vehicle::r")))
	b := findDirectChildByOrigin(t, requirement, symbolOrigin(lookup(t, index, "DeferredConnector::Vehicle::r::b")))
	if !hasRenderingEdge(rendering, EdgeConnection, a.ID, b.ID) && !hasRenderingEdge(rendering, EdgeConnection, b.ID, a.ID) {
		t.Errorf("rendering has no connection edge between a and b: %+v", rendering.Edges)
	}
	assertRenderingEdgeEndpoints(t, rendering)
}

func findNodeByOrigin(t *testing.T, roots []*Node, origin symbols.Origin) *Node {
	t.Helper()
	var found *Node
	var visit func([]*Node)
	visit = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Origin == origin {
				found = node
				return
			}
			visit(node.Children)
			if found != nil {
				return
			}
		}
	}
	visit(roots)
	if found == nil {
		t.Fatalf("no node with source origin %+v", origin)
	}
	return found
}

func findDirectChildByOrigin(t *testing.T, parent *Node, origin symbols.Origin) *Node {
	t.Helper()
	for _, child := range parent.Children {
		if child.Origin == origin {
			return child
		}
	}
	t.Fatalf("no direct child of %q with source origin %+v", parent.Name, origin)
	return nil
}

func assertRenderingEdgeEndpoints(t *testing.T, rendering *Rendering) {
	t.Helper()
	ids := map[string]bool{}
	var visit func([]*Node)
	visit = func(nodes []*Node) {
		for _, node := range nodes {
			ids[node.ID] = true
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
	for _, edge := range rendering.Edges {
		if !ids[edge.From] {
			t.Errorf("%s edge has no source node %q", edge.Kind, edge.From)
		}
		if !ids[edge.To] {
			t.Errorf("%s edge has no target node %q", edge.Kind, edge.To)
		}
	}
}

func hasRenderingEdge(rendering *Rendering, kind EdgeKind, from, to string) bool {
	for _, edge := range rendering.Edges {
		if edge.Kind == kind && edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}

func TestPlantUMLRelationshipGuillemetsAreEscaped(t *testing.T) {
	for _, test := range []struct {
		file string
		view string
		want []string
	}{
		{file: "case.sysml", view: "CaseExamples::caseDiagram", want: []string{"subject", "include"}},
		{file: "mixed.sysml", view: "MixedExamples::mixedDiagram", want: []string{"perform", "exhibit"}},
	} {
		rendering := render(t, test.file, test.view)
		plantuml, err := rendering.PlantUML()
		if err != nil {
			t.Fatalf("PlantUML(%s): %v", test.view, err)
		}
		for _, label := range test.want {
			escaped := "<U+00AB>" + label + "<U+00BB>"
			if !strings.Contains(plantuml, escaped) {
				t.Errorf("%s PlantUML omits escaped label %s:\n%s", test.view, escaped, plantuml)
			}
		}
	}
}

func mapNodeNames(nodes map[string][]*Node) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
