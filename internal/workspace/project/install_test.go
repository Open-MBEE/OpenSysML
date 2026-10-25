package project

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFixture is a git repository holding Widgets, which uses Gadgets,
// served as an archive by an httptest server.
type installFixture struct {
	repo, gadgetsURL string
	fetcher          *Fetcher
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	gadgets := kpar(t, map[string]string{ProjectFile: projectJSON("Gadgets", "2.0.0"), "Gadgets.sysml": "package Gadgets { part def Gadget; }\n"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gadgets-2.0.0.kpar" {
			_, _ = w.Write(gadgets)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	gadgetsURL := srv.URL + "/gadgets-2.0.0.kpar"
	widgets := func(version string) map[string]string {
		return map[string]string{
			ProjectFile:     projectJSON("Widgets", version, `{"resource": "`+gadgetsURL+`"}`, `{"resource": "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar"}`),
			"Widgets.sysml": "package Widgets { import Gadgets::*; part def Widget { part g : Gadget; } }\n",
		}
	}
	repo := taggedRepo(t, []string{"v1.0.0", "v1.1.0"}, widgets("1.0.0"), widgets("1.1.0"))
	return &installFixture{repo: "file://" + repo, gadgetsURL: gadgetsURL, fetcher: &Fetcher{}}
}

func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"App.sysml": "package App { import Widgets::*; part w : Widget; }\n"})
	if _, err := Init(dir, "Example", "App", ""); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstallerAddInstallsClosureAndLocks(t *testing.T) {
	fx := newInstallFixture(t)
	dir := newProject(t)
	var log bytes.Buffer
	in := &Installer{Dir: dir, Fetcher: fx.fetcher, Log: &log}
	lock, err := in.Add(context.Background(), Usage{Resource: fx.repo, VersionConstraint: "^1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Entries) != 2 {
		t.Fatalf("lock entries %+v: want Widgets and Gadgets, never the standard library", lock.Entries)
	}
	gadgets, widgets := *lock.Entry(fx.gadgetsURL), *lock.Entry(fx.repo)
	if gadgets.Resource != fx.gadgetsURL || gadgets.Path != "lib/acme-gadgets_2.0.0.kpar" || gadgets.Kind != "archive" || gadgets.Digest == "" {
		t.Fatalf("gadgets entry %+v", gadgets)
	}
	if widgets.Resource != fx.repo || widgets.Version != "1.1.0" || widgets.Tag != "v1.1.0" || widgets.Commit == "" || widgets.Path != "lib/acme-widgets_1.1.0" || len(widgets.Usage) != 1 || widgets.Usage[0] != fx.gadgetsURL {
		t.Fatalf("widgets entry %+v", widgets)
	}
	for _, p := range []string{"lib/acme-widgets_1.1.0/.project.json", "lib/acme-widgets_1.1.0/.meta.json", "lib/acme-widgets_1.1.0/Widgets.sysml", "lib/acme-gadgets_2.0.0.kpar", LockFile} {
		if !fileExists(filepath.Join(dir, filepath.FromSlash(p))) {
			t.Errorf("%s was not written", p)
		}
	}
	project, err := ReadInterchange(dir)
	if err != nil || len(project.Usage) != 1 || project.Usage[0].Resource != fx.repo || project.Usage[0].VersionConstraint != "^1.0.0" {
		t.Fatalf(".project.json usages %+v, %v", project.Usage, err)
	}
	written, _ := os.ReadFile(filepath.Join(dir, LockFile))
	again, _ := lock.Marshal()
	if !bytes.Equal(written, again) || !bytes.HasSuffix(written, []byte("}\n")) || !strings.Contains(string(written), `"lockVersion": 1`) {
		t.Fatalf("lock file is not the deterministic marshal:\n%s", written)
	}
	if idx := strings.Index(string(written), `"commit"`); idx < 0 || idx > strings.Index(string(written), `"digest"`) {
		t.Fatalf("lock keys are not sorted:\n%s", written)
	}
	reread, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rereadBytes, _ := reread.Marshal(); !bytes.Equal(rereadBytes, written) {
		t.Fatalf("lock does not round-trip")
	}
	if !strings.Contains(log.String(), "Widgets 1.1.0: installed at lib/acme-widgets_1.1.0") {
		t.Fatalf("log %q", log.String())
	}

	// The loader resolves the usage through the lock, offline.
	found := Discovery{}.Around([]string{filepath.Join(dir, "App.sysml")})
	if len(found.Enclosing) != 1 || len(found.Used) != 2 {
		t.Fatalf("discovery: enclosing %d used %d", len(found.Enclosing), len(found.Used))
	}
	if found.Used[0].Name != "Widgets" || found.Used[1].Name != "Gadgets" {
		t.Fatalf("used %s, %s", found.Used[0].Name, found.Used[1].Name)
	}

	// Sync reproduces the lock without touching it; Relock re-resolves to the same.
	log.Reset()
	if _, err := in.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "already installed") || strings.Contains(log.String(), ": installed at") {
		t.Fatalf("sync log %q", log.String())
	}
	relocked, err := in.Relock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if relockedBytes, _ := relocked.Marshal(); !bytes.Equal(relockedBytes, written) {
		t.Fatalf("relock differs:\n%s", relockedBytes)
	}
	if _, err := in.Add(context.Background(), Usage{Resource: fx.repo}); err == nil || !strings.Contains(err.Error(), "already uses it") {
		t.Fatalf("second add: %v", err)
	}
}

