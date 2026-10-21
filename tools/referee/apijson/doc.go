// Package apijson is the derived-property differential: it exports every model
// file of the corpus roots as the SysML v2 API's element JSON through this
// repository's writer and through the pinned pilot's JsonElementProcessingFacade
// (derived properties included), matches the elements by a stable key, and
// buckets every property of every matched element as present-equal,
// present-different, ours-missing or pilot-missing. Bucketed per (metaclass,
// property), the counts read as a scorecard of the metamodel's derive*
// constraints; the referee is advisory and never decides a pass.
package apijson
