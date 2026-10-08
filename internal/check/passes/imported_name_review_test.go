// SPDX-License-Identifier: Apache-2.0

package passes

import "testing"

func TestImportedNameClashReview(t *testing.T) {
	const defs = `package Left { part def Engine; } package Right { part def Engine; }
package Other { part def Engine; }`
	for _, tc := range []struct {
		name, source string
		clean        bool
	}{
		{"root", `private import Left::*; private import Right::*; part e : Engine;`, false},
		{"root-reversed", `private import Right::*; private import Left::*; part e : Engine;`, false},
		{"root-repeat", `private import Left::*; private import Left::*; part e : Engine;`, true},
		{"hidden-reexport-valid-import", `package Middle { public import Left::*; public import Right::*; }
package Consumer { private import Middle::*; private import Other::*; part e : Engine; }`, true},
		{"hidden-reexport-valid-import-reversed", `package Middle { public import Right::*; public import Left::*; }
package Consumer { private import Other::*; private import Middle::*; part e : Engine; }`, true},
		{"inherited", `part def ParentType { protected import Left::*; protected import Right::*; }
part def Sub :> ParentType { part e : Engine; }`, false},
		{"inherited-reversed", `part def ParentType { protected import Right::*; protected import Left::*; }
part def Sub :> ParentType { part e : Engine; }`, false},
		{"inherited-repeat", `part def ParentType { protected import Left::*; protected import Left::*; }
part def Sub :> ParentType { part e : Engine; }`, true},
		{"inherited-feature", `package FeaturesLeft { part item; } package FeaturesRight { part item; }
part def ParentType { protected import FeaturesLeft::*; protected import FeaturesRight::*; }
part def Sub :> ParentType { part e :> item; }`, false},
		{"inherited-feature-repeat", `package FeaturesLeft { part item; }
part def ParentType { protected import FeaturesLeft::*; protected import FeaturesLeft::*; }
part def Sub :> ParentType { part e :> item; }`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.clean {
				wantLibraryClean(t, defs+tc.source)
				return
			}
			if diags := libraryDiags(t, defs+tc.source); !hasCode(diags, "unresolved") {
				t.Fatalf("conflicting imports must remain hidden: %v", diags)
			}
		})
	}
}
