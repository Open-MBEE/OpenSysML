package repl

// InstantiationReport is what creating an object produced: the lines a caller
// prints, and the diagnostics materializing the object's feature values reported.
type InstantiationReport struct {
	Lines []string
	// FeatureValueErrors are the feature values that could not be materialized, in the order they
	// were read: a default whose value count does not conform to the feature's
	// multiplicity is one.
	FeatureValueErrors []string
	// Bounded is true when materialization stopped short — at its budget, or at
	// nesting it does not descend into — so the feature values it did not reach were not
	// checked rather than found clean.
	Bounded bool
}

// InstantiateReport creates the named object and materializes its feature values, so a
// non-interactive caller reports what materialization found rather than leaving
// it to whoever reads a feature value next. An object that cannot be created at all is an
// error; a feature value that cannot be materialized is a finding about the model.
func (s *Session) InstantiateReport(name string) (InstantiationReport, error) {
	defer s.enter()()
	lines, err := s.instantiateLines(name)
	if err != nil {
		return InstantiationReport{}, err
	}
	report := InstantiationReport{Lines: lines}

	_, fqn, lerr := s.lookupSymbol(name)
	if lerr != nil {
		// Unreachable: the object was just created under this name.
		return report, nil
	}
	inst, ok := s.instances[fqn]
	if !ok {
		return report, nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return report, err
	}
	fvErrs, bounded := ctx.MaterializationErrors(inst)
	for _, fvErr := range fvErrs {
		report.FeatureValueErrors = append(report.FeatureValueErrors, fvErr.Error())
	}
	report.Bounded = bounded
	// The steps reading those feature values are this object's, not the next command's.
	report.Lines = append(report.Lines, s.drainTrace()...)
	return report, nil
}
