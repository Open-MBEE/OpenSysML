package runtime

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/stressmodel"
)

// sparseDifferentialMaxSteps bounds one instantiation on either side, so a
// model that never settles stops at the budget on both.
const sparseDifferentialMaxSteps int64 = 20000

// TestSparseValuesDifferential instantiates every part declared at the top of
// every model under the differential roots with shared defaults on and off,
// requiring the same readable values, materialization errors and verdicts.
// Each file is a parallel subtest: every one builds its own model and contexts.
func TestSparseValuesDifferential(t *testing.T) {
	var files []string
	for _, root := range differentialRoots {
		files = append(files, sysmlFilesUnder(t, root)...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no .sysml files under the differential roots")
	}
	var parts, shared atomic.Int64
	t.Run("files", func(t *testing.T) {
		for _, path := range files {
			t.Run(path, func(t *testing.T) {
				t.Parallel()
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				n, taken := sparseDifferentialSource(t, path, src, partSymbolsUnder)
				parts.Add(int64(n))
				shared.Add(int64(taken))
			})
		}
	})
	if parts.Load() == 0 {
		t.Fatal("no part instantiated in any file")
	}
	t.Logf("%d files: %d parts compared, %d defaults and verdicts shared", len(files), parts.Load(), shared.Load())
}

// TestSparseValuesDifferentialFleet compares both sides over the fleet form of
// the stress-test constellation: two planes of twenty, each a block whose
// occurrences share its defaults except the two units per plane stating
// as-built values of their own. The constellation is read through the parts
// the model declares, so each definition is read once, as the type of its usage.
func TestSparseValuesDifferentialFleet(t *testing.T) {
	network := stressmodel.SatelliteNetwork{Planes: 2, Satellites: 20, GroundStations: 2, Fleet: true}
	src, stats := network.Source()
	name := fmt.Sprintf("fleet-%d.sysml", stats.Satellites)
	parts, shared := sparseDifferentialSource(t, name, []byte(src), partUsagesUnder)
	if shared == 0 {
		t.Errorf("%s: no default or verdict shared between the occurrences", name)
	}
	t.Logf("%s: %d parts compared, %d defaults and verdicts shared", name, parts, shared)
}

// sparseDifferentialSource builds the model src, named path, once and
// instantiates each of the parts roots picks from it on a sharing and a
// materializing context, returning the number compared and how many values the
// sharing side took from its type rather than deriving.
func sparseDifferentialSource(t *testing.T, path string, src []byte, roots func(*symbols.Scope) []*symbols.Symbol) (parts, shared int) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, src)).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	root := idx.DocumentRoot(path)
	for _, sym := range roots(root) {
		sharing := NewContext(NewModel(model, resolver), sparseDifferentialMaxSteps)
		sharing.SetSharedDefaults(true)
		materializing := NewContext(NewModel(model, resolver), sparseDifferentialMaxSteps)
		materializing.SetSharedDefaults(false)
		name := path + ": " + sym.Name
		got, taken := sparseReading(sharing, sym, root)
		want, _ := sparseReading(materializing, sym, root)
		if got != want {
			t.Errorf("%s: sharing defaults reads differently from materializing them\n--- sharing\n%s\n--- materializing\n%s", name, got, want)
		}
		parts++
		shared += taken
	}
	return parts, shared
}

// sparseReading instantiates sym on ctx and reports everything the object shows:
// every readable value, the errors reading raised, and every verdict validating
// it decides — and how many defaults and verdicts the context shared.
func sparseReading(ctx *Context, sym *symbols.Symbol, root *symbols.Scope) (string, int) {
	done := ctx.ShareVerdicts()
	defer done()
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return "instantiate: " + err.Error(), 0
	}
	var b strings.Builder
	w := &readableWalk{ctx: ctx, out: &b, visited: map[int64]bool{inst.ID: true}}
	w.walk(inst, sym.Name, 0)
	report, err := ctx.ValidateObject(inst, []*symbols.Scope{root})
	if err != nil {
		fmt.Fprintf(&b, "validate: %v\n", err)
	}
	for _, v := range report.Verdicts {
		fmt.Fprintf(&b, "%s %q on %q: %s", v.Kind, v.Text, strings.Join(v.Path, "."), v.Status)
		if v.Err != nil {
			fmt.Fprintf(&b, " (%v)", v.Err)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "valid %v bounded %v unread %d\n", report.Valid(), report.Bounded, len(report.Unread))
	return b.String(), int(ctx.SharedDefaultsTaken()) + ctx.SharedVerdictsTaken()
}

