package export

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// structureDifferential compares, for one model, every served property of every
// element the graph structure knows against the symbol-backed structure.
type structureDifferential struct {
	graph    *apiJSONGraphStructure
	symbolic *metamodel.SymbolStructure
	ids      *ElementIDs
	index    *symbols.Index
	doc      string
	symbolID map[*symbols.Symbol]string
	// identityToGraph maps the symbol structure's element identities to the
	// ids the conversion wrote, so id-valued strings compare across structures.
	identityToGraph map[string]string
	graphIDs        map[string]bool
	idProperty      bool
	// counts are per "Class::property": equal, different, symbol-missing, symbol-extra.
	counts map[string]*differentialCounts
	// samples keeps the first few differences per property, for the report.
	samples   map[string][]string
	matched   int
	unmatched []string
}

type differentialCounts struct{ equal, different, missing, extra int }

// structureDifferentialRoots are the corpora the differential sweeps, each
// recorded on its own in the expectation so an absent download skips its root.
var structureDifferentialRoots = []struct{ name, requireEnv, fetch string }{
	{name: "examples"},
	{name: "tests/testdata"},
	{name: "examples/sysml-v2-training", requireEnv: "OPENSYSML_REQUIRE_TRAINING_CORPUS", fetch: "./scripts/download-training-examples.sh"},
	{name: "examples/pilot-corpora", requireEnv: "OPENSYSML_REQUIRE_PILOT_CORPORA", fetch: "./scripts/download-pilot-corpora.sh"},
	{name: "examples/spec-examples", requireEnv: "OPENSYSML_REQUIRE_SPEC_EXAMPLES", fetch: "./scripts/download-spec-examples.sh"},
}

const structureDifferentialExpectation = "testdata/structure_differential_expected.txt"

var updateStructureDifferential = flag.Bool("update-structure-differential", false,
	"re-record testdata/structure_differential_expected.txt from this run")

// structureDifferentialRoot is the root a corpus file belongs to: the most
// specific downloaded corpus root, else the committed examples or fixtures.
func structureDifferentialRoot(path string) string {
	slash := filepath.ToSlash(path)
	best := ""
	for _, root := range structureDifferentialRoots {
		if strings.HasPrefix(slash, root.name+"/") && len(root.name) > len(best) {
			best = root.name
		}
	}
	return best
}

func corpusModelPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, root := range []string{"examples", filepath.Join("tests", "testdata")} {
		err := filepath.WalkDir(filepath.Join("..", "..", "..", root), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || (!strings.HasSuffix(path, ".sysml") && !strings.HasSuffix(path, ".kerml")) {
				return nil
			}
			relative, err := filepath.Rel(filepath.Join("..", "..", ".."), path)
			if err != nil {
				return err
			}
			paths = append(paths, relative)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(paths)
	return paths
}

// structureDifferentialTotals are one root's counts: the models, elements and,
// per served property, how the two structures agreed.
type structureDifferentialTotals struct {
	models, skipped, matched, unmatched int
	properties                          map[string]*differentialCounts
}

func (t *structureDifferentialTotals) add(diff *structureDifferential) {
	t.models++
	t.matched += diff.matched
	t.unmatched += len(diff.unmatched)
	for property, counts := range diff.counts {
		total := t.properties[property]
		if total == nil {
			total = &differentialCounts{}
			t.properties[property] = total
		}
		total.equal += counts.equal
		total.different += counts.different
		total.missing += counts.missing
		total.extra += counts.extra
	}
}

// record is the root's expectation block: its sweep counts and every property
// the structures disagree on; a line per property keeps movements reviewable.
func (t *structureDifferentialTotals) record(name string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "root %s: models %d, skipped %d, matched %d, unmatched %d\n", name, t.models, t.skipped, t.matched, t.unmatched)
	properties := make([]string, 0, len(t.properties))
	for property, counts := range t.properties {
		if counts.different > 0 || counts.missing > 0 || counts.extra > 0 {
			properties = append(properties, property)
		}
	}
	sort.Strings(properties)
	for _, property := range properties {
		counts := t.properties[property]
		fmt.Fprintf(&out, "  %s: equal %d, different %d, symbol-missing %d, symbol-extra %d\n",
			property, counts.equal, counts.different, counts.missing, counts.extra)
	}
	return out.String()
}

// expectedStructureDifferential reads the recorded block of each root.
func expectedStructureDifferential(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(structureDifferentialExpectation)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}
		}
		t.Fatal(err)
	}
	blocks := map[string]string{}
	current := ""
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if strings.HasPrefix(line, "root ") {
			current = strings.TrimSpace(strings.TrimSuffix(strings.SplitN(line[len("root "):], ":", 2)[0], ":"))
		}
		if current != "" && strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			blocks[current] += line
		}
	}
	return blocks
}

