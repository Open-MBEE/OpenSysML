package notation

import (
	"fmt"
	"sort"
)

// Category is what a production draws: a node, a compartment, a relationship or
// edge, a label or text on a drawn element, a whole view, or text the clause
// delegates to the textual notation of §8.2.2.
type Category string

const (
	CategoryNode        Category = "node"
	CategoryCompartment Category = "compartment"
	CategoryEdge        Category = "edge"
	CategoryLabel       Category = "label"
	CategoryView        Category = "view"
	CategoryTextual     Category = "textual"
)

// Categories lists the categories in the order the census reports them.
var Categories = []Category{CategoryNode, CategoryCompartment, CategoryEdge, CategoryLabel, CategoryView, CategoryTextual}

// Verdict is the census's finding for one production.
type Verdict string

const (
	// VerdictDrawn: a renderer draws the production as its figure shows it.
	VerdictDrawn Verdict = "drawn"
	// VerdictApproximate: drawn, but the medium or the writer departs from the figure.
	VerdictApproximate Verdict = "approximate"
	// VerdictUndrawn: nothing draws it.
	VerdictUndrawn Verdict = "undrawn"
	// VerdictNotApplicable: text reused from §8.2.2, or a list of alternatives
	// adjudicated through them.
	VerdictNotApplicable Verdict = "not-applicable"
)

// Verdicts lists the verdicts in the order the census reports them.
var Verdicts = []Verdict{VerdictDrawn, VerdictApproximate, VerdictUndrawn, VerdictNotApplicable}

// The rendering forms a drawn production may be held in.
const (
	FormCanvas   = "canvas"
	FormDOT      = "DOT"
	FormPlantUML = "PlantUML"
	FormMermaid  = "Mermaid"
	FormD2       = "D2"
	FormDrawio   = "draw.io"
	FormText     = "text"
	FormSVG      = "SVG"
)

// Forms lists every form in the order the census writes them.
var Forms = []string{FormCanvas, FormDOT, FormPlantUML, FormMermaid, FormD2, FormDrawio, FormText, FormSVG}

// Evidence names what draws a production and what pins the drawing.
type Evidence struct {
	// Implementation is `file:function` under internal/ir/view or the canvas.
	Implementation string   `json:"implementation,omitempty"`
	Forms          []string `json:"forms,omitempty"`
	// Test is `file_test.go:TestName` or the path of a golden file.
	Test string `json:"test,omitempty"`
}

// Adjudication is the census's verdict for one production with its reason and
// evidence.
type Adjudication struct {
	Category Category `json:"category"`
	Verdict  Verdict  `json:"verdict"`
	Reason   string   `json:"reason,omitempty"`
	Evidence
}

