package model

import (
	"sort"
	"testing"
)

func TestDocumentNamesSnapshot(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte("package A;"), 1)
	ws.Open("b.sysml", []byte("package B;"), 1)

	got := ws.DocumentNames()
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a.sysml" || got[1] != "b.sysml" {
		t.Fatalf("DocumentNames = %v, want [a.sysml b.sysml]", got)
	}
}
