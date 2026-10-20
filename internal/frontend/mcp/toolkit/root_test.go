package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree lays out a workspace root beside an outside directory, with the links
// every tool group must judge the same way.
type tree struct {
	base, root, outside string
}

func newTree(t *testing.T) tree {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tr := tree{base: base, root: filepath.Join(base, "root"), outside: filepath.Join(base, "outside")}
	for _, dir := range []string{filepath.Join(tr.root, "sub"), tr.outside} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	tr.write(t, filepath.Join(tr.root, "a.sysml"), "package A;")
	tr.write(t, filepath.Join(tr.outside, "b.sysml"), "package B;")
	tr.link(t, filepath.Join(tr.outside, "b.sysml"), filepath.Join(tr.root, "link.sysml"))
	tr.link(t, tr.outside, filepath.Join(tr.root, "dirlink"))
	tr.link(t, filepath.Join(tr.outside, "missing.sysml"), filepath.Join(tr.root, "dangle.sysml"))
	tr.link(t, "missing.sysml", filepath.Join(tr.root, "dangle-in.sysml"))
	tr.link(t, filepath.Join(tr.root, "a.sysml"), filepath.Join(tr.root, "alias.kerml"))
	tr.link(t, filepath.Join(tr.root, "a.sysml"), filepath.Join(tr.root, "copy.sysml"))
	tr.link(t, tr.root, filepath.Join(base, "rootlink"))
	return tr
}

