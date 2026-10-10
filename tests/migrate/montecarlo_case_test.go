package migrate_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// wantBlock asserts the notation holds the lines consecutively, in order, whatever indents them.
func wantBlock(t *testing.T, notation []byte, lines ...string) {
	t.Helper()
	written := strings.Split(string(notation), "\n")
	for i := range written {
		written[i] = strings.TrimSpace(written[i])
	}
	joined := "\n" + strings.Join(written, "\n") + "\n"
	if !strings.Contains(joined, "\n"+strings.Join(lines, "\n")+"\n") {
		t.Errorf("notation lacks the block:\n%s\n\nin:\n%s", strings.Join(lines, "\n"), notation)
	}
}

// montecarlo_case.xmi: a block inheriting the customization module's MonteCarloAnalysis
// binds settleTime to Mean and Deviation, mismatched values to N/OutOfSpec, and an unknown
// statistic; a block specializing it adds N and rebinds OutOfSpec, inheriting the rest.
func TestMonteCarloAnalysisIsAnAnalysisCase(t *testing.T) {
	r := migrateFixtureFile(t, "montecarlo_case")
	wantLine(t, r.Notation, "part def 'Settling Analysis' :> Sensor {")
	wantLine(t, r.Notation, "analysis def 'Settling Analysis Monte Carlo' :> Simulation::MonteCarlo {")
	wantLine(t, r.Notation, "subject analysed : 'Settling Analysis';")
	wantLine(t, r.Notation, "perform action run ::> analysed.settle;")
	wantLine(t, r.Notation, "attribute :>> observed : ScalarValues::Real = analysed.settleTime;")
	wantLine(t, r.Notation, "return Mean : ScalarValues::Real[1] = mean;")
	wantLine(t, r.Notation, "out Deviation : ScalarValues::Real[0..1] = deviation;")
	wantLine(t, r.Notation, "out OutOfSpec : ScalarValues::Integer[1] = outOfSpec;")
	wantBlock(t, r.Notation, "return Mean : ScalarValues::Real[1] = mean;",
		"out Deviation : ScalarValues::Real[0..1] = deviation;",
		"out OutOfSpec : ScalarValues::Integer[1] = outOfSpec;",
		"}")
	wantNoLine(t, r.Notation, "Median :")
	wantNoLine(t, r.Notation, "= median")
	wantLine(t, r.Notation, "analysis 'Monte Carlo' : 'Settling Analysis Monte Carlo' {")
	wantLine(t, r.Notation, "subject :>> analysed = 'settling of 5 runs';")
	wantLine(t, r.Notation, "out :>> deviation = 0.6;")
	wantBlock(t, r.Notation, "analysis def 'Retried Settling Monte Carlo' :> Simulation::MonteCarlo {",
		"subject analysed : 'Retried Settling';",
		"perform action run ::> analysed.settle;",
		"attribute :>> observed : ScalarValues::Real = analysed.settleTime;",
		"return Mean : ScalarValues::Real[1] = mean;",
		"out N : ScalarValues::Integer[1] = runs;",
		"out OutOfSpec : ScalarValues::Integer[1] = outOfSpec;",
		"out Deviation : ScalarValues::Real[0..1] = deviation;",
		"}")

	wantNote(t, r, "_analysis", migrate.Approximated, "generalization of the simulation tool's MonteCarloAnalysis is written as the analysis def 'Settling Analysis Monte Carlo' :> Simulation::MonteCarlo beside the part def, which is its subject")
	wantNote(t, r, "_bindMean", migrate.Approximated, "written in the analysis def 'Settling Analysis Monte Carlo' as the observed value, of which Mean is returned")
	wantNote(t, r, "_bindDeviation", migrate.Approximated, "as the returned Deviation, bound to deviation")
	wantNote(t, r, "_bindOutOfSpec", migrate.Approximated, "as the returned OutOfSpec, bound to outOfSpec")
	wantNote(t, r, "_bindN", migrate.Unmapped, "the connector binds the simulation tool's MonteCarloAnalysis::N to label, which is of String, and the statistic is of a number")
	wantNote(t, r, "_bindMedian", migrate.Unmapped, "the connector binds the simulation tool's MonteCarloAnalysis::Median, a statistic Simulation::MonteCarlo has no counterpart for")
	wantNote(t, r, "_bindStranger", migrate.Unmapped, "a statistic of an analysis its owner does not inherit")
	wantNote(t, r, "_bindGain", migrate.Mapped, "")
	wantNote(t, r, "_bindRetriedN", migrate.Approximated, "written in the analysis def 'Retried Settling Monte Carlo' as the returned N, bound to runs")
	wantNote(t, r, "_bindRetriedOutOfSpec", migrate.Approximated, "written in the analysis def 'Retried Settling Monte Carlo' as the returned OutOfSpec, bound to outOfSpec")
	wantNote(t, r, "_sMean", migrate.Mapped, "")
	wantNote(t, r, "_sMedian", migrate.Unmapped, "a statistic Simulation::MonteCarlo has no counterpart for")

	if r.Results == nil || len(r.Results.Configurations) != 2 {
		t.Fatalf("results = %+v, want two configurations", r.Results)
	}
	configured := map[string]simresults.ConfigurationResults{}
	for _, cfg := range r.Results.Configurations {
		if cfg.Analysis != "settleTime" {
			t.Errorf("%s analyses %q, want settleTime", cfg.AnalysisCase, cfg.Analysis)
		}
		configured[cfg.AnalysisCase] = cfg
	}
	cfg, ok := configured["'Settling Analysis Monte Carlo'"]
	if !ok {
		t.Fatalf("configurations = %+v, want one by 'Settling Analysis Monte Carlo'", r.Results.Configurations)
	}
	if want := []string{simresults.StatisticMean, simresults.StatisticDeviation, simresults.StatisticOutOfSpec}; !reflect.DeepEqual(cfg.Statistics, want) {
		t.Errorf("statistics = %v, want %v", cfg.Statistics, want)
	}
	want := &simresults.Statistics{Observable: "settleTime", Runs: 5, Mean: 3.1, Deviation: simresults.Real(0.6), OutOfSpec: simresults.Count(0)}
	if len(cfg.Snapshots) != 1 || !reflect.DeepEqual(cfg.Snapshots[0].Statistics, want) {
		t.Errorf("snapshots = %+v, want one holding %+v", cfg.Snapshots, want)
	}
	retried, ok := configured["'Retried Settling Monte Carlo'"]
	if !ok {
		t.Fatalf("configurations = %+v, want one by 'Retried Settling Monte Carlo'", r.Results.Configurations)
	}
	if want := []string{simresults.StatisticMean, simresults.StatisticRuns, simresults.StatisticOutOfSpec, simresults.StatisticDeviation}; !reflect.DeepEqual(retried.Statistics, want) {
		t.Errorf("statistics = %v, want the inherited ones too: %v", retried.Statistics, want)
	}
	want = &simresults.Statistics{Observable: "settleTime", Runs: 3, Mean: 2.9, Deviation: simresults.Real(0.4), OutOfSpec: simresults.Count(1)}
	if len(retried.Snapshots) != 1 || !reflect.DeepEqual(retried.Snapshots[0].Statistics, want) {
		t.Errorf("snapshots = %+v, want one holding %+v", retried.Snapshots, want)
	}
	wantClean(t, "montecarlo_case.sysml", r)
}

