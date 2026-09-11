package solve

// The rounded-query census measures how often the conservative Rounded marker
// sweeps in queries that are in fact provably exact — the false-undecided rate
// that narrowing the marker per term would recover. Reproduce it with:
//
//	OPENSYSML_SMT=/usr/bin/z3 go test -count=1 -run TestRoundedCensus -v ./internal/core/solve
//	OPENSYSML_SMT=/usr/local/bin/cvc5 go test -count=1 -run TestRoundedCensus -v ./internal/core/solve
//
// The population is every element the solver surfaces translate — constraints,
// requirements, satisfaction assertions and analysis cases — over the corpora
// the repository holds: the OMG training corpus, the three pilot corpora, the
// runtime conformance corpus, the bundled standard library, the repository's
// example and manual models, and the solver's own fixtures.

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// censusRow is one translated query's classification.
type censusRow struct {
	file     string
	element  string
	rounded  bool
	exact    bool
	triggers []string
	status   Status
}

// censusSummary aggregates one corpus's rows.
type censusSummary struct {
	files      int
	elements   int
	translated int
	refused    int
	rows       []censusRow
}

// TestRoundedCensus counts, over the repository's solver-facing corpora, how
// many translated queries the Rounded marker sweeps in, and how many of those
// are provably exact — the queries a per-term narrowing would recover.
func TestRoundedCensus(t *testing.T) {
	solver := requireSolver(t)
	summaries := map[string]*censusSummary{}
	census := func(corpus string) *censusSummary {
		if summaries[corpus] == nil {
			summaries[corpus] = &censusSummary{}
		}
		return summaries[corpus]
	}

	censusFiles(t, solver, census("training corpus"), corpusFiles(t, trainingDir, true), true)
	censusFiles(t, solver, census("pilot corpora"), corpusFiles(t, "../../../examples/pilot-corpora", true), true)
	censusFiles(t, solver, census("conformance corpus"), corpusFiles(t, conformanceDir, false), false)
	censusFiles(t, solver, census("examples and manual"), exampleFiles(t), true)
	censusFiles(t, solver, census("solver fixtures"), corpusFiles(t, "testdata", false), true)
	censusLibrary(t, solver, census("standard library"))

	report(t, summaries)
}

// corpusFiles lists a corpus root's model files, skipping when an optional
// corpus is absent.
func corpusFiles(t *testing.T, root string, optional bool) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".sysml" || filepath.Ext(path) == ".kerml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		if optional && os.Getenv(corpusRequiredEnv) == "" {
			t.Logf("skipping corpus at %s (%v)", root, err)
			return nil
		}
		t.Fatalf("no models at %s (%v)", root, err)
	}
	sort.Strings(files)
	return files
}

// exampleFiles lists the repository's own example and manual models, excluding
// the OMG corpora counted in their own right.
func exampleFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, root := range []string{"../../../examples", "../../../docs/manual/examples"} {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if strings.Contains(path, "pilot-corpora") || strings.Contains(path, "sysml-v2-training") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) == ".sysml" || filepath.Ext(path) == ".kerml" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	sort.Strings(files)
	return files
}

// censusFiles runs the census over model files, loading each directory's files
// as one workspace so multi-file demos resolve across their own files.
func censusFiles(t *testing.T, solver *Solver, s *censusSummary, files []string, libraries bool) {
	t.Helper()
	byDir := map[string][]string{}
	var dirs []string
	for _, path := range files {
		dir := filepath.Dir(path)
		if len(byDir[dir]) == 0 {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], path)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		ctx, idx := workspaceOf(t, byDir[dir], libraries)
		for _, path := range byDir[dir] {
			s.files++
			censusDocument(t, solver, s, ctx, idx, path)
		}
	}
}

// workspaceOf indexes model files as one workspace, over the standard library
// when asked for.
func workspaceOf(t *testing.T, paths []string, libraries bool) (*runtime.Context, *symbols.Index) {
	t.Helper()
	idx := symbols.NewIndex()
	if libraries {
		idx = libraryIndex()
	}
	var sources []*source.SourceFile
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sf := source.New(path, src)
		sources = append(sources, sf)
		idx.AddDocument(path, parser.New(sf).ParseFile())
	}
	if libraries {
		idx.ExpandWildcardImports()
	}
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 10000)
	for _, sf := range sources {
		ctx.Model().RegisterSource(sf)
	}
	return ctx, idx
}

