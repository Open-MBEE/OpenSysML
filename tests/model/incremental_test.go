package model_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/tests/stressmodel"
)

// A workspace edited incrementally (scripted and seeded random edits, reverts, closes,
// opens) must answer as one built fresh: same diagnostics, resolutions and references.

// replayDoc is one document of a replay: the versions it has had, and which one
// is open, if any.
type replayDoc struct {
	name     string
	versions [][]byte
	open     bool
	current  int
}

// replay drives an incremental workspace and checks it against a fresh one.
type replay struct {
	t     *testing.T
	ws    *model.Workspace
	docs  []*replayDoc
	rng   *rand.Rand
	steps []string
	next  int // version counter handed to Open/Update
	extra int // numbers the declarations mutations insert

	checkHook func(*replay)
}

func freshOf(r *replay) *model.Workspace {
	fresh := model.NewWorkspace()
	for _, d := range r.openDocs() {
		fresh.Open(d.name, d.versions[d.current], 1)
	}
	return fresh
}

func newReplay(t *testing.T, files map[string][]byte, seed int64) *replay {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	r := &replay{t: t, ws: model.NewWorkspace(), rng: rand.New(rand.NewSource(seed))}
	for _, name := range names {
		r.docs = append(r.docs, &replayDoc{name: name, versions: [][]byte{files[name]}})
	}
	for _, d := range r.docs {
		r.open(d, 0)
	}
	r.check("initial open")
	return r
}

func (r *replay) open(d *replayDoc, version int) {
	r.next++
	d.open, d.current = true, version
	r.ws.Open(d.name, d.versions[version], r.next)
}

func (r *replay) update(d *replayDoc, version int) {
	r.next++
	d.current = version
	r.ws.Update(d.name, d.versions[version], r.next)
}

func (r *replay) close(d *replayDoc) {
	d.open = false
	r.ws.Close(d.name)
}

func (r *replay) openDocs() []*replayDoc {
	var out []*replayDoc
	for _, d := range r.docs {
		if d.open {
			out = append(out, d)
		}
	}
	return out
}

func (r *replay) closedDocs() []*replayDoc {
	var out []*replayDoc
	for _, d := range r.docs {
		if !d.open {
			out = append(out, d)
		}
	}
	return out
}

// step performs one operation, logging it for the failure report.
func (r *replay) step(desc string, op func()) {
	r.steps = append(r.steps, desc)
	op()
	r.check(desc)
}

// scripted edits every document once, reverts it, then closes and reopens one.
func (r *replay) scripted() {
	for _, d := range r.docs {
		r.step("mutate "+d.name, func() { r.mutate(d, 0) })
		r.step("revert "+d.name, func() { r.update(d, 0) })
	}
	if len(r.docs) > 1 {
		last := r.docs[len(r.docs)-1]
		r.step("close "+last.name, func() { r.close(last) })
		r.step("reopen "+last.name, func() { r.open(last, 0) })
	}
}

// randomized performs n seeded operations: edits to a mutation or to an earlier
// version, closes and opens.
func (r *replay) randomized(n int) {
	for i := 0; i < n; i++ {
		open, closed := r.openDocs(), r.closedDocs()
		switch k := r.rng.Intn(10); {
		case k < 6 && len(open) > 0:
			d := open[r.rng.Intn(len(open))]
			r.step("mutate "+d.name, func() { r.mutate(d, r.rng.Intn(4)) })
		case k < 7 && len(open) > 0:
			d := open[r.rng.Intn(len(open))]
			v := r.rng.Intn(len(d.versions))
			r.step(fmt.Sprintf("edit %s to version %d", d.name, v), func() { r.update(d, v) })
		case k < 8 && len(open) > 1:
			d := open[r.rng.Intn(len(open))]
			r.step("close "+d.name, func() { r.close(d) })
		case len(closed) > 0:
			d := closed[r.rng.Intn(len(closed))]
			v := r.rng.Intn(len(d.versions))
			r.step(fmt.Sprintf("open %s at version %d", d.name, v), func() { r.open(d, v) })
		default:
			if len(open) > 0 {
				d := open[r.rng.Intn(len(open))]
				r.step("mutate "+d.name, func() { r.mutate(d, r.rng.Intn(4)) })
			}
		}
	}
}

var identifierRE = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)
var packageRE = regexp.MustCompile(`\bpackage\s+([A-Za-z_][A-Za-z0-9_]*)`)

