package corpus

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

var updateCorpusRoundTrip = flag.Bool("update-corpus-roundtrip", false,
	"rewrite testdata/corpus_roundtrip_expected.txt from the current results")

var updateAPIJSONRoundTrip = flag.Bool("update-api-json-roundtrip", false,
	"rewrite testdata/api_json_roundtrip_expected.txt from the current results")

const (
	corpusRoundTripExamples  = "../../examples"
	corpusRoundTripExpected  = "testdata/corpus_roundtrip_expected.txt"
	apiJSONRoundTripExpected = "testdata/api_json_roundtrip_expected.txt"

	// Byte-identical to the committed file's header, so regenerating without a
	// movement rewrites it unchanged.
	corpusRoundTripHeader = "# Round-trip verdict for every model under examples/, as \"<verdict>\\t<path>\":\n" +
		"# notation -> Turtle (hop 1) -> notation -> Turtle (hop 2), then the two\n" +
		"# Turtle graphs compared as triple sets. Verdicts: stable (hop 2 is\n" +
		"# byte-identical), whitespace-only (bytes differ, triple sets equal once the\n" +
		"# whitespace inside sysx:sourceText literals is normalised), graph-diff,\n" +
		"# unwritable (Turtle -> notation refused), unparseable (the written notation\n" +
		"# no longer converts) and refused:<class> (notation -> Turtle refused). This\n" +
		"# is a per-file ratchet, not a claim that any verdict is right; see\n" +
		"# docs/project/rdf-corpus-roundtrip.md. Regenerate with:\n" +
		"#   go test ./tests/corpus -run TestCorpusRoundTrip -update-corpus-roundtrip\n"

	apiJSONRoundTripHeader = "# Round-trip verdict for every model under examples/, as \"<verdict>\\t<path>\":\n" +
		"# notation -> the API's JSON element form (hop 1) -> notation -> the JSON\n" +
		"# element form (hop 2), then the two graphs compared as triple sets.\n" +
		"# Verdicts: stable (hop 2 is byte-identical), whitespace-only (bytes\n" +
		"# differ, triple sets equal once the whitespace inside sysx:sourceText\n" +
		"# literals is normalised), graph-diff, unwritable (the JSON form ->\n" +
		"# notation refused), unparseable (the written notation no longer\n" +
		"# converts) and refused:<class> (notation -> the JSON form refused).\n" +
		"# This is a per-file ratchet, not a claim that any verdict is right;\n" +
		"# see docs/project/rdf-corpus-roundtrip.md. Regenerate with:\n" +
		"#   go test ./tests/corpus -run TestCorpusAPIJSONRoundTrip -update-api-json-roundtrip\n"
)

// roundTripGate is one per-file ratchet over the corpus: the same walk,
// worker pool, expectation file and comparison, differing only in the hop
// format each verdict round-trips through.
type roundTripGate struct {
	name       string // the gate's name in the summary line CI greps
	updateFlag string // the flag regeneration messages name
	update     *bool
	expected   string
	header     string
	verdict    func(rel string, src []byte) (string, error)
}

var corpusRoundTripGate = roundTripGate{
	name:       "corpus round trip",
	updateFlag: "-update-corpus-roundtrip",
	update:     updateCorpusRoundTrip,
	expected:   corpusRoundTripExpected,
	header:     corpusRoundTripHeader,
	verdict:    corpusRoundTripVerdict,
}

var apiJSONRoundTripGate = roundTripGate{
	name:       "api-json corpus round trip",
	updateFlag: "-update-api-json-roundtrip",
	update:     updateAPIJSONRoundTrip,
	expected:   apiJSONRoundTripExpected,
	header:     apiJSONRoundTripHeader,
	verdict:    apiJSONRoundTripVerdict,
}

// corpusRoundTripRoot is one downloaded corpus under examples/. The committed
// models are everything under examples/ outside these roots.
type corpusRoundTripRoot struct {
	name       string // slash-separated path relative to examples/
	requireEnv string
	fetch      string
}