// montecarlo_homonym.xmi: a user's own block named MonteCarloAnalysis, generalized and
// bound to, is an ordinary block without the customization module's provenance.
func TestUserBlockNamedMonteCarloAnalysisIsNoPattern(t *testing.T) {
	r := migrateFixtureFile(t, "montecarlo_homonym")
	wantLine(t, r.Notation, "part def MonteCarloAnalysis {")
	wantLine(t, r.Notation, "part def 'Gauge Analysis' :> Gauge, MonteCarloAnalysis {")
	wantLine(t, r.Notation, "bind reading = Mean;")
	wantLine(t, r.Notation, "attribute :>> Mean = 2.5;")
	wantNoLine(t, r.Notation, "analysis def")
	wantNoLine(t, r.Notation, "Monte Carlo")
	wantNote(t, r, "_analysis", migrate.Mapped, "")
	wantNote(t, r, "_bind", migrate.Mapped, "")
	wantNote(t, r, "_runMean", migrate.Mapped, "")
	if n := r.Report.Count()[migrate.Unmapped]; n != 0 {
		t.Errorf("unmapped = %d, want none", n)
	}
	cfg := r.Results.Configurations[0]
	if cfg.Analysis != "" || cfg.AnalysisCase != "" || cfg.Statistics != nil || cfg.Snapshots[0].Statistics != nil {
		t.Errorf("configuration = %+v, want no analysis", cfg)
	}
	wantClean(t, "montecarlo_homonym.sysml", r)
}

