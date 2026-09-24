// `at`, `while`, `merge` and `decide` are literals of SysML.xtext only, so a
// KerML file reserves none of them: they declare, reference and import as any
// other name does (docs/reference/grammar/conformance-audit.md).
package SysMLOnlyWordsAreNames {
	class Timeslice;

	behavior Occurrences {
		expr at { in ts : Timeslice; return result : Timeslice; }
		expr while { in ts : Timeslice; return result : Timeslice = at(ts); }
		feature merge : Timeslice;
		feature decide : Timeslice;
		feature chained = merge.at;
	}

	package Uses {
		public import Occurrences::merge;
		feature refersToAt = Occurrences::at;
	}
}
