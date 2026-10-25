package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const widgetsModel = "package Widgets { part def Widget; }\n"

func projectJSON(name, version string, usages ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "{\n  \"name\": %q,\n  \"publisher\": \"Acme\",\n  \"version\": %q", name, version)
	if len(usages) > 0 {
		b.WriteString(",\n  \"usage\": [")
		for i, u := range usages {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(u)
		}
		b.WriteString("]")
	}
	b.WriteString("\n}\n")
	return b.String()
}

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_AUTHOR_DATE=2024-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org", "GIT_COMMITTER_DATE=2024-01-01T00:00:00Z",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// taggedRepo makes a git repository whose tags each hold the given tree.
func taggedRepo(t *testing.T, tags []string, trees ...map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	for i, tag := range tags {
		for _, entry := range mustReadDir(t, dir) {
			if entry.Name() != ".git" {
				_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
			}
		}
		writeTree(t, dir, trees[i])
		runGit(t, dir, "add", "-A")
		runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "release "+tag)
		runGit(t, dir, "tag", "-a", "-m", tag, tag)
	}
	return dir
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// tarGz writes a host-style tag archive: one top directory holding files.
func tarGz(t *testing.T, top string, files map[string]string, extra func(*tar.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	for _, name := range names {
		content := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: top + "/" + name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		extra(tw)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// kpar writes an archive of files, generating .meta.json unless given.
func kpar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	dir := t.TempDir()
	writeTree(t, dir, files)
	data, _, err := project.Bundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func widgetsTree(version string) map[string]string {
	return map[string]string{project.ProjectFile: projectJSON("Widgets", version), "Widgets.sysml": widgetsModel, ".github/ci.yml": "x", "README.md": "Widgets"}
}

func TestFetchTaggedGitRepo(t *testing.T) {
	repo := taggedRepo(t, []string{"v1.0.0", "v1.1.0", "2.0.0-rc.1", "not-a-release"},
		widgetsTree("1.0.0"), widgetsTree("1.1.0"), widgetsTree("2.0.0-rc.1"), widgetsTree("9.9.9"))
	commit := runGit(t, repo, "rev-list", "-n", "1", "v1.1.0")
	f := &Fetcher{}
	o := &project.Origin{IRI: "file://" + repo, Kind: project.OriginGit, Remote: "file://" + repo}
	c, _ := project.ParseConstraint("^1.0.0")
	rel, err := f.Fetch(context.Background(), o, c)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "1.1.0" || rel.Tag != "v1.1.0" || rel.Commit != commit || rel.SourceURL != "file://"+repo {
		t.Fatalf("release %+v: want 1.1.0 at %s", rel, commit)
	}
	if !strings.HasPrefix(rel.Digest, "sha256:") || !rel.Installable() {
		t.Fatalf("digest %q, installable %v", rel.Digest, rel.Installable())
	}
	a, err := project.ParseArchive("mem", rel.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if names := strings.Join(a.Names(), " "); names != ".meta.json .project.json README.md Widgets.sysml" {
		t.Fatalf("members %q: hidden .github must stay out, .meta.json be generated", names)
	}
	if rel.Project.Name != "Widgets" || rel.Project.Publisher != "Acme" {
		t.Fatalf("project %+v", rel.Project)
	}
}

func TestFetchHostArchive(t *testing.T) {
	repo := taggedRepo(t, []string{"v1.0.0"}, widgetsTree("1.0.0"))
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch r.URL.Path {
		case "/acme/widgets/archive/refs/tags/v1.0.0.tar.gz":
			_, _ = w.Write(tarGz(t, "widgets-1.0.0", widgetsTree("1.0.0"), nil))
		case "/acme/widgets/-/archive/v1.0.0/widgets-v1.0.0.tar.gz":
			_, _ = w.Write(tarGz(t, "widgets-v1.0.0", widgetsTree("1.0.0"), nil))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	gitBase := t.TempDir()
	if err := os.Symlink(repo, filepath.Join(gitBase, "widgets.git")); err != nil {
		t.Skip(err)
	}
	if err := os.MkdirAll(filepath.Join(gitBase, "acme"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(gitBase, "widgets.git"), filepath.Join(gitBase, "acme", "widgets.git")); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"github.com", "gitlab.com"} {
		f := &Fetcher{HostURLs: map[string]string{host: srv.URL}}
		o, err := project.ParseOrigin(project.Usage{Resource: "pkg:git/" + host + "/acme/widgets", VersionConstraint: "1.0.0"})
		if err != nil {
			t.Fatal(err)
		}
		o.Remote = "file://" + filepath.Join(gitBase, "acme", "widgets.git")
		rel, err := f.Fetch(context.Background(), o, mustConstraint(t, o.Constraint))
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if !strings.HasPrefix(rel.SourceURL, srv.URL) || rel.Tag != "v1.0.0" || rel.Commit == "" {
			t.Fatalf("%s: release %+v", host, rel)
		}
		gitRel, err := f.Fetch(context.Background(), &project.Origin{IRI: "file://" + repo, Kind: project.OriginGit, Remote: "file://" + repo}, mustConstraint(t, "1.0.0"))
		if err != nil {
			t.Fatal(err)
		}
		if gitRel.Digest != rel.Digest {
			t.Fatalf("%s: the tree digest over the archive %s differs from the clone %s", host, rel.Digest, gitRel.Digest)
		}
	}
	if len(hits) != 2 {
		t.Fatalf("archive requests %v: one per host", hits)
	}
}

func mustConstraint(t *testing.T, text string) project.Constraint {
	t.Helper()
	c, err := project.ParseConstraint(text)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFetchFallsBackToCloneWhenArchiveFails(t *testing.T) {
	repo := taggedRepo(t, []string{"v1.0.0"}, widgetsTree("1.0.0"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer srv.Close()
	gitBase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gitBase, "acme"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(gitBase, "acme", "widgets.git")); err != nil {
		t.Skip(err)
	}
	f := &Fetcher{HostURLs: map[string]string{"github.com": srv.URL}}
	o, _ := project.ParseOrigin(project.Usage{Resource: "pkg:git/github.com/acme/widgets@1.0.0"})
	o.Remote = "file://" + filepath.Join(gitBase, "acme", "widgets.git")
	rel, err := f.Fetch(context.Background(), o, mustConstraint(t, o.Constraint))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(rel.SourceURL, "/acme/widgets.git") {
		t.Fatalf("source %q: want the clone URL", rel.SourceURL)
	}
}

func TestFetchRemoteArchive(t *testing.T) {
	data := kpar(t, widgetsTree("1.0.0"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()
	o, _ := project.ParseOrigin(project.Usage{Resource: srv.URL + "/widgets.kpar"})
	rel, err := (&Fetcher{}).Fetch(context.Background(), o, project.Constraint{})
	if err != nil {
		t.Fatal(err)
	}
	if rel.Digest != digest(data) || !bytes.Equal(rel.Archive, data) || rel.Version != "1.0.0" {
		t.Fatalf("release %+v", rel)
	}
}

func TestFetchLocalInPlace(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, widgetsTree("1.0.0"))
	o, _ := project.ParseOrigin(project.Usage{Resource: dir})
	rel, err := (&Fetcher{}).Fetch(context.Background(), o, project.Constraint{})
	if err != nil {
		t.Fatal(err)
	}
	if rel.Installable() || rel.Path != dir || rel.Version != "1.0.0" {
		t.Fatalf("release %+v: a local directory is read in place", rel)
	}
	archive := filepath.Join(t.TempDir(), "w.kpar")
	if err := os.WriteFile(archive, kpar(t, widgetsTree("1.0.0")), 0o600); err != nil {
		t.Fatal(err)
	}
	o, _ = project.ParseOrigin(project.Usage{Resource: archive})
	if rel, err = (&Fetcher{}).Fetch(context.Background(), o, project.Constraint{}); err != nil || rel.Installable() || rel.Path != archive {
		t.Fatalf("archive in place: %+v, %v", rel, err)
	}
}

func TestFetchRefuses(t *testing.T) {
	repo := taggedRepo(t, []string{"v1.0.0", "v1.1.0", "v1.2.0"},
		widgetsTree("1.0.0"), widgetsTree("9.9.9"), map[string]string{"Widgets.sysml": widgetsModel})
	bad := kpar(t, widgetsTree("1.0.0"))
	bad = corruptMember(t, bad, "Widgets.sysml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad.kpar":
			_, _ = w.Write(bad)
		case "/acme/escape/archive/refs/tags/v1.0.0.tar.gz":
			_, _ = w.Write(tarGz(t, "top", widgetsTree("1.0.0"), func(tw *tar.Writer) {
				_ = tw.WriteHeader(&tar.Header{Name: "top/../../etc/passwd", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
				_, _ = tw.Write([]byte("x"))
			}))
		case "/acme/big/archive/refs/tags/v1.0.0.tar.gz":
			_, _ = w.Write(tarGz(t, "top", map[string]string{project.ProjectFile: projectJSON("Big", "1.0.0"), "big.sysml": strings.Repeat("x", 4096)}, nil))
		case "/acme/away/archive/refs/tags/v1.0.0.tar.gz":
			http.Redirect(w, r, "http://elsewhere.invalid/x.tar.gz", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	gitBase := t.TempDir()
	for _, name := range []string{"escape", "big", "away"} {
		if err := os.Symlink(repo, filepath.Join(gitBase, "acme", name+".git")); err != nil {
			if err := os.MkdirAll(filepath.Join(gitBase, "acme"), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(repo, filepath.Join(gitBase, "acme", name+".git")); err != nil {
				t.Skip(err)
			}
		}
	}
	f := &Fetcher{HostURLs: map[string]string{"github.com": srv.URL}, MaxBytes: 2048}
	git := func(c string) (*project.Origin, project.Constraint) {
		return &project.Origin{IRI: "file://" + repo, Kind: project.OriginGit, Remote: "file://" + repo}, mustConstraint(t, c)
	}
	host := func(resource string) (*project.Origin, project.Constraint) {
		o, err := project.ParseOrigin(project.Usage{Resource: resource})
		if err != nil {
			t.Fatal(err)
		}
		if o.Kind == project.OriginGit {
			o.Remote = "file://" + filepath.Join(gitBase, o.Path+".git")
		}
		return o, mustConstraint(t, o.Constraint)
	}
	cases := []struct {
		name string
		o    *project.Origin
		c    project.Constraint
		step string
		msg  string
	}{
		{"mistagged", first(git("1.1.0")), second(git("1.1.0")), "verify", `tagged v1.1.0 but its .project.json says version "9.9.9"`},
		{"missing project file", first(git("1.2.0")), second(git("1.2.0")), "verify", "not a project interchange: file://" + repo + "@v1.2.0 has no .project.json"},
		{"no matching constraint", first(git(">=3.0.0")), second(git(">=3.0.0")), "resolve", `no release tag of file://` + repo + ` satisfies ">=3.0.0"; the tags name versions 1.0.0, 1.1.0, 1.2.0`},
		{"checksum mismatch", first(host(srv.URL + "/bad.kpar")), project.Constraint{}, "verify", "checksum of \"Widgets.sysml\" does not match"},
		{"path escape", first(host("pkg:git/github.com/acme/escape@1.0.0")), second(host("pkg:git/github.com/acme/escape@1.0.0")), "fetch", "path has a .. segment"},
		{"size bound", first(host("pkg:git/github.com/acme/big@1.0.0")), second(host("pkg:git/github.com/acme/big@1.0.0")), "fetch", "more than 2048 bytes"},
		{"redirect off host", first(host("pkg:git/github.com/acme/away@1.0.0")), second(host("pkg:git/github.com/acme/away@1.0.0")), "fetch", "redirect leaves the host"},
		{"standard library", &project.Origin{IRI: "https://www.omg.org/spec/KerML/20250201/Kernel-Semantic-Library.kpar", Kind: project.OriginStandard}, project.Constraint{}, "resolve", "never installed"},
		{"local without project", &project.Origin{IRI: t.TempDir(), Kind: project.OriginLocal, Path: t.TempDir()}, project.Constraint{}, "verify", "not a project interchange"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.Fetch(context.Background(), tc.o, tc.c)
			var ie *InstallError
			if !errors.As(err, &ie) {
				t.Fatalf("Fetch = %v, want an InstallError", err)
			}
			if ie.Step != tc.step || ie.IRI != tc.o.IRI || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("error %q: want step %s of %s mentioning %q", err, tc.step, tc.o.IRI, tc.msg)
			}
		})
	}
}

func first[A, B any](a A, _ B) A  { return a }
func second[A, B any](_ A, b B) B { return b }

// corruptMember rewrites one member of an archive so its recorded checksum
// no longer matches.
func corruptMember(t *testing.T, data []byte, member string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == member {
			content = append(content, []byte("// tampered\n")...)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(content)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestExtractTarGzRefusesSiblingTopDirectories(t *testing.T) {
	data := tarGz(t, "a", map[string]string{"x": "1"}, func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "b/y", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("2"))
	})
	if _, err := extractTarGz(data, 1<<20); err == nil || !strings.Contains(err.Error(), "one directory") {
		t.Fatalf("extractTarGz = %v", err)
	}
	symlinked := tarGz(t, "a", map[string]string{project.ProjectFile: "{}"}, func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "a/link", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink})
	})
	members, err := extractTarGz(symlinked, 1<<20)
	if err != nil || len(members) != 1 {
		t.Fatalf("symlink entries are left out: %v %v", members, err)
	}
}
