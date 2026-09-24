package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/filename"
)

// linkedModel declares two documents referencing each other's content, so the
// multi-document mode writes a set whose cross-document links resolve.
const linkedModel = `package Reports {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	ref appendixDoc : Appendix;
	ref mainDoc : MainReport;

	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part overview : Paragraph {
			part back : Ref {
				ref redefines target = mainDoc;
			}
		}
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}

	part def MainReport :> Document {
		attribute redefines title = "Main Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = appendixDoc.tables;
			}
		}
	}
}
`

// TestRenderDocumentsFlag checks -render-documents writes every document of
// the model as a linked Markdown set into the named directory.
func TestRenderDocumentsFlag(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	report, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(report), "[Detail Tables](Reports-Appendix.md#tables)") {
		t.Errorf("report lacks cross-document link:\n%s", report)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(appendix), `<a id="tables"></a>`) {
		t.Errorf("appendix lacks referenced anchor:\n%s", appendix)
	}
	if !strings.Contains(string(appendix), "[Main Report](Reports-MainReport.md)") {
		t.Errorf("appendix lacks root link:\n%s", appendix)
	}

	// A repeated run reproduces the same bytes: the set is deterministic.
	again := filepath.Join(t.TempDir(), "again")
	if got := check(t, binary, linkedModel, "-render-documents", again); got.status != 0 {
		t.Fatalf("repeat exit = %d\n%s", got.status, got.output())
	}
	for _, name := range []string{"Reports-MainReport.md", "Reports-Appendix.md"} {
		first, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		second, err := os.ReadFile(filepath.Join(again, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Errorf("%s differs between runs", name)
		}
	}
}

// TestRenderDocumentsDiagramForm checks -diagram-form applies to every
// document of a set, in Markdown and in HTML.
func TestRenderDocumentsDiagramForm(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.dot.golden.md"))
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "rendered")
	if got := runCommand(t, exec.Command(binary, fixture, "-render-documents", dir, "-diagram-form", "dot")); got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	report, err := os.ReadFile(filepath.Join(dir, "Observatory-MassReport.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if string(report) != string(golden) {
		t.Errorf("set member differs from the DOT golden:\n%s", report)
	}

	site := filepath.Join(t.TempDir(), "site")
	if got := runCommand(t, exec.Command(binary, fixture, "-render-documents", site, "-doc-form", "html", "-diagram-form", "dot")); got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	page, err := os.ReadFile(filepath.Join(site, "Observatory-MassReport.html"))
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	if !strings.Contains(string(page), `<pre class="dot">`) || strings.Contains(string(page), `class="mermaid"`) {
		t.Errorf("page does not write its diagrams as DOT:\n%s", page)
	}

	puml := filepath.Join(t.TempDir(), "puml")
	if got := runCommand(t, exec.Command(binary, fixture, "-render-documents", puml, "-doc-form", "html", "-diagram-form", "plantuml")); got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	page, err = os.ReadFile(filepath.Join(puml, "Observatory-MassReport.html"))
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	if !strings.Contains(string(page), `<pre class="plantuml">@startuml`) || strings.Contains(string(page), `class="mermaid"`) {
		t.Errorf("page does not write its diagrams as PlantUML:\n%s", page)
	}

	wantReport(t, runCommand(t, exec.Command(binary, fixture, "-render-documents", filepath.Join(t.TempDir(), "x"), "-diagram-form", "svg")),
		2, `unknown diagram form "svg"`)
}