const monteCarloGeneral = `<general href="MD_customization_for_SysML.mdzip#_mc"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis" referentType="Class"/></xmi:Extension></general>`

// monteCarloRole is a connector end's role on the pattern's statistic stat.
func monteCarloRole(stat string) string {
	return `<role href="MD_customization_for_SysML.mdzip#_mc` + stat + `"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis::` + stat + `" referentType="Property"/></xmi:Extension></role>`
}

// monteCarloBlock is a block Timer Analysis of the values t and u, generalizing
// general and owning connectors, with the applications of its block stereotypes.
func monteCarloBlock(general, connectors string) (members, applications string) {
	return `
    <packagedElement xmi:type="uml:Class" xmi:id="_timer" name="Timer">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="t">` + realHref + `</ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_u" name="u">` + realHref + `</ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_analysis" name="Timer Analysis">
      <generalization xmi:type="uml:Generalization" xmi:id="_g1" general="_timer"/>
      <generalization xmi:type="uml:Generalization" xmi:id="_g2">` + general + `</generalization>` + connectors + `
    </packagedElement>`, `<sysml:Block xmi:id="_s1" base_Class="_timer"/><sysml:Block xmi:id="_s2" base_Class="_analysis"/>
  <sysml:BindingConnector xmi:id="_s4" base_Connector="_bind"/><sysml:BindingConnector xmi:id="_s5" base_Connector="_bind2"/><sysml:BindingConnector xmi:id="_s6" base_Connector="_bind3"/>`
}

// binding is a connector id binding the feature at role to the end other.
func binding(id, role, other string) string {
	return `<ownedConnector xmi:type="uml:Connector" xmi:id="` + id + `"><end xmi:type="uml:ConnectorEnd" xmi:id="` + id + `a" role="` + role + `"/><end xmi:type="uml:ConnectorEnd" xmi:id="` + id + `b">` + other + `</end></ownedConnector>`
}

// The pattern is the customization module's block alone: a cut-short, missing or
// other-module reference generalizes no analysis pattern.
func TestMonteCarloAnalysisNeedsTheModulesProvenance(t *testing.T) {
	cases := map[string]string{
		"path cut short": `<general href="MD_customization_for_SysML.mdzip#_mc"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::" referentType="Class"/></xmi:Extension></general>`,
		"no path":        `<general href="MD_customization_for_SysML.mdzip#_mc"/>`,
		"other module":   `<general href="My_Patterns.mdzip#_mc"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis" referentType="Class"/></xmi:Extension></general>`,
	}
	for name, general := range cases {
		t.Run(name, func(t *testing.T) {
			members, applications := monteCarloBlock(general, binding("_bind", "_t", monteCarloRole("Mean")))
			r := migrateDocument(t, members, applications)
			wantNoLine(t, r.Notation, "analysis def")
			wantNote(t, r, "_analysis", migrate.Approximated, "generalization of library type")
			if es := entriesFor(r, "_bind"); len(es) != 1 || es[0].Verdict != migrate.Unmapped || strings.Contains(es[0].Note, "Monte Carlo") {
				t.Errorf("entries for _bind = %+v, want one unmapped entry of no analysis", es)
			}
			wantClean(t, "provenance.sysml", r)
		})
	}
}

