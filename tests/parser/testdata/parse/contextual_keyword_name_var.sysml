// `var` marks a variable feature (KerML.xtext BasicFeaturePrefix,
// `isVariable ?= 'var'`) and names a feature everywhere else, as it does in
// `examples/parser_features_demo_action_semantics.sysml`.
package P {
	part def Counter {
		attribute var : Integer;
		attribute next = var + 1;

		// The prefix still qualifies the kind that follows it.
		var attribute total : Integer;
		derived var feature scaled : Integer;
	}

	action IterateSequence {
		attribute index : Integer;
		attribute var : Integer;

		assign index := 1;
		then while index <= 10 {
			assign var := index;
			then assign index := index + 1;
		}
	}
}
