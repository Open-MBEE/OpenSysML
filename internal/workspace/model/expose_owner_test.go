package model

import (
	"strings"
	"testing"
)

// exposeOwnerFindings returns the diagnostics of a document about where its
// `expose` is written, as "severity: message".
func exposeOwnerFindings(t *testing.T, uri string, docs map[string]string) []string {
	t.Helper()
	ws := NewWorkspace()
	for name, src := range docs {
		ws.Open(name, []byte(src), 1)
		defer ws.Close(name)
	}
	var out []string
	for _, d := range ws.Diagnostics(uri) {
		if strings.Contains(d.Message, "expose") {
			out = append(out, d.Severity.String()+": "+d.Message)
		}
	}
	return out
}

// Expose is a ViewBodyItem alone (SysML.xtext): legal in a view usage, a
// nonstandard-notation warning in a view def body, a syntax error elsewhere.
func TestExposeOwnerAcrossDocuments(t *testing.T) {
	lib := `package Lib { part def Pub; }`
	docs := map[string]string{
		"lib.sysml":  lib,
		"ok.sysml":   `package Ok { view v { expose Lib::**; } }`,
		"warn.sysml": `package Warn { view def V { expose Lib::**; } }`,
		"bad.sysml":  `package Bad { part def D { expose Lib::**; } }`,
	}

	if found := exposeOwnerFindings(t, "ok.sysml", docs); len(found) != 0 {
		t.Errorf("expose in a view usage must be legal, got %v", found)
	}
	warn := exposeOwnerFindings(t, "warn.sysml", docs)
	if len(warn) != 1 || !strings.HasPrefix(warn[0], "warning: `expose` in a view def body") {
		t.Errorf("expose in a view def body must warn once, got %v", warn)
	}
	bad := exposeOwnerFindings(t, "bad.sysml", docs)
	if len(bad) != 1 || !strings.HasPrefix(bad[0], "error: 'expose' declares what a view usage exposes and is only allowed in a view usage body") {
		t.Errorf("expose outside a view must be a syntax error once, got %v", bad)
	}
}
