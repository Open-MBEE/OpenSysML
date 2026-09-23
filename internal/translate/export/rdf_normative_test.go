package export

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func normativeGraph(t *testing.T, src string) *rdf.Graph {
	t.Helper()
	file := source.New("n.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	return graph
}

func objects(graph *rdf.Graph, subject, predicate string) []rdf.Term {
	return graph.Objects(rdf.IRI(subject), rdf.SysML+predicate)
}

func elmt(name string) rdf.Term { return rdf.IRI("urn:sysmlv2:element:" + name) }

func meta(graph *rdf.Graph, t rdf.Term) string { return rdf.LocalName(graph.Type(t)) }

// jsonValues flattens a single value or an array of values.
func jsonValues(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}

// assertEnds checks a materialized relationship element: its class, its
// identity, its owner, and that the listed end properties reach the owner and
// the collapsed target.
func assertEnds(t *testing.T, graph *rdf.Graph, rel rdf.Term, metaclass string, owner, target rdf.Term, sourceEnds, targetEnds []string) {
	t.Helper()
	if got := meta(graph, rel); got != metaclass {
		t.Fatalf("%s: want %s, got %s", rel.Value, metaclass, got)
	}
	if id := graph.Objects(rel, rdf.SysML+pElementID); len(id) != 1 || id[0].Value != rdf.LocalName(rel.Value) {
		t.Fatalf("%s: elementId %v", rel.Value, id)
	}
	for _, property := range append(append([]string{pOwner, pRelatedElement, "specific", "source"}, sourceEnds...), "owningRelatedElement") {
		found := false
		for _, o := range graph.Objects(rel, rdf.SysML+property) {
			found = found || o.Value == owner.Value
		}
		if !found {
			t.Fatalf("%s: %s does not reach %s", rel.Value, property, owner.Value)
		}
	}
	for _, property := range append(targetEnds, pTarget, "general") {
		found := false
		for _, o := range graph.Objects(rel, rdf.SysML+property) {
			found = found || o.Value == target.Value
		}
		if !found {
			t.Fatalf("%s: %s does not reach %s", rel.Value, property, target.Value)
		}
	}
}

// TestNormativeRelationshipElementsAreMaterialized covers each element the
// collapsed relationship properties now grow.
func TestNormativeRelationshipElementsAreMaterialized(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part def Vehicle;
		part def Car :> Vehicle;
		part baseWheels;
		part car : Car {
			part wheels :>> baseWheels;
			part engine [0..*];
			part spare [4];
			part extra :> wheels;
			ref part link ::> engine;
		}
		port def P;
		requirement def R;
		requirement R1 : R {
			subject v : Car;
			require constraint { v != null }
		}
		satisfy R1 by car;
	}`)
	car := elmt("N__car")
	// `part car : Car`: one FeatureTyping whose ends restate the collapsed type.
	typings := objects(graph, car.Value, "ownedTyping")
	if len(typings) != 1 {
		t.Fatalf("car ownedTyping %v", typings)
	}
	target := objects(graph, car.Value, "type")[0]
	assertEnds(t, graph, typings[0], "FeatureTyping", car, target,
		[]string{"typedFeature", "owningFeature"}, []string{"type"})
	for _, want := range []struct{ owner, prop, metaclass, collapsed string }{
		{"N__Car", "ownedSubclassification", "Subclassification", "specializes"},
		{"N__car__wheels", "ownedRedefinition", "Redefinition", "redefines"},
		{"N__car__extra", "ownedSubsetting", "Subsetting", "subsets"},
		{"N__car__link", "ownedReferenceSubsetting", "ReferenceSubsetting", "references"},
	} {
		owner := elmt(want.owner)
		rels := objects(graph, owner.Value, want.prop)
		if len(rels) != 1 || meta(graph, rels[0]) != want.metaclass {
			t.Fatalf("%s %s = %v", want.owner, want.prop, rels)
		}
		stated := objects(graph, owner.Value, want.collapsed)
		ends := objects(graph, rels[0].Value, pTarget)
		if len(ends) == 0 || len(stated) == 0 || ends[0].Value != stated[0].Value {
			t.Fatalf("%s: ends %v vs stated %v", rels[0].Value, ends, stated)
		}
	}
	// MultiplicityRange: [0..*] on engine, [4] on spare; each bound restated.
	for _, use := range []string{"N__car__engine", "N__car__spare"} {
		mults := objects(graph, "urn:sysmlv2:element:"+use, "multiplicity")
		if len(mults) != 1 || meta(graph, mults[0]) != "MultiplicityRange" {
			t.Fatalf("%s multiplicity %v", use, mults)
		}
		bounds := 0
		for _, bound := range []string{"lowerBound", "upperBound"} {
			stated := objects(graph, "urn:sysmlv2:element:"+use, bound)
			ranged := objects(graph, mults[0].Value, bound)
			if len(stated) != len(ranged) || (len(stated) > 0 && stated[0].Value != ranged[0].Value) {
				t.Fatalf("%s %s: collapsed %v vs range %v", use, bound, stated, ranged)
			}
			bounds += len(stated)
		}
		if bounds == 0 {
			t.Fatalf("%s: no bounds on usage or range", use)
		}
	}
	// port def P gains a ConjugatedPortDefinition owning a PortConjugation.
	p := elmt("N__P")
	conjs := objects(graph, p.Value, "conjugatedPortDefinition")
	if len(conjs) != 1 || meta(graph, conjs[0]) != "ConjugatedPortDefinition" {
		t.Fatalf("P conjugatedPortDefinition %v", conjs)
	}
	if name := objects(graph, conjs[0].Value, "declaredName"); len(name) != 1 || name[0].Value != "~P" {
		t.Fatalf("conjugated declaredName %v", name)
	}
	pcs := objects(graph, conjs[0].Value, "ownedPortConjugator")
	if len(pcs) != 1 || meta(graph, pcs[0]) != "PortConjugation" {
		t.Fatalf("conjugated ownedPortConjugator %v", pcs)
	}
	if orig := objects(graph, pcs[0].Value, "originalPortDefinition"); len(orig) != 1 || orig[0].Value != p.Value {
		t.Fatalf("PortConjugation originalPortDefinition %v", orig)
	}
	// `subject v` is a ReferenceUsage under a SubjectMembership.
	v := elmt("N__R1__v")
	if meta(graph, v) != "ReferenceUsage" {
		t.Fatalf("subject v metaclass %s", graph.Type(v))
	}
	ms := graph.Objects(v, rdf.SysML+pOwningMembership)
	if len(ms) != 1 || meta(graph, ms[0]) != "SubjectMembership" {
		t.Fatalf("v owningMembership %v", ms)
	}
	if param := objects(graph, ms[0].Value, "ownedSubjectParameter"); len(param) != 1 || param[0].Value != v.Value {
		t.Fatalf("SubjectMembership ownedSubjectParameter %v", param)
	}
	// satisfy's `_subject` chain mirrors the collapsed subject.
	var sat rdf.Term
	for _, e := range graph.Subjects() {
		if meta(graph, e) == "SatisfyRequirementUsage" {
			sat = e
		}
	}
	if len(objects(graph, sat.Value, "subject")) == 0 {
		t.Fatalf("satisfy states no subject")
	}
	chain := objects(graph, sat.Value, "ownedSubjectParameter")
	if len(chain) == 0 {
		sms := 0
		for _, e := range graph.Subjects() {
			if meta(graph, e) == "SubjectMembership" && len(objects(graph, e.Value, "ownedSubjectParameter")) > 0 {
				if o := objects(graph, e.Value, pOwner); len(o) == 1 && o[0].Value == sat.Value {
					sms++
				}
			}
		}
		if sms == 0 {
			t.Fatalf("satisfy owns no SubjectMembership")
		}
	}
	// `require constraint` is a ConstraintUsage under a
	// RequirementConstraintMembership of kind requirement.
	var kinds []string
	for _, e := range graph.Subjects() {
		if meta(graph, e) == "RequirementConstraintMembership" {
			if k := objects(graph, e.Value, "kind"); len(k) == 1 {
				kinds = append(kinds, k[0].Value)
			}
		}
	}
	if len(kinds) != 1 || kinds[0] != "requirement" {
		t.Fatalf("RequirementConstraintMembership kinds %v", kinds)
	}
}

// TestNormativeAPIJSONHasNoExtensionMetaclasses asserts the API element form
// carries no sysx: @type and spells name-literal targets as {"@ref"}.
func TestNormativeAPIJSONHasNoExtensionMetaclasses(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part p : Missing;
	}`)
	data, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(data, &elements); err != nil {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, raw := range doc["elements"].([]any) {
			elements = append(elements, raw.(map[string]any))
		}
	}
	ref := false
	for _, el := range elements {
		if tpe, _ := el["@type"].(string); strings.HasPrefix(tpe, "sysx:") {
			t.Fatalf("extension metaclass in API JSON: %v", el)
		}
		if el["@type"] == "PartUsage" {
			for _, v := range jsonValues(el["type"]) {
				if m, ok := v.(map[string]any); ok && m["@ref"] == "Missing" {
					ref = true
				}
			}
		}
	}
	if !ref {
		t.Fatalf("no PartUsage carries the literal type as @ref: %s", data)
	}
	back, err := ReadAPIJSON(data)
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	found := false
	for _, o := range back.Objects(elmt("N__p"), rdf.SysML+"type") {
		found = found || o.Value == "Missing"
	}
	if !found {
		t.Fatalf("the API JSON round trip lost the literal type")
	}
}

