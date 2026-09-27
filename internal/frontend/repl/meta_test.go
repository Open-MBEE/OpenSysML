package repl

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

func TestMetaHelpAndList(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	out, quit, err := s.runMeta("%help")
	if err != nil || quit {
		t.Fatalf("%%help: err=%v quit=%v", err, quit)
	}
	if !strings.Contains(strings.Join(out, "\n"), "%load") {
		t.Errorf("%%help should list commands: %v", out)
	}
	out, _, _ = s.runMeta("%list")
	if !strings.Contains(strings.Join(out, "\n"), "package P") {
		t.Errorf("%%list should show declarations: %v", out)
	}
}

// Every command sits under a heading, aliases are folded into the command
// they spell, and no line, a wide signature included, outruns the width.
func TestHelpTextIsGroupedAndWrapped(t *testing.T) {
	lines := helpText()
	help := strings.Join(lines, "\n")
	for _, want := range []string{
		"\nSession:\n", "\nSettings:\n", "\nAnalysis engines:\n", "\nChecking every schedule:\n",
		"\nLibrary discovery:\n", "\nState machine debugging:\n",
		"  %quit                     exit the REPL (also %exit)\n",
		"  %check-witness [<dir>|off]\n                            show or set",
		"  %check-bounds [depth=<n>] [states=<n>] [unroll=<n>] [timeout=<duration>]\n      | off\n",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "\n  %exit") {
		t.Errorf("help lists the alias %%exit on its own:\n%s", help)
	}
	for _, c := range metaCommandTable {
		if c.group == "" {
			t.Errorf("%s has no help heading", c.name)
		}
	}
	for _, line := range lines {
		if len(line) > helpWidth {
			t.Errorf("help line is %d wide: %q", len(line), line)
		}
	}
}

func TestMetaClear(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	if _, _, err := s.runMeta("%clear"); err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Errorf("%%clear should empty session, got %v", s.List())
	}
}

