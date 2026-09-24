package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const subjectModel = `
	package test {
		part def Truck {
			attribute payload : Real;
		}
		part loadedTruck : Truck {
			attribute redefines payload = 5.0;
		}
		requirement def PayloadReq {
			subject truck : Truck;
			require constraint { truck.payload <= 10.0 }
		}
		requirement payloadHolds : PayloadReq {
			subject truck = loadedTruck;
		}
	}
`

// TestUnboundSubjectIsReportedAsSuch: checking a requirement nothing supplies a
// subject to says the subject is unbound and how to supply one, rather than
// reporting it as a feature that carries no value.
func TestUnboundSubjectIsReportedAsSuch(t *testing.T) {
	file := parseAndBuild(t, subjectModel)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "PayloadReq", ast.DefRequirement)
	if sym == nil {
		t.Fatal("PayloadReq not found")
	}

	holds, err := ctx.EvaluateRequirement(sym, rootScope)
	if !errors.Is(err, ErrUnboundSubject) {
		t.Fatalf("PayloadReq = %v, %v; want ErrUnboundSubject", holds, err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an unbound subject is not a violation")
	}
	for _, want := range []string{"truck", "PayloadReq", "satisfy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestBoundSubjectIsEvaluated: a requirement usage binding its subject is
// checked against the object it names.
func TestBoundSubjectIsEvaluated(t *testing.T) {
	file := parseAndBuild(t, subjectModel)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "payloadHolds", ast.DefRequirement)
	if sym == nil {
		t.Fatal("payloadHolds not found")
	}

	holds, err := ctx.EvaluateRequirement(sym, rootScope)
	if err != nil {
		t.Fatalf("payloadHolds: %v", err)
	}
	if !holds {
		t.Error("payloadHolds should hold: the truck carries 5.0 of a 10.0 limit")
	}
}