// censusLibrary runs the census over the bundled standard library.
func censusLibrary(t *testing.T, solver *Solver, s *censusSummary) {
	t.Helper()
	idx := symbols.NewIndex()
	parseLibraries(t, idx)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 10000)
	for _, doc := range libraryDocuments(idx) {
		s.files++
		censusDocument(t, solver, s, ctx, idx, doc)
	}
}

// censusDocument classifies every element of one document the solver surfaces
// translate: conditions as %check translates them, and analysis cases as
// %optimize does.
func censusDocument(t *testing.T, solver *Solver, s *censusSummary, ctx *runtime.Context, idx *symbols.Index, doc string) {
	t.Helper()
	for _, el := range conditionElements(ctx, idx, doc) {
		s.elements++
		q, err := el.translate(ctx)
		if err != nil {
			s.refused++
			continue
		}
		s.translated++
		s.rows = append(s.rows, classify(t, solver, doc, el.label(), q))
	}
	for _, sym := range analysisCases(idx, doc) {
		s.elements++
		q, err := Analysis(ctx, sym, sym.OwnerScope)
		if err != nil {
			s.refused++
			continue
		}
		s.translated++
		s.rows = append(s.rows, classify(t, solver, doc, "analysis "+elementName(sym), q))
	}
}

// analysisCases lists the analysis cases a document declares, in declaration
// order.
func analysisCases(idx *symbols.Index, doc string) []*symbols.Symbol {
	root := idx.DocumentRoot(doc)
	if root == nil {
		return nil
	}
	var out []*symbols.Symbol
	walkScopes(root, func(scope *symbols.Scope) {
		for _, sym := range scopeMembers(scope) {
			switch sym.Kind {
			case symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage:
				out = append(out, sym)
			}
		}
	})
	return out
}

// classify records one query's marking, its provable exactness, what trips the
// marker, and — for a marked query — what the solver answers, since the marker
// only changes a reported verdict.
func classify(t *testing.T, solver *Solver, file, element string, q *Query) censusRow {
	t.Helper()
	row := censusRow{file: file, element: element, rounded: q.Rounded(), exact: exactQuery(q)}
	if !row.rounded {
		return row
	}
	row.triggers = queryTriggers(q)
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		row.status = StatusUnknown
		return row
	}
	row.status = result.Status
	return row
}

// exactQuery reports whether every term the query asserts or optimizes is
// provably exact: its evaluator float64 value and its exact-real value coincide
// on every assignment the evaluator can hold.
func exactQuery(q *Query) bool {
	for _, a := range q.Assertions {
		if !exactCensusTerm(a.Term) {
			return false
		}
	}
	for _, o := range q.Objectives {
		if !exactCensusTerm(o.Term) {
			return false
		}
	}
	return true
}

// exactCensusTerm mirrors roundedTerm, but proves exactness instead of assuming
// rounding: a rounding-capable node is exact only when it is constant and every
// intermediate value is exactly representable in float64.
func exactCensusTerm(t *Term) bool {
	switch t.Op {
	case OpReal:
		if _, exact := t.Real.Float64(); !exact {
			return false
		}
	case OpAdd, OpSub, OpMul, OpDiv, OpNeg:
		if t.Sort.Kind == SortReal {
			if _, ok := exactFold(t); !ok {
				return false
			}
			return true
		}
	case OpToReal:
		if _, ok := exactFold(t); !ok {
			return false
		}
		return true
	}
	for _, arg := range t.Args {
		if !exactCensusTerm(arg) {
			return false
		}
	}
	return true
}