func TestMetaLoad(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.sysml")
	if err := os.WriteFile(f, []byte("package Loaded { }"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if _, _, err := s.runMeta("%load " + f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.List(), "\n"), "Loaded") {
		t.Errorf("%%load should submit file contents: %v", s.List())
	}
}

func TestIsMeta(t *testing.T) {
	if !isMeta("%help") || isMeta("package P") || isMeta("") {
		t.Fatal("isMeta classification wrong")
	}
}

func TestQueryReportsPropertiesTheQueryCanAskFor(t *testing.T) {
	s := NewSession()
	if result := s.Submit("package Demo { part def Wheel; part wheel : Wheel; }"); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	// A reported property must be one the next query can be written with, so it
	// is reported under the name it was asked for.
	for _, selected := range []string{"rdf:type", "sysml:name", "sysml:qualifiedName"} {
		lines, err := s.Query(`oslc.where=rdf:type="PartUsage"&oslc.select=` + url.QueryEscape(selected))
		if err != nil {
			t.Fatalf("%s: %v", selected, err)
		}
		if len(lines) != 1 {
			t.Fatalf("%s reported %v, want one element", selected, lines)
		}
		field, _, ok := strings.Cut(strings.TrimPrefix(lines[0], "Demo::wheel  PartUsage  "), "=")
		if !ok || field != selected {
			t.Fatalf("%s reported %q, want it to name %q", selected, lines[0], selected)
		}
	}

	// An oslc.prefix binding renames the property in the answer too.
	lines, err := s.Query(`oslc.prefix=` + url.QueryEscape("s=<https://www.omg.org/spec/SysML#>") +
		`&oslc.where=rdf:type%3D%22PartUsage%22&oslc.select=s:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  s:name=wheel" {
		t.Fatalf("aliased select = %v", lines)
	}
}

func TestQueryAndMetaQuery(t *testing.T) {
	fresh := NewSession()
	if _, err := fresh.Query(`oslc.where=rdf:type="PartUsage"`); err == nil {
		t.Fatal("fresh Session.Query unexpectedly succeeded")
	} else if queryErr, ok := err.(*corequery.Error); !ok || queryErr.Kind != corequery.ErrNoModel {
		t.Fatalf("fresh Session.Query error = %#v, want ErrNoModel", err)
	}
	freshLines, _, err := fresh.runMeta(`%query oslc.where=rdf:type="PartUsage"`)
	if err != nil || len(freshLines) != 1 || !strings.HasPrefix(freshLines[0], "error: no model loaded") {
		t.Fatalf("fresh %%query = lines %v, err %v", freshLines, err)
	}
	broken := NewSession()
	if result := broken.Submit("package Broken {"); len(result.Diagnostics) == 0 {
		t.Fatal("broken model unexpectedly had no diagnostics")
	}
	if _, err := broken.Query(`oslc.where=rdf:type="PartUsage"`); err == nil {
		t.Fatal("query against broken model unexpectedly succeeded")
	} else if queryErr, ok := err.(*corequery.Error); !ok || queryErr.Kind != corequery.ErrNoModel {
		t.Fatalf("broken Session.Query error = %#v, want ErrNoModel", err)
	}
	s := NewSession()
	if result := s.Submit("package Demo { part def Wheel; part wheel : Wheel; }"); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	lines, err := s.Query(`oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  sysml:name=wheel" {
		t.Fatalf("Session.Query = %v", lines)
	}
	lines, err = s.Query(`oslc.where=sysml:name="spare"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("no-match Session.Query = %v", lines)
	}
	lines, _, err = s.runMeta(`%query oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  sysml:name=wheel" {
		t.Fatalf("%%query = %v", lines)
	}
	lines, _, err = s.runMeta(`  %query oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Demo::wheel  PartUsage  sysml:name=wheel" {
		t.Fatalf("indented %%query = %v", lines)
	}
	lines, _, err = s.runMeta(`%query oslc.where=rdf:type=`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "error: ") {
		t.Fatalf("malformed %%query = %v", lines)
	}
	// A query that matched nothing must not be silent, or a caller cannot tell
	// it apart from one that failed to run.
	lines, _, err = s.runMeta(`%query oslc.where=sysml:name="spare"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "no elements matched" {
		t.Fatalf("no-match %%query = %v", lines)
	}
}

// A body-expression parameter and a loop- or branch-body declaration exist only
// inside their body, so the scope-tree search backing %eval must not surface them.
func TestLookupInScopeTreeSkipsBodyLocalNames(t *testing.T) {
	s := NewSession()
	s.Submit(`package P {
		action def Sample {
			in attribute samples;
			assert constraint { samples->forAll { in bodyParam; bodyParam > 0 } }
			loop action charging { } until true;
			if true { action thenLocal; } else { action elseLocal; }
		}
	}`)
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		t.Fatal("no document scope")
	}
	if sym, _ := doc.Scope.LookupLocal("P"); sym == nil {
		t.Fatal("package P not in the document scope")
	}
	for _, name := range []string{"bodyParam", "charging", "thenLocal", "elseLocal"} {
		if syms := s.nameTable().lookup(name); len(syms) > 0 {
			t.Errorf("%s is body-local and must not be found in the scope tree", name)
		}
	}
	if syms := s.nameTable().lookup("samples"); len(syms) == 0 {
		t.Error("samples is a member of Sample and must still be found")
	}
}

func TestQueryIdentifiesUnnamedSatisfyAndItsEnds(t *testing.T) {
	s := NewSession()
	result := s.Submit(`package Demo {
		part def Toaster;
		requirement def EnergyReq { subject t : Toaster; }
		requirement r : EnergyReq { subject t : Toaster; }
		part t : Toaster;
		assert satisfy r by t;
	}`)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	query := `oslc.where=rdf:type="SatisfyRequirementUsage"&oslc.select=sysml:satisfiedRequirement,sysml:satisfyingFeature`
	want := "Demo::@4  SatisfyRequirementUsage  sysml:satisfiedRequirement=Demo::r  sysml:satisfyingFeature=Demo::t"
	lines, err := s.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("Session.Query = %v, want [%s]", lines, want)
	}
	lines, _, err = s.runMeta("%query " + query)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("%%query = %v, want [%s]", lines, want)
	}
}

