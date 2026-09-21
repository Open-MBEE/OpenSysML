package symbols

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// kindMappingDigest pins the declaration-kind → SymbolKind mapping, which the
// on-disk library index persists and must be invalidated for when it changes.
const kindMappingDigest = "6891b0021bc3879a"

func TestSymbolKindMappingIsPinnedToTheIndexFormatVersion(t *testing.T) {
	var b strings.Builder
	for k := ast.DefPart; k <= ast.DefBool; k++ {
		fmt.Fprintf(&b, "def %d=%d\n", int(k), int(definitionSymbolKind(k)))
	}
	for k := ast.UsagePart; k <= ast.UsageBool; k++ {
		fmt.Fprintf(&b, "usage %d=%d\n", int(k), int(usageSymbolKind(k)))
	}
	sum := sha256.Sum256([]byte(b.String()))
	got := hex.EncodeToString(sum[:8])
	if got != kindMappingDigest {
		t.Fatalf("declaration-kind to SymbolKind mapping changed (digest %s, want %s).\n"+
			"Persisted library index records carry these kinds: bump formatVersion in "+
			"internal/workspace/libs/record.go so cached records are invalidated, then update "+
			"kindMappingDigest to the new value.", got, kindMappingDigest)
	}
}