// TestRenderDocumentsAcrossFiles checks documents declared in different model
// files render together as one linked set.
func TestRenderDocumentsAcrossFiles(t *testing.T) {
	binary := buildCLI(t)
	models := t.TempDir()
	appendix := filepath.Join(models, "appendix.sysml")
	if err := os.WriteFile(appendix, []byte(`package Appendices {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	part def Appendix :> Document {
		attribute redefines title = "Appendix";
		part tables : Section {
			attribute redefines title = "Detail Tables";
			part body : Paragraph {
				attribute redefines text = "detail";
			}
		}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(models, "report.sysml")
	if err := os.WriteFile(report, []byte(`package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	ref appendixDoc : Appendices::Appendix;

	part def MainReport :> Document {
		attribute redefines title = "Main Report";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = appendixDoc.tables;
			}
		}
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "rendered")
	got := runCommand(t, exec.Command(binary, appendix, report, "-render-documents", dir))
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	main, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.md"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(main), "[Detail Tables](Appendices-Appendix.md#tables)") {
		t.Errorf("report lacks cross-file document link:\n%s", main)
	}
	side, err := os.ReadFile(filepath.Join(dir, "Appendices-Appendix.md"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(side), `<a id="tables"></a>`) {
		t.Errorf("appendix lacks referenced anchor:\n%s", side)
	}
}

// TestRenderDocumentsAllOrNothing checks a set that cannot be written in full
// leaves the directory as it was, with no staged leftovers.
func TestRenderDocumentsAllOrNothing(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(filepath.Join(dir, "Reports-MainReport.md"), 0o750); err != nil {
		t.Fatal(err)
	}
	previous := "previous appendix\n"
	if err := os.WriteFile(filepath.Join(dir, "Reports-Appendix.md"), []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	bystander := "not ours\n"
	if err := os.WriteFile(filepath.Join(dir, "Reports-Appendix.md.staged"), []byte(bystander), 0o644); err != nil {
		t.Fatal(err)
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "it is a directory") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(appendix) != previous {
		t.Errorf("failed set replaced the existing appendix:\n%s", appendix)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.md.staged"))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != bystander {
		t.Errorf("failed set touched an unrelated file:\n%s", kept)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".sysml-") {
			t.Errorf("staged leftover %s", entry.Name())
		}
	}
}

// TestRenderDocumentsWritesThroughSymlink checks a destination that is a
// symlink stays a symlink, with the rendered document written to its target.
func TestRenderDocumentsWritesThroughSymlink(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "linked-appendix.md")
	if err := os.WriteFile(elsewhere, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "Reports-Appendix.md")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the destination symlink was replaced by a file")
	}
	target, err := os.ReadFile(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(target), `<a id="tables"></a>`) {
		t.Errorf("the link's target lacks the rendered appendix:\n%s", target)
	}
}

// TestRenderDocumentsRejectsAliasedTargets checks two destinations resolving
// to one file fail the set before anything is written, so the later document
// cannot silently replace the earlier one.
func TestRenderDocumentsRejectsAliasedTargets(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(t.TempDir(), "shared.md")
	previous := "previous\n"
	if err := os.WriteFile(shared, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Reports-MainReport.md", "Reports-Appendix.md"} {
		if err := os.Symlink(shared, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "both resolve to") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
	kept, err := os.ReadFile(shared)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != previous {
		t.Errorf("a rejected set wrote the shared file:\n%s", kept)
	}
}

// TestRenderDocumentsRejectsDanglingAliasedTargets checks two dangling links
// reaching one absent file through aliased directories are rejected, so the
// later document cannot silently replace the first.
func TestRenderDocumentsRejectsDanglingAliasedTargets(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	realDir := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(realDir, 0o750); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	pairs := map[string]string{
		"Reports-MainReport.md": filepath.Join(realDir, "shared.md"),
		"Reports-Appendix.md":   filepath.Join(alias, "shared.md"),
	}
	for name, target := range pairs {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "both resolve to") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(realDir, "shared.md")); !os.IsNotExist(err) {
		t.Errorf("a rejected set wrote the shared file: %v", err)
	}
}