// TestNormativeVerifyRejectsDisagreement: a materialized element whose ends
// disagree with the collapsed property it restates is refused.
func TestNormativeVerifyRejectsDisagreement(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part def A;
		part def B;
		part p : A;
	}`)
	p := elmt("N__p")
	b := elmt("N__B")
	fts := objects(graph, p.Value, "ownedTyping")
	if len(fts) != 1 {
		t.Fatalf("ownedTyping %v", fts)
	}
	graph.Add(fts[0], rdf.IRI(rdf.SysML+"type"), b)
	if _, err := ToSysML(graph); err == nil || !strings.Contains(err.Error(), b.Value) {
		t.Fatalf("want a refusal naming %s, got %v", b.Value, err)
	}
}

// TestNormativeMultiplicityRangeVerifyRejectsDisagreement.
func TestNormativeMultiplicityRangeVerifyRejectsDisagreement(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part p [4];
	}`)
	p := elmt("N__p")
	mults := objects(graph, p.Value, "multiplicity")
	if len(mults) != 1 {
		t.Fatalf("multiplicity %v", mults)
	}
	graph.Add(mults[0], rdf.IRI(rdf.SysML+"lowerBound"), rdf.String("9"))
	if _, err := ToSysML(graph); err == nil {
		t.Fatalf("want a refusal on the disagreeing bound")
	}
}

