package kpar_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// gitRepo is a local git repository whose tags each hold a tree; the
// returned URL is what a usage names it by.
func gitRepo(t *testing.T, tags map[string]map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	work := filepath.Join(t.TempDir(), "widgets")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	names := make([]string, 0, len(tags))
	for tag := range tags {
		names = append(names, tag)
	}
	for _, tag := range sortedStrings(names) {
		entries, _ := os.ReadDir(work)
		for _, e := range entries {
			if e.Name() != ".git" {
				_ = os.RemoveAll(filepath.Join(work, e.Name()))
			}
		}
		for name, content := range tags[tag] {
			p := filepath.Join(work, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		git("add", "-A")
		git("commit", "-q", "--allow-empty", "-m", tag)
		git("tag", tag)
	}
	return "file://" + filepath.ToSlash(work)
}

func sortedStrings(s []string) []string {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	return s
}

func projectJSON(publisher, name, version string, usages ...string) string {
	return `{"name": "` + name + `", "publisher": "` + publisher + `", "version": "` + version + `", "usage": [` + strings.Join(usages, ", ") + `]}` + "\n"
}

// archive writes a .kpar with .meta.json checksums, corrupted when asked.
func archive(t *testing.T, files map[string]string, corrupt bool) []byte {
	t.Helper()
	checksums := map[string]map[string]string{}
	index := map[string]string{}
	for name, content := range files {
		sum := sha256.Sum256([]byte(content))
		if corrupt {
			sum[0] ^= 0xff
		}
		checksums[name] = map[string]string{"algorithm": "SHA256", "value": hex.EncodeToString(sum[:])}
		if strings.HasSuffix(name, ".sysml") || strings.HasSuffix(name, ".kerml") {
			index[strings.TrimSuffix(strings.TrimSuffix(filepath.Base(name), ".sysml"), ".kerml")] = name
		}
	}
	meta, _ := json.Marshal(map[string]any{"index": index, "created": "2025-01-01T00:00:00Z", "checksum": checksums})
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}
	f, _ := w.Create(project.MetaFile)
	_, _ = f.Write(meta)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The gate: a project uses a tagged git repository and an archive behind a
// server, -add/-install/-lock install and pin them, and a model importing
// from the installed projects validates offline.
func TestAddInstallLockThenValidate(t *testing.T) {
	bin := sysmlBinary(t)
	gears := archive(t, map[string]string{project.ProjectFile: projectJSON("Acme", "Gears", "2.0.1"), "Gears.kerml": "package Gears { datatype Gear; }\n"}, false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gears-2.0.1.kpar":
			_, _ = w.Write(gears)
		case "/bad.kpar":
			_, _ = w.Write(archive(t, map[string]string{project.ProjectFile: projectJSON("Acme", "Bad", "1.0.0"), "Bad.sysml": "package Bad;\n"}, true))
		case "/escape.kpar":
			_, _ = w.Write(archive(t, map[string]string{project.ProjectFile: projectJSON("Acme", "Escape", "1.0.0"), "../Escape.sysml": "package Escape;\n"}, false))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	widgets := func(version string) map[string]string {
		return map[string]string{
			project.ProjectFile: projectJSON("Acme", "Widgets", version, `{"resource": "`+srv.URL+`/gears-2.0.1.kpar"}`),
			"Widgets.sysml":     "package Widgets { private import Gears::*; part def Widget { attribute g : Gear; } }\n",
		}
	}
	repo := gitRepo(t, map[string]map[string]string{
		"v1.0.0": widgets("1.0.0"),
		"v1.2.0": widgets("1.2.0"),
		"v0.9.0": widgets("0.8.0"), // mistagged: the tree says 0.8.0
		"v2.0.0": {"README.md": "no project here\n"},
	})

	proj := filepath.Join(t.TempDir(), "rover")
	if err := os.MkdirAll(proj, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "Rover.sysml"), []byte("package Rover { private import Widgets::*; part w : Widget; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, bin, "-project-init", "-publisher", "Example", "-name", "Rover", proj); err != nil {
		t.Fatalf("-project-init: %v\n%s", err, out)
	}
	if out, err := run(t, bin, "-add", repo+"@^1.0.0", proj); err != nil || !strings.Contains(out, "Widgets 1.2.0: installed at lib/acme-widgets_1.2.0") || !strings.Contains(out, "Gears 2.0.1: installed at lib/acme-gears_2.0.1.kpar") {
		t.Fatalf("-add: %v\n%s", err, out)
	}
	lock, err := os.ReadFile(filepath.Join(proj, project.LockFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"resource": "` + repo + `"`, `"version": "1.2.0"`, `"tag": "v1.2.0"`, `"commit": "`, `"digest": "sha256:`, `"path": "lib/acme-gears_2.0.1.kpar"`, `"versionConstraint": "^1.0.0"`} {
		if !strings.Contains(string(lock), want) {
			t.Errorf("lock lacks %s:\n%s", want, lock)
		}
	}
	if out, err := run(t, bin, "-validate", proj); err != nil {
		t.Fatalf("-validate after -add: %v\n%s", err, out)
	}
	if out, err := run(t, bin, "-validate", filepath.Join(proj, "Rover.sysml")); err != nil {
		t.Fatalf("-validate of the file: %v\n%s", err, out)
	}

	// A fresh clone with the lock reproduces the installation exactly.
	clone := filepath.Join(t.TempDir(), "rover")
	if err := os.MkdirAll(clone, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{project.ProjectFile, project.MetaFile, project.LockFile, "Rover.sysml"} {
		data, err := os.ReadFile(filepath.Join(proj, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(clone, f), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := run(t, bin, "-install", clone); err != nil || !strings.Contains(out, "2 projects installed and locked") {
		t.Fatalf("-install: %v\n%s", err, out)
	}
	cloneLock, _ := os.ReadFile(filepath.Join(clone, project.LockFile))
	if !bytes.Equal(cloneLock, lock) {
		t.Fatalf("-install rewrote the lock:\n%s", cloneLock)
	}
	if out, err := run(t, bin, "-validate", clone); err != nil {
		t.Fatalf("-validate of the clone: %v\n%s", err, out)
	}
	if out, err := run(t, bin, "-lock", clone); err != nil {
		t.Fatalf("-lock: %v\n%s", err, out)
	}
	if relocked, _ := os.ReadFile(filepath.Join(clone, project.LockFile)); !bytes.Equal(relocked, lock) {
		t.Fatalf("-lock resolved differently:\n%s", relocked)
	}

	// Refusals, each naming the IRI and the step, each leaving the project as it was.
	adv := filepath.Join(t.TempDir(), "adv")
	if out, err := run(t, bin, "-project-init", adv); err != nil || !strings.Contains(out, "(adv 0.1.0)") {
		t.Fatalf("-project-init without -name: %v\n%s", err, out)
	}
	before, _ := os.ReadFile(filepath.Join(adv, project.ProjectFile))
	for _, c := range []struct{ name, iri, want string }{
		{"mistagged version", repo + "@0.9.0", `verify: ` + repo + `@v0.9.0 is tagged v0.9.0 but its .project.json says version "0.8.0"`},
		{"missing .project.json", repo + "@2.0.0", "not a project interchange: " + repo + "@v2.0.0 has no .project.json"},
		{"checksum mismatch", srv.URL + "/bad.kpar", "verify: "},
		{"path escape", srv.URL + "/escape.kpar", "path has a .. segment"},
		{"unknown host type", "pkg:bitbucket/acme/widgets", `package URL type "bitbucket" is not a git host projects are installed from (github, gitlab)`},
		{"constraint with no match", repo + "@^9.0.0", "resolve: no release tag of " + repo + ` satisfies "^9.0.0"`},
		{"standard library", "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", "never installed"},
	} {
		out, err := run(t, bin, "-add", c.iri, adv)
		if err == nil || !strings.Contains(out, c.want) {
			t.Errorf("%s: err %v, want %q in:\n%s", c.name, err, c.want, out)
		}
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() != 2 {
			t.Errorf("%s: exit %d, want 2", c.name, exit.ExitCode())
		}
	}
	if after, _ := os.ReadFile(filepath.Join(adv, project.ProjectFile)); !bytes.Equal(after, before) {
		t.Fatalf("a refused -add changed .project.json:\n%s", after)
	}
	if out, err := run(t, bin, "-add", repo, proj); err == nil || !strings.Contains(out, "already uses it") {
		t.Fatalf("-add of a used resource: %v\n%s", err, out)
	}
	if out, err := run(t, bin, "-add", repo, "-validate", proj); err == nil || !strings.Contains(out, "loads no model") {
		t.Fatalf("-add with -validate: %v\n%s", err, out)
	}
	if out, err := run(t, bin, "-install", "-lock", proj); err == nil || !strings.Contains(out, "one per run") {
		t.Fatalf("-install with -lock: %v\n%s", err, out)
	}
}