// TestRenderDocumentsRejectsCaseAliasedTargets checks two dangling links
// naming case variants of one absent file are rejected on a case-insensitive
// filesystem, where they would land on one entry.
func TestRenderDocumentsRejectsCaseAliasedTargets(t *testing.T) {
	binary := buildCLI(t)
	outside := t.TempDir()
	if !foldsCase(outside) {
		t.Skip("the filesystem distinguishes names by case")
	}
	dir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	pairs := map[string]string{
		"Reports-MainReport.md": filepath.Join(outside, "shared.md"),
		"Reports-Appendix.md":   filepath.Join(outside, "SHARED.md"),
	}
	for name, target := range pairs {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	got := check(t, binary, linkedModel, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "both resolve to") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
}

// TestRestoreBackupRevivesRemovedDestination checks a rollback moves a
// hard-linked backup back when a failed replacement removed the destination,
// and only sheds it while the destination still stands.
func TestRestoreBackupRevivesRemovedDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Reports-Appendix.md")
	backup := filepath.Join(dir, "backup")
	if err := os.WriteFile(target, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	restoreBackup(target, backup, false)
	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("the destination stayed missing: %v", err)
	}
	if string(restored) != "previous\n" {
		t.Errorf("target = %q", restored)
	}

	if err := os.Link(target, backup); err != nil {
		t.Fatal(err)
	}
	restoreBackup(target, backup, false)
	if _, err := os.ReadFile(target); err != nil {
		t.Fatalf("an intact destination was disturbed: %v", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Errorf("the backup remains beside an intact destination: %v", err)
	}
}

// TestReplaceFileReplacesExistingTarget checks a rollback restore lands over
// an existing committed file.
func TestReplaceFileReplacesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "Reports-Appendix.md")
	if err := os.WriteFile(backup, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := source.ReplaceFile(backup, target); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != "previous\n" {
		t.Errorf("target = %q", restored)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Errorf("the backup remains after its restore: %v", err)
	}
}

// TestRenderDocumentsCaseCollidingNames checks two documents whose names meet
// letter case aside are each written under a tagged name, so a set survives a
// filesystem that folds case, and the links between them point at the tags.
func TestRenderDocumentsCaseCollidingNames(t *testing.T) {
	binary := buildCLI(t)
	model := `package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	ref shouting : WEEKLY;

	part def Weekly :> Document {
		attribute redefines title = "Weekly";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = shouting;
			}
		}
	}
	part def WEEKLY :> Document {
		attribute redefines title = "WEEKLY";
	}
}
`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := check(t, binary, model, "-render-documents", dir)
	wantReport(t, got, 0, "Reports-Weekly~", "Reports-WEEKLY~")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var weekly, shouting string
	for _, entry := range entries {
		switch {
		case strings.HasPrefix(entry.Name(), "Reports-Weekly~"):
			weekly = entry.Name()
		case strings.HasPrefix(entry.Name(), "Reports-WEEKLY~"):
			shouting = entry.Name()
		default:
			t.Errorf("unexpected file %s", entry.Name())
		}
	}
	if weekly == "" || shouting == "" || filename.CaseFolded(weekly) == filename.CaseFolded(shouting) {
		t.Fatalf("files = %q and %q, want two names distinct letter case aside", weekly, shouting)
	}
	page, err := os.ReadFile(filepath.Join(dir, weekly))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "]("+shouting+")") {
		t.Errorf("the link does not point at the tagged file %s:\n%s", shouting, page)
	}
}

// TestRenderDocumentsSameShortName checks documents of one short name in
// different packages each get their own file, and a link to one of them
// lands on that one.
func TestRenderDocumentsSameShortName(t *testing.T) {
	binary := buildCLI(t)
	model := `package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	package Alpha {
		part def Summary :> Document {
			attribute redefines title = "Alpha Summary";
		}
	}
	package Beta {
		part def Summary :> Document {
			attribute redefines title = "Beta Summary";
		}
	}
	ref beta : Beta::Summary;

	part def Index :> Document {
		attribute redefines title = "Index";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = beta;
			}
		}
	}
}
`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := check(t, binary, model, "-render-documents", dir)
	wantReport(t, got, 0, "Reports-Alpha-Summary.md", "Reports-Beta-Summary.md", "Reports-Index.md")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("wrote %d files, want 3", len(entries))
	}
	for file, title := range map[string]string{
		"Reports-Alpha-Summary.md": "# Alpha Summary",
		"Reports-Beta-Summary.md":  "# Beta Summary",
	} {
		page, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(page), title) {
			t.Errorf("%s lacks %q:\n%s", file, title, page)
		}
	}
	index, err := os.ReadFile(filepath.Join(dir, "Reports-Index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "](Reports-Beta-Summary.md)") {
		t.Errorf("the link does not name the Beta document's file:\n%s", index)
	}
}

