package model_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// While an expose's target resolves, the view's sibling exposes are suspended,
// and a metadata usage in the view asks what the view inherits. Resolving the
// redefinition `kind` for that question once failed there and was reported,
// though `v` inherits `kind` from `Dia` (issue #649).
func TestARedefinitionInAViewWithExposesResolves(t *testing.T) {
	const src = `package Lib {
    private import ScalarValues::*;
    metadata def Tag;
    view def Dia { attribute kind : String; }
}
package P1 { private import Lib::*; }
package P2 { private import Lib::*; }
package App {
    private import Lib::*;
    view v : Dia {
        expose P1::*;
        expose P2::*;
        @Tag;
        attribute :>> kind = "grid";
    }
}
`
	ws := model.NewWorkspace()
	ws.Open("redef.sysml", []byte(src), 1)
	for _, d := range ws.Diagnostics("redef.sysml") {
		t.Errorf("the model is valid SysML; got %v", d)
	}
}

// The same redefinition naming nothing `v` inherits is still reported.
func TestAnUnresolvedRedefinitionInAViewWithExposesIsReported(t *testing.T) {
	const src = `package Lib {
    metadata def Tag;
    view def Dia { attribute kind; }
}
package P1 { private import Lib::*; }
package P2 { private import Lib::*; }
package App {
    private import Lib::*;
    view v : Dia {
        expose P1::*;
        expose P2::*;
        @Tag;
        attribute :>> nope;
    }
}
`
	ws := model.NewWorkspace()
	ws.Open("redef.sysml", []byte(src), 1)
	if len(ws.Diagnostics("redef.sysml")) == 0 {
		t.Fatal("`nope` names nothing v inherits; expected an unresolved-reference diagnostic")
	}
}
