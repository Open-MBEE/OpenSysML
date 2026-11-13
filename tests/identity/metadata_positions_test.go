package identity_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestSharedTargetMetadataBodyIdentities(t *testing.T) {
	var src strings.Builder
	src.WriteString(`package P {
	metadata def M { attribute a; attribute b; }
	part target;
	@IdentityMetadata::ElementId about target { id = "target-id"; }
	`)
	const count = 200
	for i := range count {
		fmt.Fprintf(&src, "@M about target { a = %d; b = %d; }\n", i, i)
	}
	src.WriteString("}")
	table, _ := buildTable(t, src.String())
	got := map[string]bool{}
	for _, sym := range table.Symbols() {
		if info, ok := table.Info(sym); ok {
			got[info.EffectiveID] = true
		}
	}
	for i := range count {
		for _, feature := range []string{"a", "b"} {
			fqn := fmt.Sprintf("P::@%d::%s", i+2, feature)
			if !got[rdf.EncodeElementID(fqn)] {
				t.Errorf("missing identity for %s", fqn)
			}
		}
	}
}