const corpusRoundTripCommitted = "committed"

var corpusRoundTripRoots = []corpusRoundTripRoot{
	{
		name:       "sysml-v2-training",
		requireEnv: "OPENSYSML_REQUIRE_TRAINING_CORPUS",
		fetch:      "./scripts/download-training-examples.sh",
	},
	{
		name:       "pilot-corpora/kerml-examples",
		requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA",
		fetch:      "./scripts/download-pilot-corpora.sh",
	},
	{
		name:       "pilot-corpora/sysml-examples",
		requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA",
		fetch:      "./scripts/download-pilot-corpora.sh",
	},
	{
		name:       "pilot-corpora/sysml-validation",
		requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA",
		fetch:      "./scripts/download-pilot-corpora.sh",
	},
}

// skip mirrors corpusGate.skip: skip locally, fail when the require-env is
// set, and announce it on stderr either way.
func (r corpusRoundTripRoot) skip(t *testing.T, reason string) {
	t.Helper()
	hint := fmt.Sprintf("examples/%s not downloaded (run %s)", r.name, r.fetch)
	if os.Getenv(r.requireEnv) != "" {
		t.Fatalf("%s=%s but %s: %s", r.requireEnv, os.Getenv(r.requireEnv), reason, hint)
	}
	fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
		"!!! The corpus is absent, so this run proves nothing about it.\n"+
		"!!! Fetch it with %s and re-run\n"+
		"!!!   go test -count=1 ./tests/corpus -run TestCorpusRoundTrip\n"+
		"!!! CI sets %s=1, where an absent corpus fails instead of skipping.\n\n",
		t.Name(), reason, r.fetch, r.requireEnv)
	t.Skip(hint)
}

// corpusRoundTripRootOf names the root a path relative to examples/ belongs to.
func corpusRoundTripRootOf(rel string) string {
	for _, root := range corpusRoundTripRoots {
		if strings.HasPrefix(rel, root.name+"/") {
			return root.name
		}
	}
	return corpusRoundTripCommitted
}

// corpusRoundTripFiles returns every model under examples/ as sorted
// slash-separated relative paths, plus the file count per root.
func corpusRoundTripFiles(t *testing.T) ([]string, map[string]int) {
	t.Helper()

	for _, root := range corpusRoundTripRoots {
		dir := filepath.Join(corpusRoundTripExamples, filepath.FromSlash(root.name))
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			root.skip(t, dir+" is missing")
		}
	}

	var files []string
	err := filepath.WalkDir(corpusRoundTripExamples, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch source.KindOf(path) {
		case source.KindSysML, source.KindKerML:
		default:
			return nil
		}
		rel, err := filepath.Rel(corpusRoundTripExamples, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", corpusRoundTripExamples, err)
	}
	sort.Strings(files)

	totals := make(map[string]int, len(corpusRoundTripRoots)+1)
	for _, rel := range files {
		totals[corpusRoundTripRootOf(rel)]++
	}
	if totals[corpusRoundTripCommitted] == 0 {
		t.Fatalf("%s holds no committed model files", corpusRoundTripExamples)
	}
	// An empty directory is a broken download or a bad cache restore, not a corpus.
	for _, root := range corpusRoundTripRoots {
		if totals[root.name] == 0 {
			root.skip(t, "examples/"+root.name+" holds no model files")
		}
	}
	return files, totals
}

// corpusRoundTripVerdict classifies one model's notation -> Turtle -> notation
// -> Turtle trip. An error means the harness failed, never a verdict.
func corpusRoundTripVerdict(rel string, src []byte) (string, error) {
	hop1, err := convert.Convert(rel, src, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		return "refused:" + refusalClass(rel, err), nil
	}
	back, err := convert.Convert(rel+".ttl", hop1, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		return "unwritable", nil
	}
	hop2, err := convert.Convert(rel, back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		return "unparseable", nil
	}
	if bytes.Equal(hop1, hop2) {
		return "stable", nil
	}
	first, err := rdf.ParseTurtle(hop1)
	if err != nil {
		return "", fmt.Errorf("hop 1 Turtle does not parse: %w", err)
	}
	second, err := rdf.ParseTurtle(hop2)
	if err != nil {
		return "", fmt.Errorf("hop 2 Turtle does not parse: %w", err)
	}
	if sameTriples(first, second) {
		return "whitespace-only", nil
	}
	return "graph-diff", nil
}