// mutate derives a new version of d from its current one and installs it:
// a deleted line, a renamed identifier, a declaration or an import inserted.
func (r *replay) mutate(d *replayDoc, kind int) {
	src := string(d.versions[d.current])
	var out string
	switch kind {
	case 0:
		lines := strings.Split(src, "\n")
		var candidates []int
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				candidates = append(candidates, i)
			}
		}
		if len(candidates) == 0 {
			return
		}
		i := candidates[r.rng.Intn(len(candidates))]
		out = strings.Join(append(lines[:i:i], lines[i+1:]...), "\n")
	case 1:
		locs := identifierRE.FindAllStringIndex(src, -1)
		if len(locs) == 0 {
			return
		}
		loc := locs[r.rng.Intn(len(locs))]
		out = src[:loc[1]] + "_x" + src[loc[1]:]
	case 2:
		i := strings.IndexByte(src, '{')
		if i < 0 {
			return
		}
		braces := indexesOf(src, '{')
		i = braces[r.rng.Intn(len(braces))]
		r.extra++
		decl := fmt.Sprintf(" part def Extra%d; ", r.extra)
		if source.KindOf(d.name) == source.KindKerML {
			decl = fmt.Sprintf(" class Extra%d; ", r.extra)
		}
		out = src[:i+1] + decl + src[i+1:]
	default:
		braces := indexesOf(src, '{')
		if len(braces) == 0 {
			return
		}
		pkgs := r.packageNames()
		if len(pkgs) == 0 {
			return
		}
		i := braces[r.rng.Intn(len(braces))]
		out = src[:i+1] + " import " + pkgs[r.rng.Intn(len(pkgs))] + "::*; " + src[i+1:]
	}
	d.versions = append(d.versions, []byte(out))
	r.update(d, len(d.versions)-1)
}

func indexesOf(s string, c byte) []int {
	var out []int
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			out = append(out, i)
		}
	}
	return out
}

// packageNames lists the packages the documents declare, so an inserted import
// can reach across documents.
func (r *replay) packageNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range r.docs {
		for _, m := range packageRE.FindAllStringSubmatch(string(d.versions[d.current]), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	sort.Strings(out)
	return out
}

// check compares the incremental workspace with one built fresh from the open
// documents' current versions.
func (r *replay) check(after string) {
	r.t.Helper()
	fresh := freshOf(r)
	for _, d := range r.openDocs() {
		got, want := workspaceAnswers(r.ws, d.name), workspaceAnswers(fresh, d.name)
		if diff := firstDifference(got, want); diff != "" {
			if r.checkHook != nil {
				r.checkHook(r)
			}
			r.t.Fatalf("after %q (steps: %s)\n%s: incremental differs from fresh:\n%s",
				after, strings.Join(r.steps, "; "), d.name, diff)
		}
	}
}

// workspaceAnswers renders everything the workspace says about a document:
// its diagnostics, what each written reference resolves to, and the reverse
// references of each declared element.
func workspaceAnswers(ws *model.Workspace, name string) []string {
	var out []string
	for _, d := range ws.Diagnostics(name) {
		out = append(out, fmt.Sprintf("diag %d+%d %s %s/%s: %s",
			d.Span.Offset, d.Span.Len, d.Severity, d.Source, d.Code, d.Message))
	}
	doc := ws.Document(name)
	if doc == nil {
		return append(out, "no document")
	}
	for _, ref := range resolve.References(doc.AST, doc.Scope) {
		if ref.QN == nil || len(ref.QN.Parts) == 0 {
			continue
		}
		sym, ok := ws.ResolveReferenceInDoc(name, ref)
		line := fmt.Sprintf("ref %d+%d -> %v %s", ref.QN.Span().Offset, ref.QN.Span().Len, ok, symbolID(sym))
		for _, seg := range ws.ResolveReferenceSegmentsInDoc(name, ref) {
			line += " " + symbolID(seg)
		}
		out = append(out, line)
	}
	var syms []*symbols.Symbol
	walkSymbols(doc.Scope, func(sym *symbols.Symbol) { syms = append(syms, sym) })
	for _, sym := range syms {
		locs := ws.ReferencesTo(sym)
		if len(locs) == 0 {
			continue
		}
		parts := make([]string, 0, len(locs))
		for _, loc := range locs {
			parts = append(parts, fmt.Sprintf("%s:%d+%d", loc.Doc, loc.Span.Offset, loc.Span.Len))
		}
		sort.Strings(parts)
		out = append(out, "refs-to "+symbolID(sym)+" <- "+strings.Join(parts, " "))
	}
	return out
}

func symbolID(sym *symbols.Symbol) string {
	if sym == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s@%s:%d+%d", sym.Name, sym.DocName, sym.DeclSpan.Offset, sym.DeclSpan.Len)
}

func walkSymbols(scope *symbols.Scope, visit func(*symbols.Symbol)) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		visit(sym)
		return true
	})
	for _, child := range scope.Children() {
		walkSymbols(child, visit)
	}
}

