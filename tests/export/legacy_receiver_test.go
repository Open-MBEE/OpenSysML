package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A graph written before the conversion owned a receiver-reached call's chain
// Feature — the receiver `q.p` as a bare sysml:operand FeatureChainExpression —
// still reads back as the call it came from.
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