// apiJSONRoundTripVerdict classifies one model's notation -> api-json ->
// notation -> api-json trip. The two hops parse back to graphs so the
// whitespace-only check compares what the JSON states, not its key order.
func apiJSONRoundTripVerdict(rel string, src []byte) (string, error) {
	hop1, err := convert.Convert(rel, src, convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		return "refused:" + refusalClass(rel, err), nil
	}
	back, err := convert.Convert(rel+".json", hop1, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		return "unwritable", nil
	}
	hop2, err := convert.Convert(rel, back, convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		return "unparseable", nil
	}
	if bytes.Equal(hop1, hop2) {
		return "stable", nil
	}
	first, err := export.ReadAPIJSON(hop1)
	if err != nil {
		return "", fmt.Errorf("hop 1 api-json does not parse: %w", err)
	}
	second, err := export.ReadAPIJSON(hop2)
	if err != nil {
		return "", fmt.Errorf("hop 2 api-json does not parse: %w", err)
	}
	if sameTriples(first, second) {
		return "whitespace-only", nil
	}
	return "graph-diff", nil
}

// sameTriples compares two graphs as sets, ignoring triple order and the
// whitespace inside sysx:sourceText literals.
func sameTriples(a, b *rdf.Graph) bool {
	set := func(g *rdf.Graph) map[rdf.Triple]bool {
		sourceText := rdf.OpenSysMLTerm("sourceText")
		out := make(map[rdf.Triple]bool, len(g.Triples()))
		for _, triple := range g.Triples() {
			if triple.Predicate == sourceText && triple.Object.IsLiteral() {
				triple.Object.Value = strings.Join(strings.Fields(triple.Object.Value), " ")
			}
			out[triple] = true
		}
		return out
	}
	first, second := set(a), set(b)
	if len(first) != len(second) {
		return false
	}
	for triple := range first {
		if !second[triple] {
			return false
		}
	}
	return true
}

// refusalLocation matches the " at <file>:<line>:<column>" tail of a What.
var refusalLocation = regexp.MustCompile(`^(.*?) at .*:\d+:\d+$`)

// refusalIdentifier matches the element a What names ("of Pkg::x", an IRI).
var refusalIdentifier = regexp.MustCompile(` (of|on) .*$|<[^>]*>`)

var refusalSlug = regexp.MustCompile(`[^a-z0-9]+`)

// refusalClass reduces a refusal to a stable class: the construct kind without
// location or identifiers, "syntax" for a parse failure, "error" otherwise.
func refusalClass(name string, err error) string {
	var unsupported *export.UnsupportedError
	var syntax *convert.SyntaxError
	switch {
	case errors.As(err, &unsupported):
		what := unsupported.What
		if i := strings.Index(what, " at "+name+":"); i >= 0 {
			what = what[:i]
		} else if m := refusalLocation.FindStringSubmatch(what); m != nil {
			what = m[1]
		}
		what = refusalIdentifier.ReplaceAllString(what, "")
		what = strings.TrimPrefix(strings.TrimSpace(what), "the ")
		return strings.Trim(refusalSlug.ReplaceAllString(strings.ToLower(what), "-"), "-")
	case errors.As(err, &syntax):
		return "syntax"
	default:
		return "error"
	}
}

// verdicts measures every file on a worker pool, indexed like files so
// scheduling never affects the order.
func (g roundTripGate) verdicts(t *testing.T, files []string) map[string]string {
	t.Helper()

	verdicts := make([]string, len(files))
	failures := make([]error, len(files))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				rel := files[i]
				src, err := os.ReadFile(filepath.Join(corpusRoundTripExamples, filepath.FromSlash(rel)))
				if err != nil {
					failures[i] = err
					continue
				}
				verdicts[i], failures[i] = g.verdict(rel, src)
			}
		}()
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()

	got := make(map[string]string, len(files))
	for i, rel := range files {
		if failures[i] != nil {
			t.Errorf("%s: %v", rel, failures[i])
			continue
		}
		got[rel] = verdicts[i]
	}
	return got
}