// partialModel declares three documents, one of which fails to evaluate: its
// table reads a [1] attribute the row leaves unbound. The others link to it.
const partialModel = `package Reports {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Scenario {
		attribute duration : Real;
	}
	part campaign {
		part idle : Scenario;
	}

	calc def Timings :> Query {
		in root : Element = campaign;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name"),
			columns = (Column(name = "duration", expression = Scenario::duration))
		)
	}

	ref brokenDoc : Broken;

	part def Broken :> Document {
		attribute redefines title = "Broken Timings";
		part timings : Table {
			calc rows : Timings;
		}
	}

	part def First :> Document {
		attribute redefines title = "First";
		part intro : Paragraph {
			part see : Ref {
				ref redefines target = brokenDoc;
			}
		}
	}

	part def Second :> Document {
		attribute redefines title = "Second";
		part intro : Paragraph {
			attribute redefines text = "second";
		}
	}
}
`

// TestRenderDocumentsPartialSet checks a set with one document that cannot be
// rendered still writes the others, writes a page stating the error where the
// failed document's links land, names the failure on stderr, and exits 3.
func TestRenderDocumentsPartialSet(t *testing.T) {
	binary := buildCLI(t)
	for _, form := range []struct {
		name, ext string
		args      []string
		link      string
	}{
		{"markdown", ".md", nil, "](Reports-Broken.md)"},
		{"html", ".html", []string{"-doc-form", "html"}, `href="Reports-Broken.html"`},
	} {
		t.Run(form.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "rendered")
			got := check(t, binary, partialModel, append([]string{"-render-documents", dir}, form.args...)...)
			wantReport(t, got, 3,
				"Reports-First"+form.ext+" (",
				"Reports-Second"+form.ext+" (",
				"Reports-Broken"+form.ext+" (",
				"a page stating why the document could not be rendered",
				"sysml: document Reports::Broken could not be rendered: ",
				"column duration", "duration")
			if strings.Count(got.stderr, "could not be rendered:") != 1 {
				t.Errorf("want the one failure named once:\n%s", got.stderr)
			}
			second, err := os.ReadFile(filepath.Join(dir, "Reports-Second"+form.ext))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(second), "second") {
				t.Errorf("the rendered document lacks its text:\n%s", second)
			}
			first, err := os.ReadFile(filepath.Join(dir, "Reports-First"+form.ext))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(first), form.link) {
				t.Errorf("the link to the failed document is not %s:\n%s", form.link, first)
			}
			broken, err := os.ReadFile(filepath.Join(dir, "Reports-Broken"+form.ext))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Broken Timings", "This document could not be rendered.", "duration"} {
				if !strings.Contains(string(broken), want) {
					t.Errorf("the failed document's page lacks %q:\n%s", want, broken)
				}
			}
		})
	}
}

