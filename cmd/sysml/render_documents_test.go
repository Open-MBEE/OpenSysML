package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/fsutil"
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
	source := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "Reports-Appendix.md")
	if err := os.WriteFile(source, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.Replace(source, target); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != "previous\n" {
		t.Errorf("target = %q", restored)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Errorf("the backup remains after its restore: %v", err)
	}
}

// TestRenderDocumentsCaseCollidingNames checks documents whose file names
// differ only by letter case are rejected before anything is written, so a
// set renders the same on case-sensitive and case-insensitive filesystems.
func TestRenderDocumentsCaseCollidingNames(t *testing.T) {
	binary := buildCLI(t)
	model := `package Reports {
	private import DocumentQueries::*;
	private import ScalarValues::*;

	part def Weekly :> Document {
		attribute redefines title = "Weekly";
	}
	part def WEEKLY :> Document {
		attribute redefines title = "WEEKLY";
	}
}
`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := check(t, binary, model, "-render-documents", dir)
	if got.status != 2 || !strings.Contains(got.stderr, "differ only by letter case") {
		t.Fatalf("exit = %d stderr = %q", got.status, got.stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("a rejected set created the directory: %v", err)
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