// summary is the per-verdict count line CI greps for.
func (g roundTripGate) summary(files []string, got map[string]string) string {
	counts := make(map[string]int)
	for _, rel := range files {
		verdict := got[rel]
		if strings.HasPrefix(verdict, "refused:") {
			verdict = "refused"
		}
		counts[verdict]++
	}
	var parts []string
	for _, verdict := range []string{"stable", "whitespace-only", "graph-diff", "unwritable", "unparseable", "refused"} {
		parts = append(parts, fmt.Sprintf("%d %s", counts[verdict], verdict))
	}
	return fmt.Sprintf("%s: %d files: %s", g.name, len(files), strings.Join(parts, ", "))
}

func (g roundTripGate) readExpected(t *testing.T) (map[string]int, map[string]string) {
	t.Helper()

	content, err := os.ReadFile(g.expected)
	if err != nil {
		t.Fatalf("read %s: %v", g.expected, err)
	}

	totals := make(map[string]int)
	want := make(map[string]string)
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if rest, ok := strings.CutPrefix(line, "# files: "); ok {
			root, count, found := strings.Cut(rest, " ")
			if !found {
				t.Fatalf("%s:%d: want \"# files: <root> <n>\", got %q", g.expected, i+1, line)
			}
			totals[root], err = strconv.Atoi(count)
			if err != nil {
				t.Fatalf("%s:%d: bad file count: %v", g.expected, i+1, err)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		verdict, path, found := strings.Cut(line, "\t")
		if !found || verdict == "" || path == "" {
			t.Fatalf("%s:%d: want \"<verdict>\\t<path>\", got %q", g.expected, i+1, line)
		}
		if _, dup := want[path]; dup {
			t.Fatalf("%s:%d: %s is recorded twice", g.expected, i+1, path)
		}
		want[path] = verdict
	}
	return totals, want
}

func (g roundTripGate) writeExpected(t *testing.T, files []string, totals map[string]int, got map[string]string) {
	t.Helper()

	var b strings.Builder
	b.WriteString(g.header)
	fmt.Fprintf(&b, "# files: %s %d\n", corpusRoundTripCommitted, totals[corpusRoundTripCommitted])
	for _, root := range corpusRoundTripRoots {
		fmt.Fprintf(&b, "# files: %s %d\n", root.name, totals[root.name])
	}
	for _, rel := range files {
		fmt.Fprintf(&b, "%s\t%s\n", got[rel], rel)
	}
	if err := os.WriteFile(g.expected, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", g.expected, err)
	}
}

// run measures every file and either regenerates the gate's expectation or
// fails on any movement from it.
func (g roundTripGate) run(t *testing.T) {
	files, totals := corpusRoundTripFiles(t)

	// An empty semantic cache, as on a fresh checkout and in CI.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := g.verdicts(t, files)
	t.Log(g.summary(files, got))

	if *g.update {
		g.writeExpected(t, files, totals, got)
		t.Logf("wrote %s: %d file(s)", g.expected, len(files))
		return
	}

	wantTotals, want := g.readExpected(t)
	for _, root := range append([]string{corpusRoundTripCommitted}, rootNames()...) {
		if wantTotals[root] != totals[root] {
			t.Errorf("%s holds %d model file(s), expectations were recorded against %d; "+
				"re-download the pinned corpus or regenerate with %s",
				root, totals[root], wantTotals[root], g.updateFlag)
		}
	}

	for _, path := range sortedStringKeys(want) {
		switch verdict, ok := got[path]; {
		case !ok:
			t.Errorf("%s: recorded %s but the file is gone; regenerate with %s",
				path, want[path], g.updateFlag)
		case verdict != want[path]:
			t.Errorf("%s: %s, expected %s; adjudicate the change, then regenerate with %s",
				path, verdict, want[path], g.updateFlag)
		}
	}
	for _, path := range files {
		if _, ok := want[path]; !ok {
			t.Errorf("%s: %s but the file is not recorded; regenerate with %s",
				path, got[path], g.updateFlag)
		}
	}
}

// TestCorpusRoundTrip is a per-file ratchet on the RDF round trip of every
// model under examples/; any movement fails it. See docs/project/rdf-corpus-roundtrip.md.
func TestCorpusRoundTrip(t *testing.T) {
	corpusRoundTripGate.run(t)
}

// TestCorpusAPIJSONRoundTrip is the same ratchet over the API's JSON element
// form, whose reader and writer share the RDF graph. See
// docs/project/rdf-corpus-roundtrip.md.
func TestCorpusAPIJSONRoundTrip(t *testing.T) {
	apiJSONRoundTripGate.run(t)
}

// Refusal classes must carry no location or identifier, however spelt.
func TestRefusalClass(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a/b.sysml", &export.UnsupportedError{What: "the `feature` declaration at a/b.sysml:69:9"}, "feature-declaration"},
		{"b.sysml", &export.UnsupportedError{What: "the duplicate declaration of \"transitionLink\" at b.sysml:21:5"}, "duplicate-declaration"},
		{"c.sysml", &export.UnsupportedError{What: "the succession at Some Dir/at home/c.sysml:12:3"}, "succession"},
		{"d.sysml", &export.UnsupportedError{What: "the expression <urn:x:e>"}, "expression"},
		{"e.sysml", &export.UnsupportedError{What: "the owning membership of Pkg::Part"}, "owning-membership"},
		{"e.sysml", &export.UnsupportedError{What: "the ElementId annotation on Pkg::Part"}, "elementid-annotation"},
		{"e.sysml", &export.UnsupportedError{What: "usage kind \"part\" at e.sysml:1:1"}, "usage-kind-part"},
		{"e.sysml", &export.UnsupportedError{What: "an empty document"}, "an-empty-document"},
		{"f.sysml", &convert.SyntaxError{Name: "f.sysml", Messages: []string{"f.sysml:1:1: unexpected token"}}, "syntax"},
		{"g.sysml", errors.New("boom"), "error"},
	}
	for _, c := range cases {
		if got := refusalClass(c.name, c.err); got != c.want {
			t.Errorf("refusalClass(%q, %v) = %q, want %q", c.name, c.err, got, c.want)
		}
	}
}

