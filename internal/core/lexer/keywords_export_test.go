package lexer

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestKeywordsExportsList(t *testing.T) {
	kws := source.Keywords()
	if len(kws) == 0 {
		t.Fatal("source.Keywords() returned empty")
	}
	found := false
	for _, k := range kws {
		if k == "package" {
			found = true
			break
		}
	}
	if !found {
		t.Error("source.Keywords() missing 'package'")
	}
	// Must be a copy: mutating the result must not affect subsequent calls.
	kws[0] = "MUTATED"
	if source.Keywords()[0] == "MUTATED" {
		t.Error("source.Keywords() leaked internal slice")
	}
}
