// A declaration kind of KerML.xtext that SysML.xtext does not spell is a name
// before a body or a leading multiplicity too: `step { }`, `step [1];` and
// `namespace { }` declare keywordless usages named step and namespace, as the
// pinned validator reads them (clean), not anonymous KerML declarations.
package KerMLKindWordsAreKeywordlessUsages {
	part def Sweep {
		step { }
		step [1];
		attribute connector : ScalarValues::Real;
	}
	namespace { }
}