// TestSymbolStructureMatchesGraphStructure evaluates every served property over
// the graph-backed and the symbol-backed structure for every element of every
// model under examples/ and tests/testdata, and pins per root how they disagree.
func TestSymbolStructureMatchesGraphStructure(t *testing.T) {
	t.Parallel()
	filter := os.Getenv("OPENSYSML_STRUCTURE_DIFFERENTIAL_FILTER")
	var dump *os.File
	if path := os.Getenv("OPENSYSML_STRUCTURE_DIFFERENTIAL_DUMP"); path != "" {
		var err error
		if dump, err = os.Create(path); err != nil {
			t.Fatal(err)
		}
		defer dump.Close()
	}
	totals := map[string]*structureDifferentialTotals{}
	samples := map[string][]string{}
	for _, path := range corpusModelPaths(t) {
		if filter != "" && !strings.Contains(filepath.ToSlash(path), filter) {
			continue
		}
		root := structureDifferentialRoot(path)
		total := totals[root]
		if total == nil {
			total = &structureDifferentialTotals{properties: map[string]*differentialCounts{}}
			totals[root] = total
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "..", path))
		if err != nil {
			t.Fatal(err)
		}
		diff, ok := differentialOf(path, data)
		if !ok {
			total.skipped++
			continue
		}
		total.add(diff)
		for property, lines := range diff.samples {
			for _, line := range lines {
				if len(samples[property]) < 8 {
					samples[property] = append(samples[property], filepath.ToSlash(path)+": "+line)
				}
			}
		}
		if dump != nil {
			sort.Strings(diff.unmatched)
			for _, id := range diff.unmatched {
				fmt.Fprintf(dump, "unmatched %s %s\n", filepath.ToSlash(path), id)
			}
			for property, lines := range diff.samples {
				for _, line := range lines {
					fmt.Fprintf(dump, "%s %s: %s\n", property, filepath.ToSlash(path), line)
				}
			}
		}
	}
	for _, root := range structureDifferentialRoots {
		if totals[root.name] != nil {
			continue
		}
		if os.Getenv(root.requireEnv) != "" && root.requireEnv != "" {
			t.Fatalf("%s is required by %s but absent; run %s", root.name, root.requireEnv, root.fetch)
		}
		t.Logf("%s absent; skipping its root", root.name)
	}
	if filter != "" {
		for name, total := range totals {
			t.Logf("filtered run, not checked against the expectation:\n%s", total.record(name))
		}
		return
	}
	if *updateStructureDifferential {
		recordStructureDifferential(t, totals)
		return
	}
	expected := expectedStructureDifferential(t)
	for _, root := range structureDifferentialRoots {
		total := totals[root.name]
		if total == nil {
			continue
		}
		got := total.record(root.name)
		want, recorded := expected[root.name]
		if !recorded {
			t.Errorf("%s has no recorded expectation; run with -update-structure-differential and review:\n%s", root.name, got)
			continue
		}
		if got == want {
			continue
		}
		t.Errorf("%s moved against %s (review, then re-record with -update-structure-differential):\n%s",
			root.name, structureDifferentialExpectation, structureDifferentialDelta(want, got, samples))
	}
}