// TestRenderDocumentAmbiguousName checks a short name held by documents in
// several packages is refused with every candidate's qualified name.
func TestRenderDocumentAmbiguousName(t *testing.T) {
	binary := buildCLI(t)
	model := `package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	package Alpha {
		part def Summary :> Document {
			attribute redefines title = "Alpha Summary";
		}
	}
	package Beta {
		part def Summary :> Document {
			attribute redefines title = "Beta Summary";
		}
	}
}
`
	got := check(t, binary, model, "-render-document", "Summary")
	wantReport(t, got, 2, `symbol "Summary" is ambiguous: Reports::Alpha::Summary, Reports::Beta::Summary`, "use a qualified name")
	if got.stdout != "" {
		t.Errorf("stdout = %q, want nothing", got.stdout)
	}
	got = check(t, binary, model, "-render-document", "Reports::Beta::Summary")
	wantReport(t, got, 0)
	if !strings.Contains(got.stdout, "# Beta Summary") {
		t.Errorf("the qualified name did not render the Beta document:\n%s", got.stdout)
	}
}

// TestBackUpKeepsDestinationInPlace checks a backup leaves the destination
// itself untouched, so an interrupted commit never hides a document.
func TestBackUpKeepsDestinationInPlace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Reports-Appendix.md")
	if err := os.WriteFile(target, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, movedAside, err := backUp(target)
	if err != nil {
		t.Fatal(err)
	}
	if movedAside {
		t.Fatal("a hard-link filesystem fell back to a set-aside rename")
	}
	held, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("the backed-up destination is gone: %v", err)
	}
	if string(held) != "previous\n" {
		t.Errorf("destination = %q", held)
	}
	kept, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "previous\n" {
		t.Errorf("backup = %q", kept)
	}
}

// TestRenderDocumentEmitsIncomingAnchors checks a document rendered on its own
// carries the anchors other documents of the workspace link into it.
func TestRenderDocumentEmitsIncomingAnchors(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, linkedModel, "-render-document", "Reports::Appendix")
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	if !strings.Contains(got.stdout, `<a id="tables"></a>`) {
		t.Errorf("separately rendered appendix lacks the incoming anchor:\n%s", got.stdout)
	}
}

// TestRenderDocumentsNoDocuments checks a model without documents is a
// documented failure rather than an empty directory.
func TestRenderDocumentsNoDocuments(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, "package Empty {}\n", "-render-documents", filepath.Join(t.TempDir(), "rendered"))
	if got.status != 2 || !strings.Contains(got.stderr, "declares no documents") {
		t.Errorf("exit = %d stderr = %q", got.status, got.stderr)
	}
}

// TestRenderDocumentsFlagConflicts checks -render-documents refuses runs
// asking for something else too.
func TestRenderDocumentsFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-render-document", "Reports::MainReport"),
		2, "ask for one per run")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-render", "SomeView"),
		2, "ask for one per run")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-o", "out.md"),
		2, "cannot be combined with -output")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-query", "parts"),
		2, "-query")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", "rendered", "-constraint", "C"),
		2, "check it in its own run")
}

