package passes

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The documents of one batch settle a namespace's import collisions once: an
// analysis taking what an earlier one settled reports the same and still
// records the reads that settling made, so its interface record depends on
// everything along the chain of re-exports the collisions were decided over.
func TestBatchSharesImportCollisionsWithTheirReads(t *testing.T) {
	const n = 6
	docs := map[string]string{}
	var names []string
	for k := 0; k < n; k++ {
		var src strings.Builder
		fmt.Fprintf(&src, "package P%d {\n    private import Q%d::*;\n", k, k)
		if k > 0 {
			fmt.Fprintf(&src, "    public import P%d::*;\n    part q%d : D%d;\n", k-1, k, k-1)
		}
		fmt.Fprintf(&src, "    part def D%d;\n    part e%d : E%d;\n}\n", k, k, k)
		name := fmt.Sprintf("p%d.sysml", k)
		docs[name] = src.String()
		docs[fmt.Sprintf("q%d.sysml", k)] = fmt.Sprintf("package Q%d {\n    part def E%d;\n}\n", k, k)
		names = append(names, name, fmt.Sprintf("q%d.sysml", k))
	}
	// The user imports the chain's end and enumerates it, so what it hides on
	// the way down was settled, with the import targets along the chain
	// memoized first, by whichever analysis came before; the private imports
	// only settling looks into, the user reads through the shared memo alone.
	const user = "user.sysml"
	docs[user] = fmt.Sprintf("package U {\n    private import P%d::*;\n    part u : D0;\n}\n", n-1)
	names = append(names, user)
	last := names[2*(n-1)]
	analyze := func(idx *symbols.Index, roots map[string]*ast.RootNamespace, batch *Batch, first string) ([]diag.Diagnostic, *resolve.Reads) {
		PrepareBatch(idx, batch)
		if batch.Collisions == nil {
			t.Fatal("preparing a batch shares no collisions memo")
		}
		if first != "" {
			AnalyzeInBatch(first, source.KindSysML, roots[first], nil, idx, Options{}, batch)
		}
		return AnalyzeInBatch(user, source.KindSysML, roots[user], nil, idx, Options{}, batch)
	}
	idx, roots := indexedBatch(t, docs)
	diags, settled := analyze(idx, roots, &Batch{Documents: names, Record: true}, "")
	idx, roots = indexedBatch(t, docs)
	again, taken := analyze(idx, roots, &Batch{Documents: names, Record: true}, last)
	if !reflect.DeepEqual(diags, again) {
		t.Errorf("the analysis taking the shared collisions reports %v, want what settling them reported: %v", again, diags)
	}
	if settled == nil || taken == nil || len(ofChain(settled.Namespaces)) == 0 || len(ofChain(settled.Docs)) == 0 {
		t.Fatalf("recorded reads %v and %v, want both over the chain", settled, taken)
	}
	// The audits' gathers are shared too, so only the chain's own reads compare.
	for _, chain := range []struct{ settled, taken []string }{
		{settled.Namespaces, taken.Namespaces}, {settled.Names, taken.Names}, {settled.Docs, taken.Docs},
	} {
		if want, got := ofChain(chain.settled), ofChain(chain.taken); !reflect.DeepEqual(want, got) {
			t.Errorf("the analysis taking the shared collisions read %v, want what settling them read: %v", got, want)
		}
	}
}

func ofChain(reads []string) []string {
	var out []string
	for _, read := range reads {
		if strings.HasPrefix(read, "P") || strings.HasPrefix(read, "p") || strings.HasPrefix(read, "Q") || strings.HasPrefix(read, "q") {
			out = append(out, read)
		}
	}
	return out
}