var (
	goldenForms  = []string{FormDOT, FormPlantUML, FormMermaid, FormText}
	controlForms = []string{FormDOT, FormMermaid, FormDrawio, FormCanvas}
	edgeForms    = []string{FormDOT, FormPlantUML, FormMermaid, FormD2, FormDrawio, FormCanvas}
	pinForms     = []string{FormDOT, FormCanvas}
	seqForms     = []string{FormPlantUML, FormMermaid, FormD2, FormText}
	dotForms     = []string{FormDOT}

	keywordBox   = Evidence{"internal/ir/view/label.go:keyword", goldenForms, "internal/ir/view/label_test.go:TestLabelLines"}
	compartments = Evidence{"internal/ir/view/compartments.go:compartmentName", goldenForms, "internal/ir/view/suppressed_compartments_test.go:TestGoldenSuppressedCompartments"}
	controlNode  = Evidence{"internal/ir/view/dot.go:dotSymbolAttributes", controlForms, "internal/ir/view/testdata/state-pseudostates.mermaid.golden"}
	tableEdge    = Evidence{"internal/ir/view/dot.go:dotEdgeAttributes", edgeForms, "internal/ir/view/notation_table_test.go:TestNotationTableMatchesTheWriters"}
	pins         = Evidence{"internal/ir/view/dot_ports.go:placePorts", pinForms, "internal/ir/view/action_pins_test.go:TestDOTActionFramePins"}
	boundaryPort = Evidence{"internal/ir/view/dot_ports.go:placePorts", pinForms, "internal/ir/view/testdata/interconnection-ports.dot.golden"}
	cameoFrame   = Evidence{"internal/ir/view/dot.go:openFrame", dotForms, "internal/ir/view/dot_cameo_test.go:TestGoldenDOTCameo"}
	sequence     = Evidence{"internal/ir/view/sequence.go:renderSequence", seqForms, "internal/ir/view/sequence_test.go:TestSequenceRenderingDrawsLifelinesAndMessages"}
	stateActions = Evidence{"internal/ir/view/behavior.go:stateActionCompartments", goldenForms, "internal/ir/view/dot_cameo_test.go:TestDOTStateBehaviourNames"}
	stateActNode = Evidence{"internal/ir/view/behavior.go:stateActionNodes", goldenForms, "internal/ir/view/testdata/state-forms.dot.golden"}
	stateTransit = Evidence{"internal/ir/view/behavior.go:stateMachineNode", goldenForms, "internal/ir/view/testdata/state.dot.golden"}
	stateActComp = Evidence{"internal/ir/view/compartments.go:compartmentName", goldenForms, "internal/ir/view/tree_state_compartments_test.go:TestGoldenTreeStateCompartments"}
	successions  = Evidence{"internal/ir/view/tree.go:successionText", goldenForms, "internal/ir/view/tree_state_compartments_test.go:TestGoldenTreeStateCompartments"}
	parallelMark = Evidence{"internal/ir/view/label.go:parallel", goldenForms, "internal/ir/view/testdata/state-pseudostates.mermaid.golden"}
	actionForms  = Evidence{"internal/ir/view/behavior_forms.go:statementNodeKind", goldenForms, "internal/ir/view/testdata/action-forms.dot.golden"}
	compoundBody = Evidence{"internal/ir/view/behavior_forms.go:compoundCompartments", goldenForms, "internal/ir/view/testdata/action-forms.dot.golden"}
	actionFlow   = Evidence{"internal/ir/view/behavior.go:flowNodes", goldenForms, "internal/ir/view/testdata/action.dot.golden"}
	elseBranch   = Evidence{"internal/ir/view/behavior.go:successionLabel", edgeForms, "internal/ir/view/testdata/action-forms.dot.golden"}
	terminateX   = Evidence{"internal/ir/view/dot.go:dotTerminateAttributes", controlForms, "internal/ir/view/dot_fit_test.go:TestDOTSymbolsInStatedBoxes"}
	pinArrows    = Evidence{"internal/ir/view/ports.go:pinArrow", []string{FormDOT, FormDrawio, FormCanvas}, "internal/ir/view/testdata/action.dot.golden"}
	transition   = Evidence{"internal/ir/view/behavior.go:transitionLabel", goldenForms, "internal/ir/view/testdata/state.dot.golden"}
	endLabels    = Evidence{"internal/ir/view/tree_edges.go:drawAssociation", edgeForms, "internal/ir/view/edge_ends_test.go:TestEndNamesAreSetAtTheirEnd"}
	multiplicity = Evidence{"internal/ir/view/tree_edges.go:usageMultiplicity", edgeForms, "internal/ir/view/tree_edges_test.go:TestTreeMultiplicityBoundsAreSpelledWithoutSourceText"}
	edgeLabel    = Evidence{"internal/ir/view/dot.go:dotEdgeAttributes", edgeForms, "internal/ir/view/notation_test.go:TestEdgeLabelsAreShownWhenAsked"}
	flowLabel    = Evidence{"internal/ir/view/interconnection.go:connectorLabel", edgeForms, "internal/ir/view/testdata/interconnection.dot.golden"}
	messageLabel = Evidence{"internal/ir/view/sequence.go:messageLabel", seqForms, "internal/ir/view/sequence_test.go:TestSequenceRenderingDrawsLifelinesAndMessages"}
	roleNode     = Evidence{"internal/ir/view/case.go:roleNode", goldenForms, "internal/ir/view/testdata/case.dot.golden"}
	naryForms    = []string{FormDOT, FormPlantUML, FormMermaid, FormD2, FormDrawio, FormCanvas, FormText, FormSVG}
	connDot      = Evidence{"internal/ir/view/interconnection.go:naryConnection", naryForms, "internal/ir/view/nary_test.go:TestNaryConnectionIsADotWithASegmentPerEnd"}
	defDot       = Evidence{"internal/ir/view/tree_edges.go:drawAssociationDot", naryForms, "internal/ir/view/nary_test.go:TestNaryDependencyAndConnectionDefAreDots"}
	depDot       = Evidence{"internal/ir/view/general.go:naryDependency", naryForms, "internal/ir/view/nary_test.go:TestNaryDependencyAndConnectionDefAreDots"}
	dotLabel     = Evidence{"internal/ir/view/nary.go:naryDot", naryForms, "internal/ir/view/testdata/nary-connection.dot.golden"}
	itemBox      = Evidence{"internal/ir/view/flow_on_connection.go:flowsOnConnections", naryForms, "internal/ir/view/nary_test.go:TestFlowsOnAConnectionAreItemBoxesOnItsLine"}
	itemArrow    = Evidence{"internal/ir/view/flow_on_connection.go:itemBox", naryForms, "internal/ir/view/testdata/nary-flow-on-connection.dot.golden"}
	objective    = Evidence{"internal/ir/view/case.go:objectiveNode", goldenForms, "internal/ir/view/testdata/case.dot.golden"}
	views        = map[string]Evidence{
		"general-view":          {"internal/ir/view/general.go:renderGeneral", goldenForms, "internal/ir/view/testdata/general-definition.dot.golden"},
		"interconnection-view":  {"internal/ir/view/interconnection.go:renderInterconnection", goldenForms, "internal/ir/view/testdata/interconnection.dot.golden"},
		"action-flow-view":      {"internal/ir/view/behavior.go:renderActions", goldenForms, "internal/ir/view/testdata/action.dot.golden"},
		"state-transition-view": {"internal/ir/view/behavior.go:renderStates", goldenForms, "internal/ir/view/testdata/state.dot.golden"},
		"sequence-view":         sequence,
	}
)