// Only sysx:sourceText whitespace and triple order are ignored.
func TestSameTriples(t *testing.T) {
	parse := func(turtle string) *rdf.Graph {
		g, err := rdf.ParseTurtle([]byte(turtle))
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	const prefixes = "@prefix sysx: <" + rdf.OpenSysML + "> .\n@prefix sysml: <" + rdf.SysML + "> .\n"
	a := parse(prefixes + "<urn:a> sysx:sourceText \"part  x :\\n  P;\" ; sysml:declaredName \"x\" .\n")
	if !sameTriples(a, parse(prefixes+"<urn:a> sysml:declaredName \"x\" ; sysx:sourceText \"part x : P;\" .\n")) {
		t.Error("sourceText whitespace and triple order must not distinguish graphs")
	}
	if sameTriples(a, parse(prefixes+"<urn:a> sysx:sourceText \"part  x :\\n  P;\" ; sysml:declaredName \"x \" .\n")) {
		t.Error("whitespace in a literal other than sysx:sourceText must distinguish graphs")
	}
	if sameTriples(a, parse(prefixes+"<urn:a> sysx:sourceText \"part x : P;\" .\n")) {
		t.Error("a missing triple must distinguish graphs")
	}
}

func rootNames() []string {
	names := make([]string, 0, len(corpusRoundTripRoots))
	for _, root := range corpusRoundTripRoots {
		names = append(names, root.name)
	}
	return names
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