func firstDifference(got, want []string) string {
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			return fmt.Sprintf("line %d\n  incremental: %s\n  fresh:       %s", i, g, w)
		}
	}
	return ""
}

// fixtureSets are the repository's own multi-file models: each example
// directory, the loose example files together, and each testdata directory.
func fixtureSets(t *testing.T) map[string]map[string][]byte {
	t.Helper()
	sets := map[string]map[string][]byte{}
	add := func(set, name, path string) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if sets[set] == nil {
			sets[set] = map[string][]byte{}
		}
		sets[set][name] = content
	}
	entries, err := os.ReadDir("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		path := filepath.Join("../../examples", e.Name())
		switch {
		case e.Name() == "sysml-v2-training" || e.Name() == "pilot-corpora":
			continue
		case e.IsDir():
			for _, f := range modelFiles(t, path) {
				add("examples/"+e.Name(), filepath.ToSlash(f), filepath.Join(path, f))
			}
		case model.IsModelSource(e.Name()):
			add("examples", e.Name(), path)
		}
	}
	dirs, err := os.ReadDir("../testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range dirs {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join("../testdata", e.Name())
		for _, f := range modelFiles(t, path) {
			add("testdata/"+e.Name(), filepath.ToSlash(f), filepath.Join(path, f))
		}
	}
	src, _ := stressmodel.SatelliteNetwork{Planes: 2, Satellites: 2, GroundStations: 1}.Source()
	sets["stressmodel"] = map[string][]byte{
		"satnet.sysml": []byte(src),
		"ops.sysml":    []byte("package Ops { private import SatelliteNetwork::Constellation::*; part spare : Sat0; }"),
	}
	return sets
}

// modelFiles lists the model files under dir, relative to it and sorted.
func modelFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && model.IsModelSource(path) {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// languageSets splits files into one workspace per language: KerML and SysML
// files must not share a workspace, as the corpus gates hold.
func languageSets(files map[string][]byte) map[string]map[string][]byte {
	out := map[string]map[string][]byte{}
	for name, content := range files {
		lang := "sysml"
		if source.KindOf(name) == source.KindKerML {
			lang = "kerml"
		}
		if out[lang] == nil {
			out[lang] = map[string][]byte{}
		}
		out[lang][name] = content
	}
	return out
}

func TestIncrementalEqualsFresh(t *testing.T) {
	for set, files := range fixtureSets(t) {
		for lang, docs := range languageSets(files) {
			if len(docs) == 0 {
				continue
			}
			t.Run(set+"/"+lang, func(t *testing.T) {
				r := newReplay(t, docs, 1)
				r.scripted()
				r.randomized(12)
			})
		}
	}
}

// corpusRoots are the four OMG model roots the corpus gates pin, replayed under
// the same absence policy: skipped locally, failed when the require variable is set.
var corpusRoots = []struct{ dir, requireEnv, fetch string }{
	{"../../examples/sysml-v2-training", "OPENSYSML_REQUIRE_TRAINING_CORPUS", "./scripts/download-training-examples.sh"},
	{"../../examples/pilot-corpora/kerml-examples", "OPENSYSML_REQUIRE_PILOT_CORPORA", "./scripts/download-pilot-corpora.sh"},
	{"../../examples/pilot-corpora/sysml-examples", "OPENSYSML_REQUIRE_PILOT_CORPORA", "./scripts/download-pilot-corpora.sh"},
	{"../../examples/pilot-corpora/sysml-validation", "OPENSYSML_REQUIRE_PILOT_CORPORA", "./scripts/download-pilot-corpora.sh"},
}

func TestIncrementalEqualsFreshCorpora(t *testing.T) {
	for _, root := range corpusRoots {
		t.Run(filepath.Base(root.dir), func(t *testing.T) {
			if _, err := os.Stat(root.dir); os.IsNotExist(err) {
				if os.Getenv(root.requireEnv) != "" {
					t.Fatalf("%s is set but %s is missing (run %s)", root.requireEnv, root.dir, root.fetch)
				}
				t.Skipf("%s not downloaded (run %s)", root.dir, root.fetch)
			}
			files := map[string][]byte{}
			for _, f := range modelFiles(t, root.dir) {
				content, err := os.ReadFile(filepath.Join(root.dir, f))
				if err != nil {
					t.Fatal(err)
				}
				files[filepath.ToSlash(f)] = content
			}
			for lang, docs := range languageSets(files) {
				t.Run(lang, func(t *testing.T) {
					r := newReplay(t, docs, 2)
					r.randomized(6)
				})
			}
		})
	}
}