// readableWalk reads every value an object graph shows, down to the depth a
// listing descends, naming held objects by path rather than by identity.
type readableWalk struct {
	ctx     *Context
	out     *strings.Builder
	visited map[int64]bool
}

func (w *readableWalk) walk(inst *Instance, path string, depth int) {
	if depth > maxMaterializeDepth {
		fmt.Fprintf(w.out, "%s: (not expanded)\n", path)
		return
	}
	for _, of := range w.ctx.FeaturesOfObject(inst) {
		if holdsVerdict(of.Feature) || isBehaviorKind(of.Feature) {
			continue
		}
		name := path + "." + of.Name
		fv, err := inst.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			fmt.Fprintf(w.out, "%s: <error: %v>\n", name, err)
			continue
		}
		val, err := fv.ReadValue(of.Name)
		if err != nil {
			fmt.Fprintf(w.out, "%s: <read error: %v>\n", name, err)
			continue
		}
		fmt.Fprintf(w.out, "%s = %s\n", name, w.format(val))
		for i, held := range heldInstances(w.ctx, fv) {
			if w.visited[held.ID] {
				continue
			}
			w.visited[held.ID] = true
			w.walk(held, fmt.Sprintf("%s[%d]", name, i+1), depth+1)
		}
	}
}

// format renders a value with objects named by type, since the two sides
// number objects differently when one derives without materializing.
func (w *readableWalk) format(val Value) string {
	switch val.Kind {
	case ValInstance, ValVariant:
		if id, ok := val.Object(); ok {
			if inst, ok := w.ctx.Instance(id); ok && inst.Type != nil {
				return "object " + inst.Type.Name
			}
			return "object"
		}
	case ValSequence:
		if seq := val.Sequence(); seq != nil {
			return "[" + strings.Join(w.formatAll(seq.Elements()), ", ") + "]"
		}
	case ValSet:
		if set := val.Set(); set != nil {
			return "Set{" + strings.Join(w.formatAll(set.Elements()), ", ") + "}"
		}
	}
	return FormatValue(val)
}

func (w *readableWalk) formatAll(elements []Value) []string {
	parts := make([]string, len(elements))
	for i, element := range elements {
		parts[i] = w.format(element)
	}
	return parts
}

// isBehaviorKind reports whether a feature is a behavior an object performs
// rather than a value it holds; an abstract one is a collection held.
func isBehaviorKind(feat *EffectiveFeature) bool {
	if feat.Symbol == nil || symbols.IsAbstract(feat.Symbol) {
		return false
	}
	switch feat.Symbol.Kind {
	case symbols.SymbolStateUsage, symbols.SymbolActionUsage:
		return true
	}
	return false
}

// partUsagesUnder lists the part usages declared directly in scope's packages:
// the objects the model declares, through which their definitions are read.
func partUsagesUnder(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, sym := range partSymbolsUnder(scope) {
		if sym.Kind == symbols.SymbolPartUsage {
			out = append(out, sym)
		}
	}
	return out
}

// partSymbolsUnder lists the part definitions and usages declared directly in
// scope's packages, the objects a listing instantiates by name.
func partSymbolsUnder(scope *symbols.Scope) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, sym := range scope.Members() {
		switch sym.Kind {
		case symbols.SymbolPartDef, symbols.SymbolPartUsage:
			out = append(out, sym)
		case symbols.SymbolPackage:
			out = append(out, partSymbolsUnder(sym.Scope)...)
		}
	}
	return out
}