// Reasons shared by many rows.
const (
	reasonAlternatives = "a list of alternatives; adjudicated through the alternatives it names"
	reasonTextual      = "textual notation of §8.2.2 reused as the text of a drawn element"
	reasonNoCompart    = "no renderer writes this compartment; the members are drawn as nodes or edges, or not at all"
	reasonFiltered     = "annotations are filtered out of trees by tree.go containedKind; only DiagramLayout::Note and a case objective's documentation are drawn"
	reasonSequence     = "documented as undrawn in view-rendering-forms.md § Notation table: the sequence form draws its own arrow"
	reasonActionBox    = "behavior.go actionNodeKind falls through to a plain «action» box; the form's keyword and compartments are not written"
	reasonNoKeyword    = "the box is drawn but its keyword is not written"
	reasonCameoOnly    = "only the Cameo-style DOT frame draws it (dot.go openFrame); the default style draws no frame"
	reasonOnlyParallel = "the keyword and name are written, the optional «parallel» marker is not"
)

// table is the adjudication of every production by name, built once.
var table = buildTable()

func row(category Category, verdict Verdict, reason string, ev Evidence) Adjudication {
	return Adjudication{Category: category, Verdict: verdict, Reason: reason, Evidence: ev}
}

func hub(category Category) Adjudication {
	return row(category, VerdictNotApplicable, reasonAlternatives, Evidence{})
}

func textual() Adjudication {
	return row(CategoryTextual, VerdictNotApplicable, reasonTextual, Evidence{})
}

func undrawn(category Category, reason string) Adjudication {
	return row(category, VerdictUndrawn, reason, Evidence{})
}

func drawn(category Category, ev Evidence) Adjudication { return row(category, VerdictDrawn, "", ev) }

func approximate(category Category, reason string, ev Evidence) Adjudication {
	return row(category, VerdictApproximate, reason, ev)
}

