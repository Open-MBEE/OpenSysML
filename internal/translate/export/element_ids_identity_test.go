package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestElementIDsByIdentityAreTheWrittenIDs: the id minted for a declaration
// without writing is the elementId the written graph carries for it.
func TestElementIDsByIdentityAreTheWrittenIDs(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*.sysml"))
	texts := map[string]string{"dup.sysml": readerModel, "then.sysml": thenDoneModel, "states.sysml": stateActionsModel}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		texts[filepath.Base(f)] = string(data)
	}
	compared, ends, memberships, relationships := 0, 0, 0, 0
	for name, text := range texts {
		sf := source.New(name, []byte(text))
		p := parser.New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			continue
		}
		docs := []ModelDocument{{File: sf, Root: root}}
		graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
		if err != nil {
			continue
		}
		minted, err := NewElementIDs(docs)
		if err != nil {
			t.Fatalf("%s: NewElementIDs: %v", name, err)
		}
		for _, e := range encoders {
			for node, fqn := range e.fqn {
				written, ok := graph.Lexical(e.ids.subjectForNode(node, fqn), rdf.SysML+pElementID)
				got, has := minted.bySpan[spanKey{name, node.Span()}]
				if has != ok || got != written {
					t.Errorf("%s: %s minted %q (%v), written %q (%v)", name, fqn, got, has, written, ok)
				}
				compared++
				// A member's owning membership is written as a subject of its own.
				membership, membershipWritten := graph.Object(e.ids.subjectForNode(node, fqn), rdf.SysML+pOwningMembership)
				mintedMembership, hasMembership := minted.membershipBySpan[spanKey{name, node.Span()}]
				if membershipWritten || hasMembership {
					if hasMembership != membershipWritten || mintedMembership != rdf.LocalName(membership.Value) {
						t.Errorf("%s: %s owning membership minted %q (%v), written %q (%v)", name, fqn, mintedMembership, hasMembership, membership.Value, membershipWritten)
					}
					memberships++
				}
				usage, isUsage := node.(*ast.Usage)
				if !isUsage {
					continue
				}
				// The ends a head binds are written as end features of their own.
				subject := e.ids.subjectForNode(node, fqn)
				for i, end := range usage.ConnectorEnds {
					if end == nil {
						continue
					}
					feature := rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i))
					written, ok := graph.Lexical(feature, rdf.SysML+pElementID)
					got, has := minted.bySpan[spanKey{name, end.Span()}]
					if has != ok || got != written {
						t.Errorf("%s: %s end %d minted %q (%v), written %q (%v)", name, fqn, i, got, has, written, ok)
					}
					membership, membershipWritten := graph.Object(feature, rdf.SysML+pOwningMembership)
					mintedMembership, hasMembership := minted.membershipBySpan[spanKey{name, end.Span()}]
					if hasMembership != membershipWritten || (membershipWritten && mintedMembership != rdf.LocalName(membership.Value)) {
						t.Errorf("%s: %s end %d owning membership minted %q (%v), written %q (%v)", name, fqn, i, mintedMembership, hasMembership, membership.Value, membershipWritten)
					}
					ends++
				}
			}
		}
		// The relationship elements the notation implies — head clauses, ends' reference
		// subsettings, prefix-metadata annotations — are every owned relationship written
		// that is no membership or import; the reader states exactly those.
		relationships += compareWrittenRelationships(t, name, graph, encoders, minted)
	}
	if compared == 0 || ends == 0 || memberships == 0 || relationships == 0 {
		t.Fatalf("%d declarations, %d connector ends, %d owning memberships and %d relationships compared, want all", compared, ends, memberships, relationships)
	}
}

