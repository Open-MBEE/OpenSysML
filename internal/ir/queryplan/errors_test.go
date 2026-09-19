package queryplan

import "testing"

// Every kind's message is part of the diagnostic contract, so each one is
// spelled out rather than derived from the kind.
func TestErrorMessages(t *testing.T) {
	full := Error{
		Query:     "Q",
		Target:    "T",
		Parameter: "p",
		Path:      []string{"A", "B", "A"},
		Expected:  "String",
		Actual:    "Integer",
	}

	cases := []struct {
		kind ErrorKind
		want string
	}{
		{ErrorInvalidContext, "document query planning requires an index, resolver, and semantic model"},
		{ErrorLibraryUnavailable, "document query library is unavailable"},
		{ErrorNotQueryDefinition, "Q is not a document query definition"},
		{ErrorMissingResultParameter, "query Q has no result parameter"},
		{ErrorInvalidParameter, "query Q parameter p must be an input parameter"},
		{ErrorMissingResult, "query Q has no result expression"},
		{ErrorConflictingResult, "query Q inherits conflicting result expressions"},
		{ErrorUnsupportedResult, "query Q result must be one query expression"},
		{ErrorUnknownInvocation, "query Q invokes unknown operation T"},
		{ErrorAmbiguousInvocation, "query Q invokes T, ambiguous between A, B, A"},
		{ErrorPositionalQueryArgs, "query Q must invoke query T with named arguments"},
		{ErrorDuplicateArgument, "query Q binds parameter p more than once"},
		{ErrorUnknownArgument, "query Q binds unknown parameter p of T"},
		{ErrorMissingArgument, "query Q does not bind required parameter p of T"},
		{ErrorArgumentCount, "query Q invokes T with the wrong number of arguments"},
		{ErrorArgumentType, "query Q binds parameter p of T with type Integer, expected String"},
		{ErrorArgumentMultiplicity, "query Q binds parameter p of T with multiplicity Integer, expected String"},
		{ErrorCompositionCycle, "document query composition cycle: A -> B -> A"},
		{ErrorUnknownParameter, "query Q references unknown parameter p"},
		{ErrorUnsupportedExpression, "query Q contains an unsupported result expression"},
		{ErrorKind("no-such-kind"), "query planning failed for Q"},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			err := full
			err.Kind = tc.kind
			if got := err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