// TestRenderDocumentsHTML checks -doc-form html writes the set as linked HTML
// pages sharing one stylesheet file, so a reader edits the styling in one
// place.
func TestRenderDocumentsHTML(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "site")

	got := check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "html")
	wantReport(t, got, 0, "sysml-document.css (css,", "Reports-MainReport.html (html,")
	report, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="sysml-document.css">`,
		`href="Reports-Appendix.html#tables"`,
	} {
		if !strings.Contains(string(report), want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	if strings.Contains(string(report), "@layer opensysml") {
		t.Errorf("a set links the shared sheet rather than inlining it:\n%s", report)
	}
	appendix, err := os.ReadFile(filepath.Join(dir, "Reports-Appendix.html"))
	if err != nil {
		t.Fatalf("read appendix: %v", err)
	}
	if !strings.Contains(string(appendix), `id="tables"`) {
		t.Errorf("appendix lacks the referenced identifier:\n%s", appendix)
	}
	stylesheet, err := os.ReadFile(filepath.Join(dir, "sysml-document.css"))
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	if !strings.Contains(string(stylesheet), "@layer opensysml;") {
		t.Errorf("shared stylesheet is not the default sheet:\n%s", stylesheet)
	}
	// The Markdown set is untouched by the HTML form.
	if _, err := os.Stat(filepath.Join(dir, "Reports-MainReport.md")); err == nil {
		t.Error("the HTML set wrote Markdown files too")
	}
}

// TestRenderDocumentsHTMLStylesheets checks a custom sheet of an HTML set is
// a file beside the pages that every page links, rather than bytes repeated
// in each page, and that a URL stays a link in the order it was given.
func TestRenderDocumentsHTMLStylesheets(t *testing.T) {
	binary := buildCLI(t)
	work := t.TempDir()
	dir := filepath.Join(work, "site")
	theme := filepath.Join(work, "theme.css")
	if err := os.WriteFile(theme, []byte(".sysml-document { --sysml-text: rebeccapurple; }\n"), 0o600); err != nil {
		t.Fatalf("write theme: %v", err)
	}

	got := check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "html",
		"-html-css", theme, "-html-css", "https://example.test/site.css")
	wantReport(t, got, 0, "theme.css (css,")
	links := []string{
		`<link rel="stylesheet" href="sysml-document.css">`,
		`<link rel="stylesheet" href="theme.css">`,
		`<link rel="stylesheet" href="https://example.test/site.css">`,
	}
	for _, name := range []string{"Reports-MainReport.html", "Reports-Appendix.html"} {
		page, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		at := -1
		for _, link := range links {
			next := strings.Index(string(page), link)
			if next < 0 {
				t.Fatalf("%s lacks %q:\n%s", name, link, page)
			}
			if next < at {
				t.Errorf("%s links the stylesheets out of order:\n%s", name, page)
			}
			at = next
		}
		if strings.Contains(string(page), "rebeccapurple") {
			t.Errorf("%s inlines the custom sheet rather than linking it:\n%s", name, page)
		}
	}
	written, err := os.ReadFile(filepath.Join(dir, "theme.css"))
	if err != nil {
		t.Fatalf("read written theme: %v", err)
	}
	if !strings.Contains(string(written), "rebeccapurple") {
		t.Errorf("the set's theme.css is not the sheet asked for:\n%s", written)
	}
}

// TestRenderDocumentsHTMLStylesheetNames checks a stylesheet whose name holds
// URL delimiters or spaces is written and linked under one escaped name, so
// the link a page carries names the file beside it.
func TestRenderDocumentsHTMLStylesheetNames(t *testing.T) {
	binary := buildCLI(t)
	work := t.TempDir()
	dir := filepath.Join(work, "site")
	awkward := filepath.Join(work, "my theme#1?v=2%.css")
	if err := os.WriteFile(awkward, []byte(".sysml-document { --sysml-text: rebeccapurple; }\n"), 0o600); err != nil {
		t.Fatalf("write theme: %v", err)
	}

	wantReport(t, check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "html",
		"-html-css", awkward), 0)
	name := "my.20theme.231.3Fv.3D2.25.css"
	page, err := os.ReadFile(filepath.Join(dir, "Reports-MainReport.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if want := `<link rel="stylesheet" href="` + name + `">`; !strings.Contains(string(page), want) {
		t.Errorf("report lacks %q:\n%s", want, page)
	}
	written, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read written theme: %v", err)
	}
	if !strings.Contains(string(written), "rebeccapurple") {
		t.Errorf("the set's %s is not the sheet asked for:\n%s", name, written)
	}
}

// TestRenderDocumentsHTMLFlagConflicts checks the set refuses forms and
// options it cannot write.
func TestRenderDocumentsHTMLFlagConflicts(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "site")

	wantReport(t, check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "pdf"),
		2, "render one document at a time with -render-document -doc-form pdf")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", dir, "-html-fragment", "-doc-form", "html"),
		2, "-html-fragment writes one document element to embed")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", dir, "-html-css", "theme.css"),
		2, "ask for it with -doc-form html")
	wantReport(t, check(t, binary, linkedModel, "-render-documents", dir, "-doc-form", "latex"),
		2, "unknown document form")
}