// TestNormativeReferentMembershipVerifyRejectsDisagreement.
func TestNormativeReferentMembershipVerifyRejectsDisagreement(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part p;
		part q = p;
	}`)
	var ms rdf.Term
	for _, e := range graph.Subjects() {
		if meta(graph, e) == "Membership" && strings.HasSuffix(e.Value, "_preferent") {
			ms = e
		}
	}
	if ms.Value == "" {
		t.Fatalf("no referent membership minted")
	}
	mut := rdf.NewGraph()
	for _, tr := range graph.Triples() {
		if tr.Subject == ms && tr.Predicate.Value == rdf.SysML+pMemberElement {
			continue
		}
		mut.AddTriple(tr)
	}
	mut.Add(ms, rdf.IRI(rdf.SysML+pMemberElement), elmt("N__other"))
	if _, err := ToSysML(mut); err == nil {
		t.Fatalf("want a refusal on the disagreeing memberElement")
	}
}

// TestNormativeVerifyRejectsSwappedKinds: a materialized element restating a
// target the head states under a different collapsed kind is refused — the
// collapsed write would move the target across kinds.
func TestNormativeVerifyRejectsSwappedKinds(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part def T;
		part f1 : T;
		part f : T subsets f1;
	}`)
	f := elmt("N__f")
	f1 := elmt("N__f1")
	tdef := elmt("N__T")
	fts := objects(graph, f.Value, "ownedTyping")
	sss := objects(graph, f.Value, "ownedSubsetting")
	if len(fts) != 1 || len(sss) != 1 {
		t.Fatalf("typing %v subsetting %v", fts, sss)
	}
	ft, ss := fts[0], sss[0]
	mut := rdf.NewGraph()
	swap := func(t rdf.Triple) rdf.Triple {
		switch {
		case t.Subject == ft && (t.Predicate.Value == rdf.SysML+"type" || t.Predicate.Value == rdf.SysML+"general" || t.Predicate.Value == rdf.SysML+"target") && t.Object.Value == tdef.Value:
			return rdf.Triple{Subject: ft, Predicate: t.Predicate, Object: f1}
		case t.Subject == ss && (t.Predicate.Value == rdf.SysML+"subsettedFeature" || t.Predicate.Value == rdf.SysML+"general" || t.Predicate.Value == rdf.SysML+"target") && t.Object.Value == f1.Value:
			return rdf.Triple{Subject: ss, Predicate: t.Predicate, Object: tdef}
		}
		return t
	}
	for _, tr := range graph.Triples() {
		mut.AddTriple(swap(tr))
	}
	var uerr *UnsupportedError
	if err := mustDecode(mut); err == nil || !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedError on the swapped kinds, got %v", err)
	}
}