// compareWrittenRelationships checks that the ids minted for document name's implied
// relationships (ElementIDs.relationshipBySpan) are exactly the owned relationships the
// graph writes under its declarations and their bound ends, memberships and imports aside.
func compareWrittenRelationships(t *testing.T, name string, graph *rdf.Graph, encoders []*encoder, minted *ElementIDs) int {
	t.Helper()
	written := map[string]bool{}
	for _, e := range encoders {
		subjects := []rdf.Term{}
		for node, fqn := range e.fqn {
			subject := e.ids.subjectForNode(node, fqn)
			subjects = append(subjects, subject)
			if usage, ok := node.(*ast.Usage); ok {
				for i, end := range usage.ConnectorEnds {
					if end != nil {
						subjects = append(subjects, rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i)))
					}
				}
			}
		}
		for _, subject := range subjects {
			aside := map[string]bool{}
			for _, property := range []string{pOwnedMembership, pOwnedImport} {
				for _, o := range graph.Objects(subject, rdf.SysML+property) {
					aside[rdf.LocalName(o.Value)] = true
				}
			}
			for _, o := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
				if id := rdf.LocalName(o.Value); !aside[id] {
					written[id] = true
				}
			}
		}
	}
	stated := map[string]bool{}
	for key, id := range minted.relationshipBySpan {
		if key.doc == name {
			stated[id] = true
		}
	}
	for id := range written {
		if !stated[id] {
			t.Errorf("%s: the writer writes relationship %s, the reader states no id for it", name, id)
		}
	}
	for id := range stated {
		if !written[id] {
			t.Errorf("%s: the reader states relationship %s, the writer writes none", name, id)
		}
	}
	return len(written)
}

// writtenRelationshipsModel writes every relationship the identity reader mints: a
// definition's and a usage's specializations, two typings, subsets, redefines, a satisfy's
// requirement (reference form), bound ends, a dependency's prefix metadata, a metadata body's
// implicit redefinition, variant references, an import.
const writtenRelationshipsModel = "package R {\n\tprivate import ScalarValues::*;\n\tpart def Base;\n\tpart def Derived :> Base;\n\tport def P;\n\tmetadata def M;\n\trequirement def Req;\n\tpart def Sys {\n\t\tattribute m : Real;\n\t\tpart a : Base, Derived;\n\t\tpart b :> a;\n\t\tpart d specializes a;\n\t\tpart c subsets a redefines b;\n\t\tport p : P;\n\t\tport q : ~P;\n\t\tconnect a to b;\n\t\tsatisfy Req by a;\n\t\t#M dependency from a to b;\n\t\t@H { flown = true; }\n\t\tpart v : Base { variant b; variant a; }\n\t}\n\tmetadata def H { attribute flown : Boolean; }\n}\n"

// TestElementIDReaderMintsEveryWrittenRelationship: each form of implied relationship
// reads through the reader with the id the writer gives it, keyed by the node it is written for.
func TestElementIDReaderMintsEveryWrittenRelationship(t *testing.T) {
	sf, _, doc := readerIndex(t, "r.sysml", writtenRelationshipsModel)
	docs := []ModelDocument{{File: sf, Root: doc.Root}}
	graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
	if err != nil {
		t.Fatalf("modelToRDF: %v", err)
	}
	minted, err := NewElementIDs(docs)
	if err != nil {
		t.Fatalf("NewElementIDs: %v", err)
	}
	if n := compareWrittenRelationships(t, "r.sysml", graph, encoders, minted); n < 10 {
		t.Fatalf("%d relationships compared, want the fixture's", n)
	}
	reader := NewElementIDReader([]ElementIDDocument{{File: sf, Root: doc.Root}})
	suffixes := map[string]int{}
	for _, e := range encoders {
		for node, fqn := range e.fqn {
			for key, want := range e.writtenRelationships(node, e.parentDeclarations()[node], e.ids.subjectForNode(node, fqn), e.parentDeclarations()) {
				got, err := reader.ReadRelationship("r.sysml", key)
				if err != nil || got != want {
					t.Errorf("%s: relationship at %v read %q (%v), minted %q", fqn, key.Span(), got, err, want)
				}
				suffixes[want[strings.LastIndex(want, "_"):]]++
			}
		}
	}
	for _, suffix := range []string{"_sc0", "_ft0", "_ft1", "_sp0", "_ss0", "_rd0", "_rs", "_prs", "_an"} {
		if suffixes[suffix] == 0 {
			t.Errorf("no relationship read under %s; read %v", suffix, suffixes)
		}
	}
}