func TestInstallerSyncHonoursLock(t *testing.T) {
	fx := newInstallFixture(t)
	dir := newProject(t)
	in := &Installer{Dir: dir, Fetcher: fx.fetcher}
	if _, err := in.Add(context.Background(), Usage{Resource: fx.repo, VersionConstraint: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	// The constraint widens; the lock still pins 1.0.0 and sync keeps it.
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(projectJSON("App", "0.1.0", `{"resource": "`+fx.repo+`", "versionConstraint": "^1.0.0"}`)), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := in.Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e := lock.Entry(fx.repo); e.Version != "1.0.0" {
		t.Fatalf("sync moved the pin to %s", e.Version)
	}
	// -lock re-resolves to the latest the constraint admits.
	lock, err = in.Relock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e := lock.Entry(fx.repo); e.Version != "1.1.0" || !fileExists(filepath.Join(dir, "lib", "acme-widgets_1.1.0", ProjectFile)) {
		t.Fatalf("relock: %+v", e)
	}
	// A pin outside the constraint is refused, as is a digest that moved.
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(projectJSON("App", "0.1.0", `{"resource": "`+fx.repo+`", "versionConstraint": "~1.0.0"}`)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = in.Sync(context.Background())
	if ie := asInstallError(t, err); ie.Step != "resolve" || !strings.Contains(err.Error(), "no longer satisfies") {
		t.Fatalf("sync with a stale pin: %v", err)
	}
	lock, _ = ReadLock(dir)
	lock.Entry(fx.repo).Constraint = "^1.0.0"
	lock.Entry(fx.repo).Digest = "sha256:0000"
	if err := lock.Write(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ProjectFile), []byte(projectJSON("App", "0.1.0", `{"resource": "`+fx.repo+`", "versionConstraint": "^1.0.0"}`)), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(dir, "lib", "acme-widgets_1.1.0"))
	_, err = in.Sync(context.Background())
	if ie := asInstallError(t, err); ie.Step != "verify" || !strings.Contains(err.Error(), "the lock records sha256:0000") {
		t.Fatalf("sync with a moved digest: %v", err)
	}
}

func asInstallError(t *testing.T, err error) *InstallError {
	t.Helper()
	var ie *InstallError
	if !errors.As(err, &ie) {
		t.Fatalf("%v is not an InstallError", err)
	}
	return ie
}

func TestInstallerLocalUsageIsReadInPlace(t *testing.T) {
	dir := newProject(t)
	local := filepath.Join(filepath.Dir(dir), "widgets-src")
	writeTree(t, local, widgetsTree("1.0.0"))
	in := &Installer{Dir: dir}
	lock, err := in.Add(context.Background(), Usage{Resource: "../widgets-src"})
	if err != nil {
		t.Fatal(err)
	}
	e := lock.Entries[0]
	if e.Kind != "local" || e.Path != "../widgets-src" || e.Digest != "" || fileExists(filepath.Join(dir, "lib")) {
		t.Fatalf("local entry %+v; lib exists %v", e, fileExists(filepath.Join(dir, "lib")))
	}
	found := Discovery{}.Around([]string{filepath.Join(dir, "App.sysml")})
	if len(found.Used) != 1 || found.Used[0].Name != "Widgets" || resolvedKey(found.Used[0].Dir) != resolvedKey(local) {
		t.Fatalf("discovery of the in-place usage: %+v", found.Used)
	}
	if _, err := in.Add(context.Background(), Usage{Resource: "https://www.omg.org/spec/SysML/20250201/Systems-Library.kpar"}); err == nil || !strings.Contains(err.Error(), "never installed") {
		t.Fatalf("adding a standard library IRI: %v", err)
	}
}

func TestDiscoveryLockWinsOverName(t *testing.T) {
	dir := newProject(t)
	// Two installed projects both named Widgets; the lock pins the usage to
	// the one under lib, the hand-copied one in .sysand/lib matches by name.
	writeTree(t, filepath.Join(dir, "lib", "other-widgets_3.0.0"), map[string]string{ProjectFile: `{"name": "Widgets", "publisher": "Other", "version": "3.0.0"}`, "W.sysml": "package Widgets;"})
	writeTree(t, filepath.Join(dir, ".sysand", "lib", "acme-widgets_1.0.0"), widgetsTree("1.0.0"))
	writeTree(t, dir, map[string]string{ProjectFile: projectJSON("App", "0.1.0", `{"resource": "pkg:github/acme/widgets"}`)})
	found := Discovery{}.Around([]string{filepath.Join(dir, "App.sysml")})
	if len(found.Used) != 1 || found.Used[0].Publisher != "Acme" {
		t.Fatalf("without a lock the slug match picks acme-widgets: %+v", found.Used)
	}
	lock := &Lock{Dir: dir, Entries: []LockEntry{{Resource: "pkg:github/acme/widgets", IRI: "pkg:github/acme/widgets", Kind: "host", Name: "Widgets", Publisher: "Other", Version: "3.0.0", Source: "x", Path: "lib/other-widgets_3.0.0"}}}
	if err := lock.Write(); err != nil {
		t.Fatal(err)
	}
	found = Discovery{}.Around([]string{filepath.Join(dir, "App.sysml")})
	if len(found.Used) != 1 || found.Used[0].Publisher != "Other" {
		t.Fatalf("with a lock the pinned path wins: %+v", found.Used)
	}
}

func TestReadSysandHints(t *testing.T) {
	dir := t.TempDir()
	toml := `lock_version = "1"

[[project]]
publisher = "Acme"
name = "Widgets"
version = "1.0.0"
identifiers = [
    "pkg:github/acme/widgets",
    'urn:kpar:acme/widgets',
]
usages = []
sources = [{ remote_git = "https://example.org/acme/widgets.git" }]

[[project]]
name = "Gadgets"
version = "2.0.0"
identifiers = ["pkg:sysand/gadgets"]
`
	if err := os.WriteFile(filepath.Join(dir, SysandLockFile), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	hints, err := ReadSysandHints(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := SysandHints{"pkg:github/acme/widgets": "1.0.0", "urn:kpar:acme/widgets": "1.0.0", "pkg:sysand/gadgets": "2.0.0"}
	if len(hints) != len(want) {
		t.Fatalf("hints %v, want %v", hints, want)
	}
	for k, v := range want {
		if hints[k] != v {
			t.Errorf("hint %s = %q, want %q", k, hints[k], v)
		}
	}
	if none, err := ReadSysandHints(t.TempDir()); err != nil || none != nil {
		t.Fatalf("no sysand lock: %v %v", none, err)
	}
}

func TestSysandHintPinsFirstResolution(t *testing.T) {
	fx := newInstallFixture(t)
	dir := newProject(t)
	writeTree(t, dir, map[string]string{
		ProjectFile:    projectJSON("App", "0.1.0", `{"resource": "`+fx.repo+`", "versionConstraint": "^1.0.0"}`),
		SysandLockFile: "[[project]]\nname = \"Widgets\"\nversion = \"1.0.0\"\nidentifiers = [\"" + fx.repo + "\"]\n",
	})
	lock, err := (&Installer{Dir: dir, Fetcher: fx.fetcher}).Sync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if e := lock.Entry(fx.repo); e.Version != "1.0.0" {
		t.Fatalf("the sysand hint was not honoured: %+v", e)
	}
	if fileExists(filepath.Join(dir, SysandLockFile+".bak")) || !fileExists(filepath.Join(dir, SysandLockFile)) {
		t.Fatal("sysand-lock.toml must be left as it was")
	}
}

func TestInitWritesProjectAndMeta(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"A.sysml": "package A;\n"})
	p, err := Init(dir, "Example", "App", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "App" || p.Publisher != "Example" || p.Version != "0.1.0" {
		t.Fatalf("project %+v", p)
	}
	content, _ := os.ReadFile(filepath.Join(dir, ProjectFile))
	if string(content) != "{\n  \"name\": \"App\",\n  \"publisher\": \"Example\",\n  \"usage\": [],\n  \"version\": \"0.1.0\"\n}\n" {
		t.Fatalf(".project.json:\n%s", content)
	}
	meta, _ := os.ReadFile(filepath.Join(dir, MetaFile))
	parsed, err := ParseMeta(meta)
	if err != nil || parsed.Index["A"] != "A.sysml" {
		t.Fatalf(".meta.json %v: %s", err, meta)
	}
	if _, err := Init(dir, "", "App", ""); err == nil || !strings.Contains(err.Error(), "already holds") {
		t.Fatalf("second init: %v", err)
	}
	if _, err := Init(t.TempDir(), "", "", ""); err == nil || !strings.Contains(err.Error(), "needs a name") {
		t.Fatalf("nameless init: %v", err)
	}
	if _, err := Init(t.TempDir(), "", "X", "one"); err == nil {
		t.Fatal("a non-semver version was accepted")
	}
}