// exactFold folds a constant term exactly, reporting ok only when the evaluator
// computing it in float64 provably yields the same value: every intermediate a
// float64 rounds is exactly representable.
func exactFold(t *Term) (*big.Rat, bool) {
	switch t.Op {
	case OpInt:
		return new(big.Rat).SetInt64(t.Int), true
	case OpReal:
		if _, exact := t.Real.Float64(); !exact {
			return nil, false
		}
		return t.Real, true
	case OpToReal:
		v, ok := exactFold(t.Args[0])
		if !ok {
			return nil, false
		}
		// float64(int64) is exact only within ±2^53.
		if _, exact := v.Float64(); !exact {
			return nil, false
		}
		return v, true
	case OpNeg:
		v, ok := exactFold(t.Args[0])
		if !ok {
			return nil, false
		}
		return new(big.Rat).Neg(v), true
	case OpAdd, OpSub, OpMul, OpDiv:
		left, ok := exactFold(t.Args[0])
		if !ok {
			return nil, false
		}
		right, ok := exactFold(t.Args[1])
		if !ok {
			return nil, false
		}
		var v *big.Rat
		switch t.Op {
		case OpAdd:
			v = new(big.Rat).Add(left, right)
		case OpSub:
			v = new(big.Rat).Sub(left, right)
		case OpMul:
			v = new(big.Rat).Mul(left, right)
		case OpDiv:
			if right.Sign() == 0 {
				return nil, false
			}
			v = new(big.Rat).Quo(left, right)
		}
		if t.Sort.Kind == SortReal {
			if _, exact := v.Float64(); !exact {
				return nil, false
			}
		}
		return v, true
	}
	return nil, false
}

// queryTriggers names what trips the Rounded marker in a query, deduplicated.
func queryTriggers(q *Query) []string {
	seen := map[string]bool{}
	var out []string
	add := func(kind string) {
		if !seen[kind] {
			seen[kind] = true
			out = append(out, kind)
		}
	}
	for _, a := range q.Assertions {
		termTriggers(a.Term, add)
	}
	for _, o := range q.Objectives {
		termTriggers(o.Term, add)
	}
	sort.Strings(out)
	return out
}

// termTriggers classifies each node that trips roundedTerm: what rounds, and
// whether the node is constant (recoverable when exact) or over free values.
func termTriggers(t *Term, add func(string)) {
	switch t.Op {
	case OpReal:
		if _, exact := t.Real.Float64(); !exact {
			add("inexact real literal")
		}
	case OpAdd, OpSub, OpMul, OpDiv, OpNeg:
		if t.Sort.Kind == SortReal {
			if _, ok := exactFold(t); ok {
				add("constant real arithmetic (exact)")
			} else if constantTerm(t) {
				add("constant real arithmetic (inexact)")
			} else if t.Op == OpDiv {
				add("division over free values")
			} else {
				add("real arithmetic over free values")
			}
		}
	case OpToReal:
		if _, ok := exactFold(t); ok {
			add("integer widening (constant, exact)")
		} else if constantTerm(t) {
			add("integer widening (constant, beyond 2^53)")
		} else {
			add("integer widening over free values")
		}
	}
	for _, arg := range t.Args {
		termTriggers(arg, add)
	}
}

// constantTerm reports whether no variable takes part in the term.
func constantTerm(t *Term) bool {
	if t.Op == OpVar {
		return false
	}
	for _, arg := range t.Args {
		if !constantTerm(arg) {
			return false
		}
	}
	return true
}

// report logs the census: per-corpus counts, then every marked query with what
// trips it and what the solver answered.
func report(t *testing.T, summaries map[string]*censusSummary) {
	t.Helper()
	corpora := make([]string, 0, len(summaries))
	for corpus := range summaries {
		corpora = append(corpora, corpus)
	}
	sort.Strings(corpora)
	var rounded, recoverable int
	for _, corpus := range corpora {
		s := summaries[corpus]
		var marked, exact int
		for _, row := range s.rows {
			if row.rounded {
				marked++
				if row.exact {
					exact++
				}
			} else if !row.exact {
				t.Errorf("%s: %s in %s is unmarked yet not provably exact — the census contradicts the marker",
					corpus, row.element, row.file)
			}
		}
		rounded += marked
		recoverable += exact
		t.Logf("%s: %d files, %d elements, %d translated (%d refused), %d marked rounded, %d of those provably exact",
			corpus, s.files, s.elements, s.translated, s.refused, marked, exact)
		for _, row := range s.rows {
			if row.rounded {
				t.Logf("  rounded [%s] %s in %s: triggers=%s; solver answers %s",
					map[bool]string{true: "provably exact", false: "inexact"}[row.exact],
					row.element, row.file, strings.Join(row.triggers, ", "), row.status)
			}
		}
	}
	t.Logf("total: %d queries marked rounded, %d provably exact (recoverable), %d genuinely inexact",
		rounded, recoverable, rounded-recoverable)
}