const thenDoneModel = "package P {\n\taction def A {\n\t\taction a;\n\t\tfirst start then a;\n\t\tthen done;\n\t}\n}\n"

// TestElementIDReaderWritesNoIDForAnAbsorbedFinalNode: the final node `then
// done;` introduces is written as its succession's end, so it has no id of its own.
func TestElementIDReaderWritesNoIDForAnAbsorbedFinalNode(t *testing.T) {
	sf, _, doc := readerIndex(t, "then.sysml", thenDoneModel)
	e, err := newEncoder(sf, doc.Root, "", IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	absorbed := 0
	for node := range e.absorbed {
		absorbed++
		if id, ok := NewElementIDReader([]ElementIDDocument{doc}).Read("then.sysml", node, e.fqn[node]); ok != nil || id != "" {
			t.Fatalf("the absorbed final node reads id %q (%v), want none", id, ok)
		}
	}
	if absorbed != 1 {
		t.Fatalf("%d absorbed final nodes, want 1", absorbed)
	}
}

// TestElementIDReaderRefusesCollidingIDs: ids an author declares that land on one
// IRI, or on another element's membership, refuse the model as Convert does.
func TestElementIDReaderRefusesCollidingIDs(t *testing.T) {
	for name, text := range map[string]string{
		"shared":     "package P {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"shared\"; } }\n\tpart def B { @IdentityMetadata::ElementId { id = \"shared\"; } }\n}\n",
		"membership": "package P {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"stable\"; } }\n\tpart def B { @IdentityMetadata::ElementId { id = \"stable_om\"; } }\n}\n",
	} {
		_, _, doc := readerIndex(t, name+".sysml", text)
		_, err := NewElementIDReader([]ElementIDDocument{doc}).Read("", nil, "P::A")
		if err == nil || !strings.Contains(err.Error(), "lands on the same IRI") {
			t.Fatalf("%s: err = %v, want the collision refused", name, err)
		}
		_, err = NewElementIDReader([]ElementIDDocument{doc, {File: source.New("other.sysml", []byte("package Q { part def C; }\n"))}}).Read("", nil, "P::A")
		if err == nil || !strings.Contains(err.Error(), "lands on the same IRI") {
			t.Fatalf("%s, with another document: err = %v, want the collision refused", name, err)
		}
	}
}

// TestElementIDReaderClaimsWhatTheWriterClaims: a root claims no owning membership, and
// a referenced document's declarations are checked across documents only (modelToRDF).
func TestElementIDReaderClaimsWhatTheWriterClaims(t *testing.T) {
	_, _, root := readerIndex(t, "root.sysml", "package P {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\t@IdentityMetadata::ElementId { id = \"stable\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"stable_om\"; } }\n}\n")
	if _, err := NewElementIDReader([]ElementIDDocument{root}).Read("", nil, "P::A"); err != nil {
		t.Fatalf("a member id on a root's would-be membership IRI was refused: %v", err)
	}
	_, _, written := readerIndex(t, "w.sysml", "package W { part def C; }\n")
	_, _, referenced := readerIndex(t, "r.sysml", "package R {\n\t@IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n\tpart def A { @IdentityMetadata::ElementId { id = \"shared\"; } }\n\tpart def B { @IdentityMetadata::ElementId { id = \"shared\"; } }\n}\n")
	docs := []ModelDocument{{File: written.File, Root: written.Root}, {File: referenced.File, Root: referenced.Root, Referenced: true}}
	if _, _, _, err := modelToRDF(docs, IDQualifiedName); err != nil {
		t.Fatalf("the writer refuses the model the test assumes it accepts: %v", err)
	}
	if _, err := NewElementIDs(docs); err != nil {
		t.Fatalf("a referenced document's repeated id was refused, where the writer writes the model: %v", err)
	}
}

const boundEndsModel = `package P {
	part def D { port p; attribute x; }
	part a : D; part b : D;
	connect a to b;
	connection named connect a to b;
	interface i connect a.p to b.p;
	bind a.x = b.x;
	action def A {
		action x; action y;
		succession first x then y;
	}
}
`

// TestElementIDReaderMintsEveryBoundEnd: each form of head that binds ends — an
// anonymous and a named connection, an interface, a binding, a succession — has
// every end's id minted as the writer writes it.
func TestElementIDReaderMintsEveryBoundEnd(t *testing.T) {
	sf, _, doc := readerIndex(t, "ends.sysml", boundEndsModel)
	docs := []ModelDocument{{File: sf, Root: doc.Root}}
	graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
	if err != nil {
		t.Fatalf("modelToRDF: %v", err)
	}
	minted, err := NewElementIDs(docs)
	if err != nil {
		t.Fatalf("NewElementIDs: %v", err)
	}
	forms := map[string]int{}
	for _, e := range encoders {
		for node, fqn := range e.fqn {
			usage, ok := node.(*ast.Usage)
			if !ok || len(usage.ConnectorEnds) == 0 {
				continue
			}
			subject := e.ids.subjectForNode(node, fqn)
			for i, end := range usage.ConnectorEnds {
				written, ok := graph.Lexical(rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i)), rdf.SysML+pElementID)
				if !ok || written == "" {
					t.Errorf("%s end %d: the writer states no elementId", fqn, i)
				}
				got, has := minted.OfDeclaration("ends.sysml", end, "")
				if !has || got != written {
					t.Errorf("%s end %d: minted %q (%v), written %q", fqn, i, got, has, written)
				}
				forms[usage.Keyword]++
			}
		}
	}
	for _, form := range []string{"connect", "connection", "interface", "bind", "succession"} {
		if forms[form] < 2 {
			t.Errorf("%s: %d ends compared, want both", form, forms[form])
		}
	}
}

