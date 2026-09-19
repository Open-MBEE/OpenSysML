package model

import "testing"

// A name a namespace imported privately is not re-exported by it, on the
// unqualified route as much as the qualified one.
func TestPrivateWildcardImportIsNotReExportedAcrossDocuments(t *testing.T) {
	docs := map[string]string{
		"lower.sysml": `package Lower { part def Hidden; }`,
		"mid.sysml":   `package Mid { private import Lower::*; }`,
		"app.sysml":   `package App { private import Mid::*; part def Thing { part p : Hidden; } }`,
	}
	if found := openAll(t, "app.sysml", docs); len(found) != 1 {
		t.Fatalf("expected one unresolved finding for the privately imported Hidden, got %d: %v",
			len(found), found)
	}
	if found := openAll(t, "mid.sysml", docs); len(found) != 0 {
		t.Fatalf("Mid itself must still see Hidden, got %v", found)
	}
}
