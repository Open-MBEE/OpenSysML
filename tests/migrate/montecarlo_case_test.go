package migrate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
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
	wantLine(t, r.Notation, "return Mean : ScalarValues::Real = mean;")
	wantLine(t, r.Notation, "out Deviation : ScalarValues::Real[0..1] = deviation;")
	wantLine(t, r.Notation, "out OutOfSpec : ScalarValues::Integer = outOfSpec;")
	wantBlock(t, r.Notation, "return Mean : ScalarValues::Real = mean;",
		"out Deviation : ScalarValues::Real[0..1] = deviation;",
		"out OutOfSpec : ScalarValues::Integer = outOfSpec;",
		"}")
	wantNoLine(t, r.Notation, "Median :")
	wantNoLine(t, r.Notation, "= median")
	wantLine(t, r.Notation, "analysis 'Monte Carlo' : 'Settling Analysis Monte Carlo' {")
	wantLine(t, r.Notation, "subject :>> analysed : 'settling of 5 runs';")
	wantLine(t, r.Notation, "out :>> deviation = 0.6;")
	wantBlock(t, r.Notation, "analysis def 'Retried Settling Monte Carlo' :> Simulation::MonteCarlo {",
		"subject analysed : 'Retried Settling';",
		"perform action run ::> analysed.settle;",
		"attribute :>> observed : ScalarValues::Real = analysed.settleTime;",
		"return Mean : ScalarValues::Real = mean;",
		"out N : ScalarValues::Integer = runs;",
		"out OutOfSpec : ScalarValues::Integer = outOfSpec;",
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
	wantBlock(t, r.Notation, "attribute :>> observed : ScalarValues::Real = analysed.t;", "return Mean : ScalarValues::Real = mean;", "}",
		"part def 'Timer Retry' :> 'Timer Analysis';",
		"analysis def 'Timer Retry Monte Carlo' :> Simulation::MonteCarlo {",
		"subject analysed : 'Timer Retry';")
	wantBlock(t, r.Notation, "attribute :>> observed : ScalarValues::Real = analysed.u;", "return Mean : ScalarValues::Real = mean;", "}")
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