// stateActionsModel declares entry, exit and do actions of a state, in the inline and
// the named forms: their owning membership is the entry/exit/do member itself.
const stateActionsModel = "package S {\n\tstate def D;\n\tstate s : D {\n\t\tentry action start;\n\t\tdo action run { action step; }\n\t\texit action stop;\n\t\tentry; then idle;\n\t\tstate idle;\n\t}\n}\n"

// TestElementIDReaderOwningMembershipOfALibraryElement: a standard library element the
// model only references has an owning membership id too — the one the writer gives it.
func TestElementIDReaderOwningMembershipOfALibraryElement(t *testing.T) {
	sf, _, doc := readerIndex(t, "lib.sysml", "package M {\n\tprivate import ScalarValues::*;\n\tattribute x : Real;\n}\n")
	reader := NewElementIDReader([]ElementIDDocument{{File: sf, Root: doc.Root}})
	got, err := reader.ReadOwningMembership("", nil, "ScalarValues::Real")
	if err != nil {
		t.Fatal(err)
	}
	element, ok := reader.ids.Of("ScalarValues::Real")
	if !ok || got == "" {
		t.Fatalf("library element id %q (%v), owning membership %q, want both", element, ok, got)
	}
	facts := reader.ids.encoders[0].ids
	var want string
	for _, sym := range reader.ids.res.Index().LookupQualified("ScalarValues::Real") {
		if libraryFQN, ok := facts.libraryElement(sym); ok {
			want = rdf.LocalName(facts.owningMembershipOf(sym.Decl, facts.subjectForNode(sym.Decl, libraryFQN)).Value)
		}
	}
	if got != want {
		t.Errorf("owning membership = %q, want the writer's %q", got, want)
	}
	if root, _ := reader.ReadOwningMembership("", nil, "ScalarValues"); root != "" {
		t.Errorf("a library root has an owning membership %q, want none", root)
	}
}