func TestReplQueryPositionalScopeAndDeclarationOrder(t *testing.T) {
	s := NewSession()
	result := s.Submit(`package Demo { part def Toaster; requirement def EnergyReq { subject t : Toaster; } requirement r : EnergyReq { subject t : Toaster; } part t : Toaster; assert satisfy r by t; }`)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	model := &replQueryModel{session: s, index: s.browseIndex()}
	candidates, err := model.Candidates([]string{"Demo::@4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Kind != symbols.SymbolSatisfyRequirementUsage ||
		model.Identity(candidates[0]) != "Demo::@4" {
		t.Fatalf("positional scope candidates = %v, want the Demo::@4 satisfy", candidates)
	}
	if _, err := model.Candidates([]string{"Demo::@99"}); err == nil ||
		!strings.Contains(err.Error(), `query scope names an element the model does not have: "Demo::@99"`) {
		t.Fatalf("unknown positional scope error = %v", err)
	}

	ordered := NewSession()
	result = ordered.Submit(`package Demo { connect a to b; part a; part b; }`)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Submit interleaved model diagnostics = %v", result.Diagnostics)
	}
	model = &replQueryModel{session: ordered, index: ordered.browseIndex()}
	candidates, err = model.Candidates(nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(candidates))
	for i, sym := range candidates {
		ids[i] = model.Identity(sym)
	}
	want := []string{"Demo", "Demo::@0", "Demo::a", "Demo::b"}
	if !slices.Equal(ids, want) {
		t.Fatalf("unscoped query identities = %v, want %v", ids, want)
	}

	scoped := NewSession()
	result = scoped.Submit(`package Demo { part outer { connect a to b; part a; part b; } }`)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Submit scoped interleaved model diagnostics = %v", result.Diagnostics)
	}
	model = &replQueryModel{session: scoped, index: scoped.browseIndex()}
	candidates, err = model.Candidates([]string{"Demo::outer"})
	if err != nil {
		t.Fatal(err)
	}
	ids = make([]string, len(candidates))
	for i, sym := range candidates {
		ids[i] = model.Identity(sym)
	}
	want = []string{"Demo::outer", "Demo::outer::@0", "Demo::outer::a", "Demo::outer::b"}
	if !slices.Equal(ids, want) {
		t.Fatalf("scoped query identities = %v, want %v", ids, want)
	}
}

func TestReplQueryIdentifiesNamedChildrenOfUnnamedOwners(t *testing.T) {
	s := NewSession()
	result := s.Submit(`package Demo { part { part wheel; } }`)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	model := &replQueryModel{session: s, index: s.browseIndex()}
	lines, err := s.Query(`oslc.where=sysml:name="wheel"&oslc.select=sysml:owner`)
	if err != nil {
		t.Fatal(err)
	}
	want := "Demo::@0::wheel  PartUsage  sysml:owner=Demo::@0"
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("Session.Query = %v, want [%s]", lines, want)
	}
	candidates, err := model.Candidates([]string{"Demo::@0"})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(candidates))
	for i, sym := range candidates {
		ids[i] = model.Identity(sym)
	}
	if want := []string{"Demo::@0", "Demo::@0::wheel"}; !slices.Equal(ids, want) {
		t.Fatalf("unnamed-owner scope identities = %v, want %v", ids, want)
	}
	candidates, err = model.Candidates([]string{"Demo::@0::wheel"})
	if err != nil || len(candidates) != 1 || model.Identity(candidates[0]) != "Demo::@0::wheel" {
		t.Fatalf("named-child scope candidates = %v, err %v", candidates, err)
	}
}

func TestReplQueryAssignsPositionalIdentityToKerMLDeclarations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.kerml")
	content := []byte("type C specializes Base::Anything; feature : C;")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	report, err := s.LoadPathsReport([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors {
		t.Fatalf("KerML load reported errors: %v", report.Found)
	}
	model := &replQueryModel{session: s, index: s.browseIndex()}
	candidates, err := model.Candidates(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, sym := range candidates {
		if sym.Name == "" && export.IsPositionalIdentity(model.Identity(sym)) {
			return
		}
	}
	t.Fatalf("KerML query candidates have no positional identity for the unnamed feature: %v", candidates)
}
