package queryexec

import "testing"

func TestExecuteRelatedDerivationUsages(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_derivation.sysml")

	// A typed usage and a `#derivation` usage read their clause ends in order;
	// the plain connection from systemMass states no derivation.
	assertRelated(t, fixture, "systemMass", "derivation", "outgoing", 1,
		[]string{"mirrorMass"})
	assertRelated(t, fixture, "mirrorMass", "derivation", "outgoing", 1,
		[]string{"segmentMass"})
	assertRelated(t, fixture, "mirrorMass", "derivation", "incoming", 1,
		[]string{"systemMass"})
	assertRelated(t, fixture, "segmentMass", "derivation", "incoming", 1,
		[]string{"mirrorMass"})

	// maxDepth follows derived-of-derived chains in both directions.
	assertRelated(t, fixture, "systemMass", "derivation", "outgoing", 2,
		[]string{"mirrorMass", "segmentMass"})
	assertRelated(t, fixture, "segmentMass", "derivation", "incoming", 2,
		[]string{"mirrorMass", "systemMass"})

	// An n-ary clause derives every later end from the first.
	assertRelated(t, fixture, "instrumentMass", "derivation", "outgoing", 1,
		[]string{"cameraMass", "spectrographMass"})
	assertRelated(t, fixture, "spectrographMass", "derivation", "incoming", 1,
		[]string{"instrumentMass"})
	assertRelated(t, fixture, "instrumentMass", "derivation", "incoming", 1, nil)
}

func TestExecuteRelatedDerivationBodyEndRoles(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_derivation.sysml")

	// Roles stated by `#original`/`#derive` metadata and by subsetting the
	// library's role features override end order.
	assertRelated(t, fixture, "pointingAccuracy", "derivation", "outgoing", 1,
		[]string{"trackingAccuracy", "guidingAccuracy"})
	assertRelated(t, fixture, "trackingAccuracy", "derivation", "incoming", 1,
		[]string{"pointingAccuracy"})
	assertRelated(t, fixture, "guidingAccuracy", "derivation", "incoming", 1,
		[]string{"pointingAccuracy"})
	assertRelated(t, fixture, "trackingAccuracy", "derivation", "outgoing", 1, nil)
}

func TestExecuteRelatedDerivationDefinitions(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_derivation.sysml")

	// A usage of a derivation definition takes its roles from the definition's
	// ends, whatever order its own ends are written in.
	assertRelated(t, fixture, "pointingBudget", "derivation", "outgoing", 1,
		[]string{"jitterBudget", "driftBudget"})
	assertRelated(t, fixture, "driftBudget", "derivation", "incoming", 1,
		[]string{"pointingBudget"})

	// A definition's typed ends relate the requirement definitions: the
	// `#derivation` definition and the migrator's specializing form alike.
	assertRelated(t, fixture, "MassRequirement", "derivation", "outgoing", 1,
		[]string{"MirrorMassRequirement", "InstrumentMassRequirement"})
	assertRelated(t, fixture, "MirrorMassRequirement", "derivation", "outgoing", 1,
		[]string{"SegmentMassRequirement"})
	assertRelated(t, fixture, "MassRequirement", "derivation", "outgoing", 2,
		[]string{"MirrorMassRequirement", "InstrumentMassRequirement", "SegmentMassRequirement"})
	assertRelated(t, fixture, "SegmentMassRequirement", "derivation", "incoming", 2,
		[]string{"MirrorMassRequirement", "MassRequirement"})
}

func TestExecuteRelatedRefinement(t *testing.T) {
	fixture := loadExecutionFixtureFile(t, "testdata/tmt_derivation.sysml")

	// Prefix and body metadata forms, from client to supplier.
	assertRelated(t, fixture, "pointingBudget", "refinement", "outgoing", 1,
		[]string{"pointingAccuracy"})
	assertRelated(t, fixture, "pointingAccuracy", "refinement", "incoming", 1,
		[]string{"pointingBudget"})
	assertRelated(t, fixture, "mirrorDesign", "refinement", "outgoing", 1,
		[]string{"mirrorMass"})
	assertRelated(t, fixture, "mirrorMass", "refinement", "incoming", 1,
		[]string{"mirrorDesign"})

	// Several clients and suppliers relate every client to every supplier.
	assertRelated(t, fixture, "mountDesign", "refinement", "outgoing", 1,
		[]string{"trackingAccuracy", "guidingAccuracy"})
	assertRelated(t, fixture, "guidingAccuracy", "refinement", "incoming", 1,
		[]string{"mountDesign", "guiderDesign"})

	// A plain dependency states no refinement; a refinement is no derivation.
	assertRelated(t, fixture, "thermalDesign", "refinement", "outgoing", 1, nil)
	assertRelated(t, fixture, "systemMass", "refinement", "incoming", 1, nil)
	assertRelated(t, fixture, "pointingBudget", "derivation", "incoming", 1, nil)
}
