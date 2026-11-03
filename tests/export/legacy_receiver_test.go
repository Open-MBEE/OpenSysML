package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A call reaching a calc usage typed by no Function (`: Real`, or so through a subsetting)
// states the implied library Calculation as its function and reads back from the graph alone.
func TestReceiverCalleeImpliedFunctionRoundTrips(t *testing.T) {
	src := "package P {\n\tpart def Pt {\n\t\tcalc real : ScalarValues::Real;\n\t\tcalc viaReal :> real;\n\t}\n\tpart def D {\n\t\tpart p : Pt;\n\t\tattribute a = p.real(1);\n\t\tattribute b = p.viaReal(1);\n\t}\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if strings.Count(string(turtle), "sysml:function ") != 2 {
		t.Fatalf("want both calls to state a function:\n%s", turtle)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("from the graph alone: %v", err)
	}
	for _, want := range []string{"attribute a = p.real(1);", "attribute b = p.viaReal(1);"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("%s did not read back:\n%s", want, back)
		}
	}
}

// An older graph that wrote the receiver `q.p` as a bare sysml:operand
// FeatureChainExpression still imports as the call `q.p.f(1)`.
func TestLegacyReceiverOperandGraphStillImports(t *testing.T) {
	turtle, err := os.ReadFile(filepath.Join("testdata", "convert", "legacy_receiver_operand.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(turtle), "sysml:operand expr:Legacy__D__d_pvalue_poperand ;") {
		t.Fatal("the fixture does not carry the legacy operand shape")
	}
	back, err := convert.Convert("legacy.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("from the legacy graph: %v", err)
	}
	if !strings.Contains(string(back), "attribute d = q.p.f(1);") {
		t.Errorf("the legacy graph did not read back as q.p.f(1):\n%s", back)
	}
}