// TestNormativeVerifyCoveredRejectsMissingKind: once a head carries
// materialized elements, every IRI target of each collapsed kind must be
// carried by one whose metaclass maps to that kind.
func TestNormativeVerifyCoveredRejectsMissingKind(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part def T;
		part f1 : T;
		part f : T subsets f1;
	}`)
	if err := mustDecode(graph); err != nil {
		t.Fatalf("the unmutated graph must still decode: %v", err)
	}
	sss := objects(graph, elmt("N__f").Value, "ownedSubsetting")
	if len(sss) != 1 {
		t.Fatalf("ownedSubsetting %v", sss)
	}
	mut := rdf.NewGraph()
	for _, tr := range graph.Triples() {
		if tr.Subject == sss[0] {
			continue
		}
		mut.AddTriple(tr)
	}
	var uerr *UnsupportedError
	if err := mustDecode(mut); err == nil || !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedError on the uncovered subsets target, got %v", err)
	}
}

// TestNormativeMultiplicityRangeVerifyRejectsOwnerOnlyBound: a bound the
// feature states that the range does not carry is refused in the other
// direction.
func TestNormativeMultiplicityRangeVerifyRejectsOwnerOnlyBound(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part p [4];
	}`)
	graph.Add(elmt("N__p"), rdf.IRI(rdf.SysML+"lowerBound"), rdf.String("9"))
	if err := mustDecode(graph); err == nil || !strings.Contains(err.Error(), "9") {
		t.Fatalf("want a refusal on the bound only the feature states, got %v", err)
	}
}

func mustDecode(graph *rdf.Graph) error {
	_, err := ToSysML(graph)
	return err
}

// TestNormativeVerifyRejectsLiteralMismatchedEnd: a collapsed literal target is
// stated by its value, so an element end naming a different concrete target is
// refused; the same literal it states decodes.
func TestNormativeVerifyRejectsLiteralMismatchedEnd(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part p : Missing;
	}`)
	p := elmt("N__p")
	fts := objects(graph, p.Value, "ownedTyping")
	if len(fts) != 1 {
		t.Fatalf("ownedTyping %v", fts)
	}
	// The unmutated graph — the literal "Missing" both sides state — decodes.
	if err := mustDecode(graph); err != nil {
		t.Fatalf("the literal-matched graph must decode: %v", err)
	}
	mut := rdf.NewGraph()
	for _, tr := range graph.Triples() {
		if tr.Subject == fts[0] && tr.Predicate.Value == rdf.SysML+"type" {
			continue
		}
		mut.AddTriple(tr)
	}
	mut.Add(fts[0], rdf.IRI(rdf.SysML+"type"), elmt("N__Other"))
	var uerr *UnsupportedError
	if err := mustDecode(mut); err == nil || !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedError on the differently named end, got %v", err)
	}
}

// TestNormativeVerifyCoveredConsumesOneKindPerChild: a Subsetting maps to both
// subsets and specializes, but covers a stated target under one kind only.
func TestNormativeVerifyCoveredConsumesOneKindPerChild(t *testing.T) {
	graph := normativeGraph(t, `package N {
		part def T;
		part f1 : T;
		part f : T subsets f1 specializes f1;
	}`)
	if err := mustDecode(graph); err != nil {
		t.Fatalf("the two-kind graph must decode: %v", err)
	}
	f := elmt("N__f")
	sss := objects(graph, f.Value, "ownedSubsetting")
	if len(sss) != 2 {
		t.Fatalf("want one Subsetting per kind, got %v", sss)
	}
	mut := rdf.NewGraph()
	for _, tr := range graph.Triples() {
		if tr.Subject == sss[1] {
			continue
		}
		mut.AddTriple(tr)
	}
	var uerr *UnsupportedError
	if err := mustDecode(mut); err == nil || !errors.As(err, &uerr) {
		t.Fatalf("want an UnsupportedError on the uncovered kind, got %v", err)
	}
}

// TestReferentMembershipIDCollisionIsRefused: a minted referent membership's
// element-namespace id is claimed, so a declared id landing on it is refused
// rather than merged.
func TestReferentMembershipIDCollisionIsRefused(t *testing.T) {
	e := &encoder{
		graph:    rdf.NewGraph(),
		subjects: map[string]string{},
		ids: &identityFacts{
			form:    IDQualifiedName,
			pkgOf:   map[string]string{},
			localOf: map[string]string{},
		},
	}
	fre := rdf.ElementIRIForID("N__x")
	e.graph.Add(fre, rdf.IRI(rdf.RDFType), rdf.IRI(rdf.SysML+mFeatureReference))
	e.graph.Add(fre, rdf.IRI(rdf.SysML+pReferent), elmt("N__p"))
	e.claim("urn:sysmlv2:element:N__x_referent", "N::d")
	e.materializeReferentMemberships(fre)
	if e.idErr == nil {
		t.Fatal("the id collision on the minted membership was not refused")
	}
	var uerr *UnsupportedError
	if !errors.As(e.idErr, &uerr) {
		t.Fatalf("want an UnsupportedError, got %v", e.idErr)
	}
}