// recordStructureDifferential rewrites the expectation, keeping the blocks of
// roots this run did not sweep.
func recordStructureDifferential(t *testing.T, totals map[string]*structureDifferentialTotals) {
	t.Helper()
	expected := expectedStructureDifferential(t)
	var out strings.Builder
	out.WriteString("# Graph-backed versus symbol-backed metamodel.Structure, per corpus root: the\n")
	out.WriteString("# served properties the two disagree on. Re-record with\n")
	out.WriteString("#   go test ./internal/translate/export -run TestSymbolStructureMatchesGraphStructure -update-structure-differential\n")
	out.WriteString("# after adjudicating every movement; the downloaded roots need their corpora present.\n")
	for _, root := range structureDifferentialRoots {
		if total := totals[root.name]; total != nil {
			out.WriteString(total.record(root.name))
		} else if block, ok := expected[root.name]; ok {
			out.WriteString(block)
		}
	}
	if err := os.WriteFile(structureDifferentialExpectation, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// structureDifferentialDelta reports the lines that moved, each with the
// samples this run saw for its property.
func structureDifferentialDelta(want, got string, samples map[string][]string) string {
	wanted := map[string]bool{}
	for _, line := range strings.Split(want, "\n") {
		wanted[line] = true
	}
	gotten := map[string]bool{}
	var out strings.Builder
	for _, line := range strings.Split(got, "\n") {
		gotten[line] = true
		if wanted[line] || strings.TrimSpace(line) == "" {
			continue
		}
		fmt.Fprintf(&out, "+ %s\n", line)
		property := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
		for _, sample := range samples[property] {
			fmt.Fprintf(&out, "      %s\n", sample)
		}
	}
	for _, line := range strings.Split(want, "\n") {
		if !gotten[line] && strings.TrimSpace(line) != "" {
			fmt.Fprintf(&out, "- %s\n", line)
		}
	}
	return out.String()
}

func differentialOf(path string, data []byte) (*structureDifferential, bool) {
	sf := source.New(path, data)
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		return nil, false
	}
	docs := []ModelDocument{{File: sf, Root: root}}
	graph, res, encoders, err := modelToRDF(docs, IDQualifiedName)
	if err != nil || len(encoders) == 0 {
		return nil, false
	}
	ids, err := identityIDsOf(res, encoders, []bool{false})
	if err != nil {
		return nil, false
	}
	model := encoders[0].ids.model
	index := newAPIJSONWriteIndex(graph)
	graphEvaluation := newAPIJSONEvaluationWithIndex(index)
	symbolic := metamodel.NewSymbolStructure(res, model)
	symbolEvaluator := metamodel.New(res, model, metamodel.Options{Structure: symbolic})
	diff := &structureDifferential{
		graph:           graphEvaluation.structure,
		symbolic:        symbolic,
		ids:             ids,
		index:           res.Index(),
		doc:             sf.Name(),
		symbolID:        map[*symbols.Symbol]string{},
		identityToGraph: map[string]string{},
		graphIDs:        map[string]bool{},
		counts:          map[string]*differentialCounts{},
		samples:         map[string][]string{},
	}
	// Reach every symbol from the document root through ownership.
	var symbolsSeen []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	var visit func(sym *symbols.Symbol)
	visit = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		symbolsSeen = append(symbolsSeen, sym)
		for _, rel := range model.OwnedRelationships(sym) {
			visit(rel)
			if related, ok := model.ReflectiveElements(rel, "ownedRelatedElement"); ok {
				for _, owned := range related {
					visit(owned)
				}
			}
		}
	}
	if scope := res.Index().DocumentRoot(sf.Name()); scope != nil {
		for _, member := range scope.Members() {
			visit(member)
		}
	}
	graphByID := map[string]metamodel.Element{}
	for position, subject := range index.subjects {
		if position >= index.settled {
			break
		}
		if LibraryReference(graph, subject) {
			continue
		}
		if id, ok := rdf.SubjectID(subject); ok {
			if element, ok := diff.graph.element(subject); ok {
				graphByID[id] = element
			}
		}
	}
	// Join every symbol to its graph id before comparing anything, so an
	// id-valued property spells the same element the same way on both sides.
	matchedIDs := map[string]bool{}
	var matched []*symbols.Symbol
	for _, sym := range symbolsSeen {
		id, ok := diff.graphID(sym)
		if !ok {
			continue
		}
		if _, ok := graphByID[id]; !ok {
			continue
		}
		matchedIDs[id] = true
		matched = append(matched, sym)
		if effective, ok := symbolic.ElementIdentity(symbolic.ElementFor(sym)); ok {
			diff.identityToGraph[effective] = id
		}
	}
	for id := range graphByID {
		diff.graphIDs[id] = true
		if !matchedIDs[id] {
			diff.unmatched = append(diff.unmatched, id)
		}
	}
	diff.matched = len(matched)
	for _, sym := range matched {
		id, _ := diff.graphID(sym)
		diff.compare(graphEvaluation.evaluator, graphByID[id], symbolEvaluator, symbolic.ElementFor(sym))
	}
	return diff, true
}

// graphID is the elementId the conversion wrote for the element sym reflects.
func (d *structureDifferential) graphID(sym *symbols.Symbol) (string, bool) {
	if id, ok := d.symbolID[sym]; ok {
		return id, id != ""
	}
	id, ok := d.computeGraphID(sym)
	if !ok {
		id = ""
	}
	d.symbolID[sym] = id
	return id, ok
}

