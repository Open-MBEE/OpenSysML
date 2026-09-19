package repl

import (
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// codeReservedName is the parser's code for a declaration name spelled with a
// reserved keyword, which KerML does not reserve for these contextual words.
const codeReservedName = "reserved-keyword-name"

// kermlContextualSrc uses SysML-only contextual keywords as ordinary KerML
// names, in declarations and in a qualified reference.
const kermlContextualSrc = `package F51K {
	feature at = 1;
	feature while = 2;
	feature merge = 3;
	feature decide = 4;
	feature total = at + while + merge + decide;
}
`

// A snippet submitted from a .kerml file is parsed as KerML, so the contextual
// keywords are plain names there: no warning, no error.
func TestF51KerMLFileSnippetAcceptsContextualKeywords(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{{Name: "f51.kerml", Text: kermlContextualSrc}})
	if hasCode(res.Diagnostics, codeReservedName) {
		t.Fatalf("KerML does not reserve these words, got %v", codesOf(res.Diagnostics))
	}
	if s.HasErrors() {
		t.Fatalf("the snippet is legal KerML, got %v", s.DiagnosticLines())
	}
}

// The same text submitted from a .sysml file keeps the SysML reservation: the
// names warn and the qualified use of one fails to parse.
func TestF51SysMLFileSnippetKeepsContextualKeywordsReserved(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{{Name: "f51.sysml", Text: kermlContextualSrc}})
	if !hasCode(res.Diagnostics, codeReservedName) {
		t.Fatalf("want a %s finding, got %v", codeReservedName, codesOf(res.Diagnostics))
	}
}

// A snippet typed at the prompt has no file, so it keeps the default kind where
// the contextual keywords stay reserved.
func TestF51PromptSnippetKeepsContextualKeywordsReserved(t *testing.T) {
	s := NewSession()
	res := s.Submit("package P { feature at = 1; }\n")
	if !hasCode(res.Diagnostics, codeReservedName) {
		t.Fatalf("want a %s finding, got %v", codeReservedName, codesOf(res.Diagnostics))
	}
}

// One session mixes both languages: the .kerml snippet is silent about KerML
// notation while the .sysml snippet still warns, and a prompt snippet resolves
// names the .kerml snippet declared.
func TestF51MixedSessionKeepsPerSnippetKinds(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{
		{Name: "k.kerml", Text: "namespace K { feature f = 1; }\n"},
		{Name: "s.sysml", Text: "namespace S { part def D; }\n"},
	})
	var kerml, sysml bool
	for _, d := range res.Diagnostics {
		if d.Code != passes.CodeKerMLNotation {
			continue
		}
		if d.Span.Offset < len("namespace K { feature f = 1; }\n") {
			kerml = true
		} else {
			sysml = true
		}
	}
	if kerml {
		t.Fatalf("`namespace` is legal in the .kerml snippet, got %v", codesOf(res.Diagnostics))
	}
	if !sysml {
		t.Fatalf("the .sysml snippet must keep the %s warning, got %v", passes.CodeKerMLNotation, codesOf(res.Diagnostics))
	}

	res = s.Submit("package Q { attribute r = K::f; }\n")
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("the prompt must see the .kerml snippet's names: %v", d)
		}
	}
}

// %load submits each file under its own name, so a loaded .kerml file behaves
// as the file does under -validate.
func TestF51LoadedKerMLFileAcceptsContextualKeywords(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "f51.kerml"), kermlContextualSrc)

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	diags := s.Diagnostics()
	if hasCode(diags, codeReservedName) {
		t.Fatalf("KerML does not reserve these words, got %v", codesOf(diags))
	}
	if s.HasErrors() {
		t.Fatalf("the file is legal KerML, got %v", s.DiagnosticLines())
	}
}

// A .kerml file may declare members at its top level with no enclosing
// namespace; a compound prompt expression must still reach them.
func TestF51PromptCompoundExprReachesTopLevelKerMLNames(t *testing.T) {
	s := NewSession()
	s.SubmitFiles([]SourceFile{{Name: "top.kerml", Text: "feature f = 5;\n"}})
	got := run(t, s, "%eval f * 2")
	wants(t, got, "= 10")
	rejects(t, got, "unresolved reference")
}

// A malformed .kerml snippet reports diagnostics without panicking, and the
// session stays usable.
func TestF51MalformedKerMLSnippetIsRobust(t *testing.T) {
	s := NewSession()
	s.SubmitFiles([]SourceFile{{Name: "bad.kerml", Text: "namespace Broken { feature f =\n"}})
	res := s.Submit("package After { attribute a = 1; }\n")
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError && d.Span.Offset > len("namespace Broken { feature f =\n") {
			t.Fatalf("the session must stay usable after a malformed snippet: %v", d)
		}
	}
}