func buildTable() map[string]Adjudication {
	t := map[string]Adjudication{}
	set := func(adj Adjudication, names ...string) {
		for _, n := range names {
			if _, dup := t[n]; dup {
				panic("notation: " + n + " adjudicated twice")
			}
			t[n] = adj
		}
	}
	// Lists of alternatives, extended across the clauses with `=|`.
	set(hub(CategoryNode), "element", "general-element", "element-node", "general-node", "namespace-node", "type-node",
		"definition-node", "usage-node", "dependencies-and-annotations-element", "dependencies-element",
		"annotation-node", "annotated-element", "interconnection-element", "action-flow-element", "action-flow-node",
		"subject-actors-stakeholders-node", "connection-end", "flow-end-node", "msg-end-node", "performer-node",
		"parameter", "distinguished-parameter", "eventer", "exhibitor", "satisfier", "requirer", "framer", "assumer",
		"assertor", "allocation-node", "state-transition-element", "state-transition-node", "state-source",
		"timeslice-or-snapshot-node", "sq-graphical-element", "sq-graphical-node", "require-constraint-element")
	set(hub(CategoryCompartment), "compartment")
	set(hub(CategoryEdge), "general-relationship", "type-relationship", "usage-edge", "action-flow-relationship",
		"connection-relationship", "expose-relationship", "sq-graphical-relationship")
	set(hub(CategoryView), "frameless-view")

	// Text of §8.2.2 written inside a drawn element.
	set(textual(), "el-prefix", "usage-cp", "text-block", "language-string", "definition-name-with-alias",
		"usage-name-with-alias", "c-adornment", "a-property", "a-direction", "a-subsetting", "a-redefinition",
		"state-subaction-body", "state-subaction-body-textual", "state-subaction-declaration", "action-body",
		"action-body-textual", "condition-expression", "iteration-expression", "send-action-expression",
		"accept-action-expression", "metadata-feature-decl", "metadata-feature-name", "metadata-feature-name-value-list",
		"expression-text", "view-frame-info-contents", "sq-port-label")

	// The contents and elements of every compartment are §8.2.2 text.
	set(textual(),
		"actions-compartment-contents", "actions-compartment-element", "actors-compartment-contents",
		"actors-compartment-element", "allocations-compartment-contents", "allocations-compartment-element",
		"analyses-compartment-contents", "analyses-compartment-element", "assert-constraints-compartment-contents",
		"assert-constraints-compartment-element", "assume-constraints-compartment-contents",
		"attributes-compartment-contents", "attributes-compartment-element", "calcs-compartment-contents",
		"calcs-compartment-element", "connections-compartment-contents", "connections-compartment-element",
		"constraints-compartment-contents", "constraints-usage-compartment-element",
		"directed-features-compartment-contents", "directed-features-compartment-element", "ends-compartment-contents",
		"ends-compartment-element", "enums-compartment-element", "exhibit-states-compartment-contents",
		"exhibit-states-compartment-element-compartment", "exposes-compartment-contents", "exposes-compartment-element",
		"features-compartment-contents", "features-compartment-element", "filters-compartment-contents",
		"filters-compartment-element", "flows-compartment-contents", "flows-compartment-element",
		"frames-compartment-contents", "frames-compartment-element", "include-use-cases-compartment-contents",
		"include-use-cases-compartment-element", "includes-compartment-contents", "includes-compartment-element",
		"individuals-compartment-contents", "individuals-compartment-element", "interfaces-compartment-contents",
		"interfaces-compartment-element", "items-compartment-contents", "items-compartment-element",
		"members-compartment-contents", "members-compartment-element", "objective-compartment-contents",
		"objective-compartment-element", "occurrences-compartment-contents", "occurrences-compartment-element",
		"packages-compartment-contents", "packages-compartment-element", "parameters-compartment-contents",
		"parameters-compartment-element", "parts-compartment-contents", "parts-compartment-element",
		"perform-actions-compartment-contents", "perform-actions-compartment-element", "performed-by-compartment-contents",
		"ports-compartment-contents", "ports-compartment-element", "relationships-compartment-contents",
		"relationships-compartment-element", "rendering-compartment-contents", "require-constraints-compartment-contents",
		"requirements-compartment-contents", "requirements-compartment-element", "result-compartment-contents",
		"satisfies-compartment-contents", "satisfy-requirements-compartment-contents", "snapshots-compartment-contents",
		"snapshots-compartment-element", "stakeholders-compartment-contents", "stakeholders-compartment-element",
		"state-actions-compartment-contents", "state-actions-compartment-element", "states-compartment-contents",
		"states-compartment-element", "subject-compartment-contents", "subject-compartment-element",
		"successions-compartment-contents", "timeslices-compartment-contents", "timeslices-compartment-element",
		"use-cases-compartment-contents", "use-cases-compartment-element", "variants-compartment-contents",
		"verification-methods-compartment-contents", "verification-methods-compartment-element",
		"verifications-compartment-contents", "verifications-compartment-element", "verifies-compartment-contents",
		"verifies-compartment-element", "viewpoints-compartment-contents", "viewpoints-compartment-element",
		"views-compartment-contents", "views-compartment-element")

	// Whole views.
	for name, ev := range views {
		set(drawn(CategoryView, ev), name)
	}
	set(approximate(CategoryView, reasonCameoOnly, cameoFrame), "root-view", "framed-view")
	set(approximate(CategoryCompartment, reasonCameoOnly+"; its header reads the view's name without the «view» keyword", cameoFrame), "view-frame-name-compartment")
	set(undrawn(CategoryCompartment, "no form draws the frame's info compartments"), "view-frame-info-compartment-tr", "view-frame-info-compartment-bl", "view-frame-info-compartment-br")

	// §8.2.3.2: the general view's own productions.
	set(undrawn(CategoryCompartment, "a nested general view inside a node's compartment is not drawn; nested nodes are drawn as a tree"), "general-compartment")
	set(undrawn(CategoryLabel, "hidden compartment rows are dropped without the '…' mark (compartments.go hidden rows)"), "ellipsis-at-lower-left-corner")
	set(undrawn(CategoryNode, "the greyed element inside a textual compartment is not drawn; compartment rows are plain text"), "element-inside-textual-compartment")
	set(drawn(CategoryLabel, edgeLabel), "rel-name", "relationship-name")

	// §8.2.3.3 dependencies.
	set(drawn(CategoryNode, depDot), "n-ary-association-dot")
	set(drawn(CategoryEdge, depDot), "n-ary-dependency", "n-ary-dependency-client-or-supplier-link", "n-ary-dependency-supplier-link")

	// §8.2.3.4 annotations.
	set(undrawn(CategoryNode, reasonFiltered), "comment-node", "comment-without-keyword", "comment-with-keyword", "documentation-node", "textual-representation-node")
	set(undrawn(CategoryCompartment, reasonFiltered), "documentation-compartment")
	set(approximate(CategoryEdge, "only the anchor of a DiagramLayout::Note or a case objective is drawn (EdgeAnchor); comments are filtered out", Evidence{"internal/ir/view/dot.go:dotEdgeAttributes", edgeForms, "internal/ir/view/dot_cameo_test.go:TestNotesAcrossForms"}), "annotation-link")

	// §8.2.3.5 packages.
	packageBox := Evidence{"internal/ir/view/dot.go:dotClusterAttributes", goldenForms, "internal/ir/view/testdata/general-package.dot.golden"}
	set(approximate(CategoryNode, "a «package» box without the tab; PlantUML's `package` folder is the nearest form", packageBox), "package-node", "package-with-name-inside")
	set(undrawn(CategoryNode, "the name-in-tab form is not drawn; packages are keyword boxes"), "package-with-name-in-tab")
	set(undrawn(CategoryNode, "an imported package is drawn as an ordinary package node at the end of an import edge"), "imported-package-with-name-inside", "imported-package-with-name-in-tab")
	set(drawn(CategoryCompartment, packageBox), "package-compartment")
	set(undrawn(CategoryCompartment, reasonNoCompart), "packages-compartment", "members-compartment", "relationships-compartment")

	// §8.2.3.6 definitions and usages.
	set(drawn(CategoryCompartment, compartments), "compartment-stack")
	set(undrawn(CategoryCompartment, reasonNoCompart), "features-compartment", "variants-compartment", "variant-elementusages-compartment")
	set(undrawn(CategoryNode, "a user-defined keyword (metadata-based definition) is not written; label.go keyword writes the element's kind"), "extended-def", "extended-def-name-compartment", "extended-usage", "extended-usage-name-compartment")

	set(row(CategoryNode, VerdictNotApplicable, "the textual grammar declares no event occurrence definition (`event` is a usage prefix), so nothing can be declared for the figure", Evidence{}), "event-occurrence-def")
	set(row(CategoryCompartment, VerdictNotApplicable, "the textual grammar declares no event occurrence definition (`event` is a usage prefix), so nothing can be declared for the figure", Evidence{}), "event-occurrence-def-name-compartment")

	// Keyword boxes: «kind def» and «kind» name compartments, the nodes they head.
	kinds := []string{"attribute", "enumeration", "occurrence", "item", "part", "port", "connection",
		"interface", "allocation", "flow", "action", "state", "calc", "constraint", "requirement", "concern", "analysis",
		"verification", "use-case", "view", "viewpoint", "metadata"}
	for _, k := range kinds {
		set(drawn(CategoryNode, keywordBox), k+"-def")
		if k != "state" {
			set(drawn(CategoryCompartment, keywordBox), k+"-def-name-compartment")
		}
	}
	set(drawn(CategoryNode, keywordBox), "attribute", "enumeration", "occurrence", "occurrence-ref", "timeslice", "snapshot",
		"item", "item-ref", "part", "part-ref", "port-usage", "action", "action-ref", "perform-action-usage", "state",
		"state-ref", "exhibit-state-usage", "calc", "constraint", "constraint-ref", "assert-constraint-node",
		"assume-constraint-node", "require-constraint-node", "requirement", "requirement-ref", "concern", "analysis",
		"verification", "use-case", "include-use-case-usage", "viewpoint", "view", "subject", "stakeholder")
	set(drawn(CategoryCompartment, keywordBox), "attribute-name-compartment", "occurrence-name-compartment",
		"timeslice-name-compartment", "item-name-compartment", "part-name-compartment", "port-name-compartment",
		"action-name-compartment", "perform-action-name-compartment", "exhibit-state-name-compartment",
		"calc-name-compartment", "constraint-name-compartment", "enumeration-name-compartment", "assert-constraint-name-compartment",
		"assume-constraint-name-compartment", "require-constraint-name-compartment", "requirement-name-compartment",
		"concern-name-compartment", "analysis-name-compartment", "verification-name-compartment",
		"use-case-name-compartment", "include-use-case-name-compartment", "viewpoint-name-compartment", "view-name-compartment")
	set(approximate(CategoryNode, "an event occurrence is drawn as a plain «occurrence» box", keywordBox), "event-occurrence")
	set(approximate(CategoryCompartment, "written as «occurrence»; the `event` prefix is not part of the keyword", keywordBox), "event-occurrence-name-compartment")
	set(drawn(CategoryCompartment, parallelMark), "state-def-name-compartment", "state-name-compartment")
	set(approximate(CategoryNode, "written as «satisfy», the figure's keyword is «satisfy requirement»", keywordBox), "satisfy-requirement-usage")
	set(approximate(CategoryCompartment, "written as «satisfy», the figure's keyword is «satisfy requirement»", keywordBox), "satisfy-requirement-name-compartment")
	set(approximate(CategoryNode, "written as «frame», the figure's keyword is «frame concern»", keywordBox), "frame-concern-node")
	set(approximate(CategoryCompartment, "written as «frame», the figure's keyword is «frame concern»", keywordBox), "frame-concern-name-compartment")
	set(undrawn(CategoryNode, "a verify requirement usage lives in a verification case's objective, which is drawn as a note (case.go objectiveNode)"), "verify-requirement-node", "verify-requirement-name-compartment")
	set(drawn(CategoryNode, keywordBox), "connection", "interface", "allocation", "flow-node")
	set(drawn(CategoryCompartment, keywordBox), "connection-name-compartment", "interface-name-compartment", "allocation-name-compartment", "flow-name-compartment")
	set(approximate(CategoryNode, "a box headed «actor» in DOT, PlantUML, Mermaid and text; only draw.io draws the stick figure (drawio.go plainStyle umlActor)",
		Evidence{"internal/ir/view/drawio.go:plainStyle", []string{FormDOT, FormPlantUML, FormMermaid, FormDrawio, FormText}, "internal/ir/view/testdata/case.dot.golden"}), "actor")
	set(drawn(CategoryLabel, roleNode), "subject-name", "actor-name", "stakeholder-name")

	// Compartments the renderers head as the figure does.
	set(drawn(CategoryCompartment, compartments), "attributes-compartment", "occurrences-compartment", "timeslices-compartment",
		"snapshots-compartment", "items-compartment", "parts-compartment", "ports-compartment", "connections-compartment",
		"interfaces-compartment", "allocations-compartment", "actions-compartment", "perform-actions-compartment",
		"states-compartment", "exhibit-states-compartment", "constraints-compartment", "assert-constraints-compartment",
		"requirements-compartment", "require-constraints-compartment", "assume-constraints-compartment",
		"actors-compartment", "stakeholders-compartment", "frames-compartment", "concerns-compartment",
		"analyses-compartment", "verifications-compartment", "use-cases-compartment", "include-use-cases-compartment",
		"views-compartment", "viewpoints-compartment")
	set(drawn(CategoryCompartment, Evidence{"internal/ir/view/label.go:parameterLines", goldenForms, "internal/ir/view/testdata/calc-parameters.dot.golden"}), "parameters-compartment")
	set(approximate(CategoryCompartment, "headed `enumerations`; the figure heads it `enums`", compartments), "enums-compartment")
	set(approximate(CategoryCompartment, "headed `calculations`; the figure heads it `calcs`", compartments), "calcs-compartment")
	set(approximate(CategoryCompartment, "headed `renderings`; the figure heads it `rendering`", compartments), "rendering-compartment")
	set(approximate(CategoryCompartment, "headed `subjects`; the figure heads it `subject`", compartments), "subject-compartment")
	set(approximate(CategoryCompartment, "headed `satisfys` by compartments.go compartmentName's plural; the figure heads it `satisfies`", compartments), "satisfy-requirements-compartment", "satisfies-compartment")
	set(approximate(CategoryCompartment, "the objective is drawn as a note anchored to the case (case.go objectiveNode), not as a compartment", objective), "objective-compartment")
	set(undrawn(CategoryCompartment, reasonNoCompart), "individuals-compartment", "sequence-compartment", "directed-features-compartment",
		"interconnection-compartment", "ends-compartment", "flows-compartment", "performed-by-compartment",
		"result-compartment",
		"verifies-compartment", "verification-methods-compartment", "includes-compartment", "exposes-compartment", "filters-compartment")

	// §8.2.3.9 sequences.
	set(drawn(CategoryNode, sequence), "sq-head-node", "sq-part", "sq-l-node", "lifeline")
	set(undrawn(CategoryNode, "a port is not given a lifeline; sequence.go lifelinesOf collects parts and occurrences"), "sq-port")
	set(undrawn(CategoryNode, "a feature-chain proxy lifeline is not drawn"), "sq-proxy")
	set(undrawn(CategoryLabel, "labels the undrawn sq-proxy"), "proxy-label")
	set(undrawn(CategoryEdge, reasonSequence), "message", "sq-succession")
	set(undrawn(CategoryLabel, "labels the undrawn sq-succession"), "succession-label")
	set(drawn(CategoryLabel, messageLabel), "message-label")

	// §8.2.3.12 ports and parameters: the boundary squares and their labels.
	set(approximate(CategoryNode, "the port square is placed on the owner's boundary but the direction arrow (pdh/pdv) inside it is not drawn", boundaryPort), "port-l", "port-r", "port-t", "port-b")
	set(drawn(CategoryNode, pinArrows), "param-l", "param-r", "param-t", "param-b")
	set(undrawn(CategoryLabel, "no form draws the direction arrow inside a port or parameter square"), "pdh", "pdv")
	set(drawn(CategoryLabel, boundaryPort), "port-label")
	set(drawn(CategoryLabel, pins), "param-label")
	set(approximate(CategoryNode, "a conjugated port is written as `~Type` text (view.go typingTexts), not as the proxy glyph", Evidence{"internal/ir/view/view.go:typingTexts", pinForms, "internal/ir/view/testdata/interconnection-ports.dot.golden"}), "proxy-v", "proxy-h")

	// §8.2.3.13–.16 connections, interfaces, allocations, flows.
	set(drawn(CategoryLabel, endLabels), "rolename", "cdef-label")
	set(drawn(CategoryLabel, multiplicity), "multiplicity")
	set(drawn(CategoryLabel, edgeLabel), "connection-label")
	set(approximate(CategoryEdge, "drawn as a connection edge between the ports; the «interface» keyword is not written", tableEdge), "interface-connection")
	set(drawn(CategoryLabel, edgeLabel), "interface-label")
	set(drawn(CategoryNode, defDot), "n-ary-def-connection-dot")
	set(drawn(CategoryNode, connDot), "n-ary-connection-dot")
	set(drawn(CategoryEdge, defDot), "n-ary-connection-def", "n-ary-def-segment")
	set(drawn(CategoryEdge, connDot), "n-ary-connection", "n-ary-segment")
	set(drawn(CategoryLabel, dotLabel), "cdot-label")
	set(undrawn(CategoryView, "a connection's own interconnection view nested in its node is not drawn"), "connection-definition-elaboration", "connection-usage-elaboration")
	set(drawn(CategoryLabel, flowLabel), "flow-label", "succession-flow-label")
	set(drawn(CategoryNode, itemBox), "flow-on-connection")
	set(drawn(CategoryNode, itemArrow), "flow-node-r", "flow-node-l", "sflow-node-r", "sflow-node-l", "message-node-r", "message-node-l")

	// §8.2.3.17 actions.
	set(drawn(CategoryNode, controlNode), "start-node", "done-node", "fork-node", "join-node", "decision-node", "merge-node")
	set(approximate(CategoryNode, "the figure's X in a circle in DOT, Mermaid (cross-circ), draw.io (umlDestroy) and the canvas; PlantUML has no terminate pseudostate and D2 no crossed circle, both draw a plain end", terminateX), "terminate-node")
	set(approximate(CategoryNode, "a rounded box with the keyword compartment in every form; the send and accept corner glyphs are drawn in DOT, draw.io and the canvas, written as ▷ and ◁ in PlantUML and Mermaid, and not drawn in D2", actionForms), "send-action-node", "accept-action-node")
	set(drawn(CategoryNode, actionForms), "while-loop-action-node", "for-loop-action-node", "if-else-action-node", "assign-action-node")
	set(drawn(CategoryCompartment, actionForms), "send-action-name-compartment", "accept-action-name-compartment",
		"while-loop-action-name-compartment", "for-loop-action-name-compartment", "ifelse-action-name-compartment", "assign-action-name-compartment")
	set(drawn(CategoryCompartment, compoundBody), "while-condition", "until-condition", "iteration", "loop-body", "if-condition", "then-body", "else-body")
	set(drawn(CategoryCompartment, actionFlow), "action-flow-compartment")
	set(undrawn(CategoryCompartment, "swimlanes are not drawn"), "perform-actions-swimlanes", "swimlane")
	set(drawn(CategoryEdge, elseBranch), "else-branch")
	set(drawn(CategoryLabel, transition), "guard-expression", "transition-label", "trigger-expression")

	// §8.2.3.18 states.
	set(drawn(CategoryCompartment, stateActions), "state-entry-action", "state-do-action", "state-exit-action")
	set(drawn(CategoryNode, stateActNode), "entry-action", "exit-action", "do-action")
	set(drawn(CategoryCompartment, stateActNode), "entry-action-name-comp", "exit-action-name-comp", "do-action-name-comp")
	set(drawn(CategoryCompartment, stateTransit), "state-transition-compartment")
	set(drawn(CategoryCompartment, stateActComp), "state-actions-compartment")
	set(drawn(CategoryCompartment, successions), "successions-compartment")

	// §8.2.3.21 requirements and cases.
	set(drawn(CategoryEdge, tableEdge), "distinguished-parameter-link", "frame-relationship")
	set(undrawn(CategoryEdge, "the line from a concern to its stakeholder is not drawn"), "concern-stakeholder-link")

	// §8.2.3.27 metadata.
	set(undrawn(CategoryNode, "a metadata usage annotating an element is not drawn as a «metadata» note"), "metadata-feature-annotation-node")
	return t
}

// Adjudicate returns the census's verdict for a production. A production
// without one is an error: a changed specification must not pass unnoticed.
func Adjudicate(name string, edges *NotationTable) (Adjudication, error) {
	if adj, ok := table[name]; ok {
		return adj, nil
	}
	if adj, ok := edges.adjudicate(name); ok {
		return adj, nil
	}
	return Adjudication{}, fmt.Errorf("no adjudication for production %s", name)
}

// AdjudicatedNames lists the productions the static table adjudicates, sorted.
func AdjudicatedNames() []string {
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