// A binding the tool's pattern cannot be read from is refused with its reason,
// and the analysis def written without it, observing nothing it cannot reach.
func TestMonteCarloBindingsRefusedWithReasons(t *testing.T) {
	clock := `
    <packagedElement xmi:type="uml:Class" xmi:id="_clock" name="Clock">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_v" name="v">` + realHref + `</ownedAttribute>
    </packagedElement>`
	cases := []struct {
		name, connectors, id, note string
		members, applications      string
	}{
		{"two features bound to Mean",
			binding("_bind", "_t", monteCarloRole("Mean")) + binding("_bind2", "_u", monteCarloRole("Mean")), "_bind2",
			"binds its Mean to t, u alike, so its statistics summarise no one observable, so the analysis reads no observed and returns no Mean", "", ""},
		{"one end",
			`<ownedConnector xmi:type="uml:Connector" xmi:id="_bind"><end xmi:type="uml:ConnectorEnd" xmi:id="_e">` + monteCarloRole("Mean") + `</end></ownedConnector>`, "_bind",
			"a connector with 1 ends is not migrated", "", ""},
		{"no role at the other end",
			`<ownedConnector xmi:type="uml:Connector" xmi:id="_bind"><end xmi:type="uml:ConnectorEnd" xmi:id="_e1"/><end xmi:type="uml:ConnectorEnd" xmi:id="_e2">` + monteCarloRole("Mean") + `</end></ownedConnector>`, "_bind",
			"the connector binds the simulation tool's MonteCarloAnalysis::Mean to nothing in the document", "", ""},
		{"a statistic of nothing observed",
			binding("_bind", "_t", monteCarloRole("Deviation")), "_bind",
			"the connector binds the simulation tool's MonteCarloAnalysis::Deviation to t, but 'Timer Analysis' inherits MonteCarloAnalysis but binds its Mean to no value of its own, so its statistics summarise no observable, so the statistic is of nothing and is not returned", "", ""},
		{"two statistics bound to each other",
			binding("_bind", "_t", monteCarloRole("Mean")) + `<ownedConnector xmi:type="uml:Connector" xmi:id="_bind2"><end xmi:type="uml:ConnectorEnd" xmi:id="_e1">` + monteCarloRole("Deviation") + `</end><end xmi:type="uml:ConnectorEnd" xmi:id="_e2">` + monteCarloRole("OutOfSpec") + `</end></ownedConnector>`, "_bind2",
			"which lives outside the document", "", ""},
		{"the pattern's block as a role",
			binding("_bind", "_t", `<role href="MD_customization_for_SysML.mdzip#_mc"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis" referentType="Property"/></xmi:Extension></role>`), "_bind",
			"which lives outside the document", "", ""},
		{"Deviation bound twice",
			binding("_bind", "_t", monteCarloRole("Mean")) + binding("_bind2", "_t", monteCarloRole("Deviation")) + binding("_bind3", "_u", monteCarloRole("Deviation")), "_bind3",
			"the connector binds the simulation tool's MonteCarloAnalysis::Deviation to u, which another connector of the block already binds it to", "", ""},
		{"Mean bound to a value of another block",
			binding("_bind", "_v", monteCarloRole("Mean")), "_bind",
			"the connector binds the simulation tool's MonteCarloAnalysis::Mean to Clock::v, which is no feature of Timer Analysis",
			clock, `<sysml:Block xmi:id="_s7" base_Class="_clock"/>`},
		{"Mean bound through a nested path",
			`<ownedAttribute xmi:type="uml:Property" xmi:id="_part" name="clock" type="_clock" aggregation="composite"/>` + binding("_bind", "_v", monteCarloRole("Mean")), "_bind",
			"the connector binds the simulation tool's MonteCarloAnalysis::Mean to v through a nested path, and the statistic is of a value of the block itself",
			clock, `<sysml:Block xmi:id="_s7" base_Class="_clock"/><sysml:NestedConnectorEnd xmi:id="_s8" base_ConnectorEnd="_binda"><propertyPath xmi:idref="_part"/></sysml:NestedConnectorEnd>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			members, applications := monteCarloBlock(monteCarloGeneral, tc.connectors)
			r := migrateDocument(t, members+tc.members, applications+tc.applications)
			wantLine(t, r.Notation, "analysis def 'Timer Analysis Monte Carlo' :> Simulation::MonteCarlo {")
			wantNoLine(t, r.Notation, "analysed.v")
			wantNote(t, r, tc.id, migrate.Unmapped, tc.note)
			wantClean(t, "refused.sysml", r)
		})
	}
}

// A block specializing an analysis binds Mean to a value of its own: its analysis
// observes that value, the general's its own, and neither is a Mean bound twice.
func TestMonteCarloSpecialRebindsTheMean(t *testing.T) {
	members, applications := monteCarloBlock(monteCarloGeneral, binding("_bind", "_t", monteCarloRole("Mean")))
	r := migrateDocument(t, members+`
    <packagedElement xmi:type="uml:Class" xmi:id="_retry" name="Timer Retry">
      <generalization xmi:type="uml:Generalization" xmi:id="_g3" general="_analysis"/>`+binding("_bind2", "_u", monteCarloRole("Mean"))+`
    </packagedElement>`, applications+`<sysml:Block xmi:id="_s7" base_Class="_retry"/>`)
	wantBlock(t, r.Notation, "attribute :>> observed : ScalarValues::Real = analysed.t;", "return Mean : ScalarValues::Real[1] = mean;", "}",
		"part def 'Timer Retry' :> 'Timer Analysis';",
		"analysis def 'Timer Retry Monte Carlo' :> Simulation::MonteCarlo {",
		"subject analysed : 'Timer Retry';")
	wantBlock(t, r.Notation, "attribute :>> observed : ScalarValues::Real = analysed.u;", "return Mean : ScalarValues::Real[1] = mean;", "}")
	wantNote(t, r, "_bind", migrate.Approximated, "written in the analysis def 'Timer Analysis Monte Carlo' as the observed value, of which Mean is returned")
	wantNote(t, r, "_bind2", migrate.Approximated, "written in the analysis def 'Timer Retry Monte Carlo' as the observed value, of which Mean is returned")
	wantClean(t, "rebound.sysml", r)
}

// A statistic slot of an analysis that observes nothing is refused, but for the
// count of runs, which is of the runs themselves.
func TestMonteCarloSlotsOfNothingObservedAreRefused(t *testing.T) {
	members, applications := monteCarloBlock(monteCarloGeneral, "")
	r := migrateDocument(t, members+`
    <packagedElement xmi:type="uml:InstanceSpecification" xmi:id="_run" name="run" classifier="_analysis">
      <slot xmi:type="uml:Slot" xmi:id="_n">
        <definingFeature href="MD_customization_for_SysML.mdzip#_mcN"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis::N" referentType="Property"/></xmi:Extension></definingFeature>
        <value xmi:type="uml:LiteralInteger" xmi:id="_nv" value="3"/>
      </slot>
      <slot xmi:type="uml:Slot" xmi:id="_mean">
        <definingFeature href="MD_customization_for_SysML.mdzip#_mcMean"><xmi:Extension extender="MagicDraw UML 2024x"><referenceExtension referentPath="MD Customization for SysML::analysis patterns::MonteCarloAnalysis::Mean" referentType="Property"/></xmi:Extension></definingFeature>
        <value xmi:type="uml:LiteralReal" xmi:id="_meanv" value="2.5"/>
      </slot>
    </packagedElement>`, applications)
	wantLine(t, r.Notation, "out :>> runs = 3;")
	wantNoLine(t, r.Notation, "mean = 2.5")
	wantNote(t, r, "_n", migrate.Mapped, "")
	wantNote(t, r, "_mean", migrate.Unmapped, "the slot holds the simulation tool's MonteCarloAnalysis::Mean, but 'Timer Analysis' inherits MonteCarloAnalysis but binds its Mean to no value of its own, so its statistics summarise no observable, so the statistic is of nothing")
	wantClean(t, "unobserved.sysml", r)
}

// montecarlo_diagram.xmi: the parametric diagram of a block of the pattern shows the
// symbols of Mean, Deviation and OutOfSpec, the two values and the three bindings. The
// view exposes the analysis def in place of the Mean symbol and the def's returns in
// place of the other statistics, positions them where the symbols were, and routes the
// Mean binding along observed, whose value binds it to the analysed value; a binding to
// a return, bound to a statistic nothing draws, is positioned but draws no edge.
func TestMonteCarloParametricViewExposesTheAnalysis(t *testing.T) {
	r := migrateStreamLaidOut(t, "montecarlo_diagram", false)
	wantClean(t, "montecarlo_diagram", r)
	wantBlock(t, r.Notation, "part def 'Settling Analysis' :> Sensor {",
		"view 'Settling Parametrics' {",
		"expose 'Settling Analysis Monte Carlo'::observed;",
		"expose 'Settling Analysis Monte Carlo'::Deviation;",
		"expose 'Settling Analysis Monte Carlo'::OutOfSpec;",
		"expose Sensor::settleTime;",
		"expose Sensor::misses;",
		"expose 'Settling Analysis Monte Carlo';")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about 'Settling Analysis Monte Carlo' { x = 70; y = 56; width = 81; height = 26; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about 'Settling Analysis Monte Carlo'::Deviation { x = 200; y = 56; width = 102; height = 26; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about 'Settling Analysis Monte Carlo'::OutOfSpec { x = 340; y = 56; width = 111; height = 26; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about Sensor::settleTime { x = 35; y = 98; width = 156; height = 26; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Route about 'Settling Analysis Monte Carlo'::observed { points = (105, 82, 105, 98); }")
	wantLine(t, r.Notation, "render Views::asInterconnectionDiagram;")
	// The model mapping is unchanged: the bindings are the analysis def's members, no connector.
	wantNoLine(t, r.Notation, "binding")
	wantNoLine(t, r.Notation, "bind ")
	wantBlock(t, r.Notation, "analysis def 'Settling Analysis Monte Carlo' :> Simulation::MonteCarlo {",
		"subject analysed : 'Settling Analysis';",
		"perform action run ::> analysed.settle;",
		"attribute :>> observed : ScalarValues::Real = analysed.settleTime;",
		"return Mean : ScalarValues::Real[1] = mean;",
		"out Deviation : ScalarValues::Real[0..1] = deviation;",
		"out OutOfSpec : ScalarValues::Integer[1] = outOfSpec;",
		"}")

	wantNote(t, r, "_bindMean", migrate.Approximated, "the binding to the simulation tool's MonteCarloAnalysis::Mean is written in the analysis def 'Settling Analysis Monte Carlo' as the observed value, of which Mean is returned")
	wantNote(t, r, "_bindDeviation", migrate.Approximated, "the binding to the simulation tool's MonteCarloAnalysis::Deviation is written in the analysis def 'Settling Analysis Monte Carlo' as the returned Deviation, bound to deviation")
	wantNote(t, r, "_diag_par", migrate.Approximated, "a SysML Parametric Diagram written as a view rendered asInterconnectionDiagram; "+
		"6 of 14 shown elements are not written and not exposed; laid out from the diagram's own symbol stream: "+
		"5 of 11 shown elements positioned (6 not exposed), 1 of 3 connectors routed (2 not drawn)")
	for _, e := range entriesFor(r, "_diag_par") {
		if strings.Contains(e.Note, "not written)") {
			t.Errorf("diagram note reports a binding's route as not written: %s", e.Note)
		}
	}
	report := reportText(t, r)
	for _, want := range []string{"# routes of Connector: 1 written", "# routes of Connector: 2 not drawn",
		"placements: 5 of 11 written (6 not exposed, 0 resolving to no element); routes: 1 of 3 written (2 not pinned, 0 resolving to no element)"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}

	// The rendering draws the analysis def where the Mean symbol was, its observed
	// value a pin on its border, and the binding from that pin to the analysed
	// value along the symbol's route.
	s := session(t, r)
	rendering, err := s.ViewRendering("'Settling Analysis'::'Settling Parametrics'")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	nodes := map[string]*view.Node{}
	for _, root := range rendering.Roots {
		nodes[root.Name] = root
		for _, child := range root.Children {
			nodes[child.Name] = child
		}
	}
	analysis, settleTime := nodes["'Settling Analysis Monte Carlo'"], nodes["Sensor::settleTime"]
	if analysis == nil || settleTime == nil || nodes["Sensor::misses"] == nil || len(nodes) != 3 {
		t.Fatalf("rendering draws %v, want the analysis def, settleTime and misses alone", slices.Sorted(maps.Keys(nodes)))
	}
	if analysis.Kind != "analysis def" || analysis.Geometry == nil || *analysis.Geometry != (view.Geometry{X: 70, Y: 56, Width: 81, Height: 26, HasSize: true}) {
		t.Errorf("analysis def node = %s %+v, want the Mean symbol's geometry", analysis.Kind, analysis.Geometry)
	}
	pins := map[string]view.Port{}
	for _, pin := range analysis.Ports {
		pins[pin.Name] = pin
	}
	observed, ok := pins["observed"]
	if !ok || pins["Mean"].Direction != view.PortOut || pins["Deviation"].Direction != view.PortOut || pins["analysed"].Direction != view.PortUndirected {
		t.Fatalf("analysis def pins = %+v, want analysed, observed and the out statistics Mean and Deviation", analysis.Ports)
	}
	if len(rendering.Edges) != 1 {
		t.Fatalf("edges = %+v, want the one binding", rendering.Edges)
	}
	edge := rendering.Edges[0]
	if edge.From != analysis.ID || edge.FromPort != observed.ID || edge.To != settleTime.ID || edge.ToPort != "" || edge.Kind != view.EdgeBinding || edge.Label != "binding" {
		t.Errorf("edge = %+v, want a binding from pin %s of %s to %s", edge, observed.ID, analysis.ID, settleTime.ID)
	}
	if !reflect.DeepEqual(edge.Route, []view.Point{{X: 105, Y: 82}, {X: 105, Y: 98}}) {
		t.Errorf("edge route = %v, want the symbol's (105,82) (105,98)", edge.Route)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
}