func (d *structureDifferential) computeGraphID(sym *symbols.Symbol) (string, bool) {
	switch {
	case sym.Implicit != nil:
		return d.implicitGraphID(sym.Implicit)
	case sym.Expr != nil:
		return d.expressionGraphID(sym)
	}
	if implied, ok := symbols.ImpliedDeclOf(sym); ok && implied.Form == symbols.ImpliedDefaultMultiplicity {
		if element, _, ok := d.ids.OfExpressionPart(d.doc, implied.Of, "multiplicity"); ok {
			return element, true
		}
		owner, ok := d.ids.OfDeclaration(d.doc, implied.Of, "")
		if !ok {
			return "", false
		}
		return rdf.ExpressionNodeID(owner, "multiplicity"), true
	}
	return d.ids.OfDeclaration(d.doc, sym.Decl, d.fqn(sym))
}

// implicitGraphID mirrors how the conversion mints a relationship it writes for
// the notation: a membership from its member, a written clause from its node.
func (d *structureDifferential) implicitGraphID(rel *symbols.ImplicitRelationship) (string, bool) {
	switch rel.Form {
	case symbols.ImplicitMembership:
		if rel.Member == nil {
			return "", false
		}
		if id, ok := d.ids.OwningMembershipOfDeclaration(d.doc, rel.Member.Decl, d.fqn(rel.Member)); ok && rel.Member.Expr == nil {
			return id, true
		}
		member, ok := d.graphID(rel.Member)
		if !ok {
			return "", false
		}
		return member + rdf.OwningMembershipSuffix, true
	case symbols.ImplicitWritten:
		if rel.Node == nil {
			return "", false
		}
		if rel.Node.Span().Len == 0 && rel.Node.Target != nil {
			return d.ids.RelationshipOf(d.doc, rel.Node.Target)
		}
		return d.ids.RelationshipOf(d.doc, rel.Node)
	case symbols.ImplicitImport:
		return d.ids.OfDeclaration(d.doc, rel.Decl, "")
	case symbols.ImplicitReferenceMembership:
		if rel.Owner == nil || rel.Slot == "" {
			return "", false
		}
		_, membership, ok := d.ids.OfExpressionPart(d.doc, rel.Owner.Decl, rel.Slot)
		if !ok && rel.Owner.Expr != nil {
			return d.expressionPartID(rel.Owner, rel.Slot)
		}
		return membership, ok
	case symbols.ImplicitFeatureChaining:
		if rel.Owner == nil || rel.Slot == "" {
			return "", false
		}
		return d.expressionPartID(rel.Owner, rel.Slot)
	}
	return "", false
}

// expressionGraphID mirrors the conversion's expression ids: a node of the
// notation by its node, a part it mints (parameters, results) by its slot.
func (d *structureDifferential) expressionGraphID(sym *symbols.Symbol) (string, bool) {
	owner := sym.Expr.Owner
	if owner == nil {
		return "", false
	}
	switch sym.Expr.Role {
	case symbols.ExpressionParameter:
		return d.expressionPartID(owner, fmt.Sprintf("in%d", sym.Expr.Index))
	case symbols.ExpressionResult:
		return d.expressionPartID(owner, "out")
	case symbols.ExpressionBody:
		if id, ok := d.ids.OfDeclaration(d.doc, sym.Decl, ""); ok {
			return id, true
		}
		return d.expressionPartID(owner, "body")
	case symbols.ExpressionBodyResult:
		return d.expressionPartID(owner, "result")
	case symbols.ExpressionChangeWrapper:
		if owner.Expr == nil || owner.Expr.Owner == nil {
			return "", false
		}
		return d.expressionPartID(owner.Expr.Owner, "a0")
	case symbols.ExpressionChainFeature:
		return d.expressionPartID(owner, sym.Expr.Slot)
	}
	if sym.Decl != nil && sym.Decl != owner.Decl {
		if id, ok := d.ids.OfDeclaration(d.doc, sym.Decl, ""); ok {
			return id, true
		}
	}
	return "", false
}

// expressionPartID is the id of the part the conversion mints at slot under owner.
func (d *structureDifferential) expressionPartID(owner *symbols.Symbol, slot string) (string, bool) {
	if element, _, ok := d.ids.OfExpressionPart(d.doc, owner.Decl, slot); ok && element != "" {
		return element, true
	}
	base, ok := d.graphID(owner)
	if !ok {
		return "", false
	}
	return rdf.ExpressionNodeID(base, slot), true
}