func (tr tree) write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (tr tree) link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func TestResolveKeepsEveryPathUnderTheRoot(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	if r.Dir != tr.root || r.Real != tr.root {
		t.Fatalf("NewRoot(%s) = %+v", tr.root, r)
	}
	accepted := map[string]Resolved{
		".":                                {Path: tr.root, Real: tr.root},
		tr.root:                            {Path: tr.root, Real: tr.root},
		"a.sysml":                          {Path: filepath.Join(tr.root, "a.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"  a.sysml ":                       {Path: filepath.Join(tr.root, "a.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"sub/../a.sysml":                   {Path: filepath.Join(tr.root, "a.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		filepath.Join(tr.root, "a.sysml"):  {Path: filepath.Join(tr.root, "a.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"copy.sysml":                       {Path: filepath.Join(tr.root, "copy.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"alias.kerml":                      {Path: filepath.Join(tr.root, "alias.kerml"), Real: filepath.Join(tr.root, "a.sysml")},
		"sub/new.sysml":                    {Path: filepath.Join(tr.root, "sub", "new.sysml"), Real: filepath.Join(tr.root, "sub", "new.sysml")},
		"sub/deeper/new.sysml":             {Path: filepath.Join(tr.root, "sub", "deeper", "new.sysml"), Real: filepath.Join(tr.root, "sub", "deeper", "new.sysml")},
		"dangle-in.sysml":                  {Path: filepath.Join(tr.root, "dangle-in.sysml"), Real: filepath.Join(tr.root, "missing.sysml")},
		"dangle-in.sysml/deeper/new.sysml": {Path: filepath.Join(tr.root, "dangle-in.sysml", "deeper", "new.sysml"), Real: filepath.Join(tr.root, "missing.sysml", "deeper", "new.sysml")},
	}
	for in, want := range accepted {
		if got, err := r.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	refused := map[string]string{
		"":                                   "a path is required",
		"   ":                                "a path is required",
		"..":                                 tr.base + " is outside the workspace root " + tr.root,
		"../outside/b.sysml":                 filepath.Join(tr.outside, "b.sysml") + " is outside the workspace root " + tr.root,
		"sub/../../outside/b.sysml":          filepath.Join(tr.outside, "b.sysml") + " is outside the workspace root " + tr.root,
		filepath.Join(tr.outside, "b.sysml"): filepath.Join(tr.outside, "b.sysml") + " is outside the workspace root " + tr.root,
		tr.root + "-sibling/b.sysml":         tr.root + "-sibling/b.sysml is outside the workspace root " + tr.root,
		"link.sysml":                         filepath.Join(tr.root, "link.sysml") + " leads through a symlink to " + filepath.Join(tr.outside, "b.sysml") + ", outside the workspace root " + tr.root,
		"dirlink/b.sysml":                    filepath.Join(tr.root, "dirlink", "b.sysml") + " leads through a symlink to " + filepath.Join(tr.outside, "b.sysml") + ", outside the workspace root " + tr.root,
		"dirlink/new/deeper.sysml":           filepath.Join(tr.root, "dirlink", "new", "deeper.sysml") + " leads through a symlink to " + filepath.Join(tr.outside, "new", "deeper.sysml") + ", outside the workspace root " + tr.root,
		"dangle.sysml":                       filepath.Join(tr.root, "dangle.sysml") + " leads through a symlink to " + filepath.Join(tr.outside, "missing.sysml") + ", outside the workspace root " + tr.root,
		"dangle.sysml/deeper.sysml":          filepath.Join(tr.root, "dangle.sysml", "deeper.sysml") + " leads through a symlink to " + filepath.Join(tr.outside, "missing.sysml", "deeper.sysml") + ", outside the workspace root " + tr.root,
	}
	for in, want := range refused {
		if got, err := r.Resolve(in); err == nil || err.Error() != want {
			t.Errorf("Resolve(%q) = %+v, %v; want %q", in, got, err, want)
		}
	}
}

func TestARootThatIsASymlinkIsJudgedAsSpelledAndWhereItPoints(t *testing.T) {
	tr := newTree(t)
	spelled := filepath.Join(tr.base, "rootlink")
	r, err := NewRoot(spelled)
	if err != nil {
		t.Fatal(err)
	}
	if r.Dir != spelled || r.Real != tr.root {
		t.Fatalf("NewRoot(%s) = %+v", spelled, r)
	}
	for in, want := range map[string]Resolved{
		".":               {Path: spelled, Real: tr.root},
		"a.sysml":         {Path: filepath.Join(spelled, "a.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"copy.sysml":      {Path: filepath.Join(spelled, "copy.sysml"), Real: filepath.Join(tr.root, "a.sysml")},
		"sub/new.sysml":   {Path: filepath.Join(spelled, "sub", "new.sysml"), Real: filepath.Join(tr.root, "sub", "new.sysml")},
		"dangle-in.sysml": {Path: filepath.Join(spelled, "dangle-in.sysml"), Real: filepath.Join(tr.root, "missing.sysml")},
	} {
		if got, err := r.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	// The root's own resolved path is spelled outside the root as given, and
	// is refused like any other path spelled elsewhere.
	for _, in := range []string{tr.root, filepath.Join(tr.root, "a.sysml"), "link.sysml", "dirlink/b.sysml", "dangle.sysml"} {
		if got, err := r.Resolve(in); err == nil || !strings.Contains(err.Error(), "outside the workspace root "+spelled) {
			t.Errorf("Resolve(%q) = %+v, %v; want a refusal naming the root as spelled", in, got, err)
		}
	}
}

func TestNewRootTakesARelativeDirFromTheWorkingDirectoryAndNeedsItToExist(t *testing.T) {
	tr := newTree(t)
	t.Chdir(tr.base)
	r, err := NewRoot("rootlink")
	if err != nil || r.Dir != filepath.Join(tr.base, "rootlink") || r.Real != tr.root {
		t.Errorf("NewRoot(rootlink) = %+v, %v", r, err)
	}
	if got, err := r.Resolve(filepath.Join(tr.base, "rootlink", "a.sysml")); err != nil || got.Real != filepath.Join(tr.root, "a.sysml") {
		t.Errorf("Resolve(an absolute path under the relative root) = %+v, %v", got, err)
	}
	if _, err := NewRoot("nowhere"); err == nil || !strings.Contains(err.Error(), "resolving the workspace root nowhere") {
		t.Errorf("NewRoot(nowhere) = %v; want a refusal", err)
	}
}

func TestExemptPassesAFileReallyUnderAnExemptDirectory(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(tr.base, "lib")
	if err := os.Mkdir(lib, 0o750); err != nil {
		t.Fatal(err)
	}
	tr.write(t, filepath.Join(lib, "lib.sysml"), "package Lib;")
	tr.link(t, filepath.Join(tr.outside, "b.sysml"), filepath.Join(lib, "planted.sysml"))
	tr.link(t, lib, filepath.Join(tr.base, "liblink"))
	in := filepath.Join(lib, "lib.sysml")
	if _, err := r.Resolve(in); err == nil {
		t.Errorf("Resolve(%q) without the exemption succeeded", in)
	}
	for _, exempt := range []string{lib, filepath.Join(tr.base, "liblink")} {
		if got, err := r.Resolve(in, Exempt(exempt)); err != nil || got != (Resolved{Path: in, Real: in}) {
			t.Errorf("Resolve(%q, Exempt(%s)) = %+v, %v", in, exempt, got, err)
		}
		via := filepath.Join(tr.base, "liblink", "lib.sysml")
		if got, err := r.Resolve(via, Exempt(exempt)); err != nil || got != (Resolved{Path: via, Real: in}) {
			t.Errorf("Resolve(%q, Exempt(%s)) = %+v, %v", via, exempt, got, err)
		}
	}
	// A link planted under the exempt directory to a file elsewhere is not exempt, nor is a file beside it.
	for _, in := range []string{filepath.Join(lib, "planted.sysml"), filepath.Join(tr.outside, "b.sysml"), filepath.Join(tr.base, "lib-sibling", "x.sysml")} {
		if got, err := r.Resolve(in, Exempt(lib)); err == nil || !strings.Contains(err.Error(), "outside the workspace root") {
			t.Errorf("Resolve(%q, Exempt(lib)) = %+v, %v; want a refusal", in, got, err)
		}
	}
	// An exempt directory that does not exist exempts nothing.
	if _, err := r.Resolve(filepath.Join(tr.base, "nowhere", "x.sysml"), Exempt(filepath.Join(tr.base, "nowhere"))); err == nil {
		t.Error("a nonexistent exempt directory passed a path under it")
	}
}

func TestNameMeansRefusesALinkWhoseNameMeansAnotherFormat(t *testing.T) {
	tr := newTree(t)
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	tr.write(t, filepath.Join(tr.root, "graph.turtle"), "@prefix ex: <http://example.org/> .\n")
	tr.link(t, filepath.Join(tr.root, "graph.turtle"), filepath.Join(tr.root, "graph.ttl"))
	tr.link(t, filepath.Join(tr.root, "a.sysml"), filepath.Join(tr.root, "input.data"))
	tr.link(t, filepath.Join(tr.root, "a.sysml"), filepath.Join(tr.root, "a.json"))
	mismatch := "whose extension names another format"
	for _, tc := range []struct {
		in     string
		role   NameRole
		refuse bool
	}{
		{"alias.kerml", NameIgnored, false},
		{"alias.kerml", NameLanguage, true},
		{"alias.kerml", NameFormat, true},
		{"copy.sysml", NameFormat, false},
		{"a.sysml", NameFormat, false},
		{"graph.ttl", NameFormat, false},
		{"input.data", NameLanguage, true},
		{"a.json", NameLanguage, true},
		{"a.json", NameFormat, true},
		{"sub/new.kerml", NameFormat, false},
	} {
		got, err := r.Resolve(tc.in, NameMeans(tc.role))
		if refused := err != nil && strings.Contains(err.Error(), mismatch); refused != tc.refuse || (err != nil && !refused) {
			t.Errorf("Resolve(%q, NameMeans(%d)) = %+v, %v; want refused=%v", tc.in, tc.role, got, err, tc.refuse)
		}
	}
	// The name check does not let a link out of the root pass.
	if _, err := r.Resolve("link.sysml", NameMeans(NameFormat)); err == nil || !strings.Contains(err.Error(), "outside the workspace root") {
		t.Errorf("Resolve(link.sysml, NameMeans) = %v", err)
	}
}

func TestUnderJudgesTheSpellingOnly(t *testing.T) {
	for _, tc := range []struct {
		root, path string
		want       bool
	}{
		{"/w", "/w", true},
		{"/w", "/w/a", true},
		{"/w", "/w/a/b", true},
		{"/w", "/", false},
		{"/w", "/w2", false},
		{"/w", "/w/../x", false},
		{"/w", "/x/w/a", false},
		{"/w/sub", "/w", false},
	} {
		if got := Under(filepath.FromSlash(tc.root), filepath.FromSlash(tc.path)); got != tc.want {
			t.Errorf("Under(%s, %s) = %v", tc.root, tc.path, got)
		}
	}
}

func TestAMemberNamedBeneathAnArchiveFileIsJudgedByTheArchive(t *testing.T) {
	tr := newTree(t)
	tr.write(t, filepath.Join(tr.root, "model.kpar"), "not a directory")
	tr.write(t, filepath.Join(tr.outside, "away.kpar"), "not a directory")
	tr.link(t, filepath.Join(tr.outside, "away.kpar"), filepath.Join(tr.root, "linked.kpar"))
	r, err := NewRoot(tr.root)
	if err != nil {
		t.Fatal(err)
	}
	member := filepath.Join("model.kpar", "A.sysml")
	res, err := r.Resolve(member)
	if err != nil {
		t.Fatalf("Resolve(%q) through an archive file in the root: %v", member, err)
	}
	if want := filepath.Join(tr.root, member); res.Path != want || res.Real != want {
		t.Errorf("Resolve(%q) = %+v, want Path and Real %s", member, res, want)
	}
	linked := filepath.Join("linked.kpar", "A.sysml")
	if _, err := r.Resolve(linked); err == nil || !strings.Contains(err.Error(), "leads through a symlink to") {
		t.Errorf("Resolve(%q) through an archive linked from outside = %v; want a refusal", linked, err)
	}
}
