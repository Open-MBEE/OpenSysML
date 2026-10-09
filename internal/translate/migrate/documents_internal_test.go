package migrate

import "testing"

func TestChainEmptyTreatsSharedQueryAsNonEmpty(t *testing.T) {
	c := chain{ctx: qshared(qcall("Descendants"))}
	if c.empty() {
		t.Fatal("shared query source reported empty")
	}
}