func (d *structureDifferential) compare(graphEvaluator *metamodel.Evaluator, graphElement metamodel.Element,
	symbolEvaluator *metamodel.Evaluator, symbolElement metamodel.Element) {
	class, ok := d.graph.Metaclass(graphElement)
	if !ok {
		return
	}
	for _, rule := range metamodel.Rules() {
		if !d.graph.Specializes(class, rule.DefiningClass) {
			continue
		}
		property := rule.DefiningClass + "::" + rule.Property
		counts := d.counts[property]
		if counts == nil {
			counts = &differentialCounts{}
			d.counts[property] = counts
		}
		d.idProperty = strings.HasSuffix(rule.Property, "Id") || strings.HasSuffix(rule.Property, "Ids")
		want, wantOK := graphEvaluator.PropertyReadOnlyUncached(graphElement, rule.DefiningClass, rule.Property)
		got, gotOK := symbolEvaluator.PropertyReadOnlyUncached(symbolElement, rule.DefiningClass, rule.Property)
		switch {
		case !wantOK && !gotOK:
			counts.equal++
		case wantOK && !gotOK:
			counts.missing++
			d.sample(property, class, graphElement, "graph "+d.spell(want, true)+", symbol unsupported")
		case !wantOK && gotOK:
			counts.extra++
		default:
			left, right := d.spell(want, true), d.spell(got, false)
			if left == right {
				counts.equal++
			} else {
				counts.different++
				d.sample(property, class, graphElement, "graph "+left+", symbol "+right)
			}
		}
	}
}

func (d *structureDifferential) sample(property, class string, element metamodel.Element, line string) {
	if len(d.samples[property]) >= 2 {
		return
	}
	id, _ := d.graph.ElementIdentity(element)
	d.samples[property] = append(d.samples[property], fmt.Sprintf("%s %s: %s", class, id, line))
}

// spell renders a value with elements by their written id, so a graph and a
// symbol value compare by identity rather than by handle.
func (d *structureDifferential) spell(value metamodel.Value, fromGraph bool) string {
	switch value.Kind {
	case metamodel.NullValue:
		return "null"
	case metamodel.BooleanValue:
		return fmt.Sprint(value.Boolean)
	case metamodel.IntegerValue:
		return fmt.Sprint(value.Integer)
	case metamodel.RealValue:
		return fmt.Sprint(value.Real)
	case metamodel.StringValue:
		if !d.idProperty {
			return fmt.Sprintf("%q", value.String)
		}
		if fromGraph && d.graphIDs[value.String] {
			return "#" + value.String
		}
		if id, ok := d.identityToGraph[value.String]; !fromGraph && ok {
			return "#" + id
		}
		return fmt.Sprintf("%q", value.String)
	case metamodel.EnumValue:
		return "enum " + value.Enum
	case metamodel.ElementValue:
		return d.spellElement(value.Element, fromGraph)
	case metamodel.MembershipValue:
		return d.spellElement(metamodel.MembershipElement(value.Membership), fromGraph)
	case metamodel.SequenceValue:
		parts := make([]string, 0, len(value.Values))
		for _, member := range value.Values {
			parts = append(parts, d.spell(member, fromGraph))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return "invalid"
}

func (d *structureDifferential) spellElement(element metamodel.Element, fromGraph bool) string {
	if element.Symbol != nil && !element.IsMembership {
		return "lib " + d.fqn(element.Symbol)
	}
	structure := metamodel.Structure(d.symbolic)
	if fromGraph {
		structure = d.graph
	}
	if element.IsMembership {
		if members, ok := structure.RelatedElements(element, "memberElement"); ok && len(members) == 1 {
			if id, ok := d.elementID(element, fromGraph); ok {
				return "#" + id
			}
			return "membership of " + d.spellElement(members[0], fromGraph)
		}
	}
	if fromGraph {
		if id, ok := d.graph.ElementIdentity(element); ok {
			return "#" + id
		}
		if sym, ok := d.graph.LibrarySymbol(element); ok {
			return "lib " + d.fqn(sym)
		}
		return "element?"
	}
	if sym, ok := d.symbolic.SymbolOf(element); ok {
		if id, ok := d.graphID(sym); ok {
			return "#" + id
		}
		if d.index.Library(sym) {
			return "lib " + d.fqn(sym)
		}
		return "unmapped " + d.fqn(sym)
	}
	return "element?"
}

func (d *structureDifferential) fqn(sym *symbols.Symbol) string {
	if d.index == nil || sym == nil {
		return ""
	}
	return d.index.GetFQN(sym)
}

// elementID is the written id of an element of either structure, if it has one.
func (d *structureDifferential) elementID(element metamodel.Element, fromGraph bool) (string, bool) {
	if fromGraph {
		return d.graph.ElementIdentity(element)
	}
	if sym, ok := d.symbolic.SymbolOf(element); ok {
		return d.graphID(sym)
	}
	return "", false
}
