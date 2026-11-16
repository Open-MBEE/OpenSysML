package resolve_test

import "testing"

// The empty unrestricted name (KerML §8.2.2.3) names its package like any
// other: its members are reached through it, at the root as when nested.
func TestEmptyNamedPackageQualifiesItsMembers(t *testing.T) {
	for _, tc := range []struct {
		name, src, written string
		unresolved         bool
	}{
		// A diagnostic names what was written without its quotes.
		{"root member is not a root name", `package '' { part y; } part z :> y;`, "y", true},
		{"root member through the name", `package '' { part y; } part z :> ''::y;`, "''::y", false},
		{"nested member is not an owner name", `package P { package '' { part y; } part z :> y; }`, "y", true},
		{"nested member through the name", `package P { package '' { part y; } part z :> ''::y; }`, "''::y", false},
		{"imported from the root", `package '' { part y; } package C { private import ''::*; part z :> y; }`, "y", false},
		{"imported by name", `package P { package '' { part y; } } package C { private import P::''::y; part z :> y; }`, "y", false},
		{"the empty-named element", `package P { part ''; part z :> ''; }`, "''", false},
		{"nothing is named empty", `package P { part; part z :> ''; }`, "", true},
		{"whitespace is another name", `package P { part ''; part z :> '   '; }`, "   ", true},
		{"the empty short name", `package P { part <''> a; part z :> ''; }`, "''", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := unresolvedMessage(t, tc.src, tc.written)
			if (got != "") != tc.unresolved {
				t.Errorf("unresolved = %q, want unresolved %v", got, tc.unresolved)
			}
		})
	}
}
