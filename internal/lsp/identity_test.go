package lsp

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// uuidV4 matches a lowercase RFC 4122 version-4 UUID.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// cursorAt is the empty range at the first occurrence of marker in src.
func cursorAt(t *testing.T, src, marker string) protocol.Range {
	t.Helper()
	off := strings.Index(src, marker)
	if off < 0 {
		t.Fatalf("marker %q not in source", marker)
	}
	pos := offsetToPosition([]byte(src), off)
	return protocol.Range{Start: pos, End: pos}
}

// identityActionsFor returns the identity annotation actions offered at rng.
func identityActionsFor(t *testing.T, file, src string, rng protocol.Range) []protocol.CodeAction {
	t.Helper()
	var out []protocol.CodeAction
	for _, act := range actionsFor(t, file, src, rng) {
		if act.Kind == identityActionKind {
			out = append(out, act)
		}
	}
	return out
}

// mintAction returns the one minting action offered at rng, or fails.
func mintAction(t *testing.T, file, src string, rng protocol.Range) protocol.CodeAction {
	t.Helper()
	var minting []protocol.CodeAction
	for _, act := range identityActionsFor(t, file, src, rng) {
		if strings.HasPrefix(act.Title, "Annotate ") {
			minting = append(minting, act)
		}
	}
	if len(minting) != 1 {
		t.Fatalf("minting actions = %+v, want one", minting)
	}
	if len(minting[0].Diagnostics) != 0 {
		t.Errorf("minting is opt-in, yet the action is attached to diagnostics %+v", minting[0].Diagnostics)
	}
	return minting[0]
}

// applyAll returns src with every edit of the action applied, later positions
// first so earlier offsets stay valid, inserts at one position in array order.
func applyAll(t *testing.T, src string, act protocol.CodeAction, file string) string {
	t.Helper()
	if act.Edit == nil {
		t.Fatalf("action %q has no workspace edit", act.Title)
	}
	edits := act.Edit.Changes[uri.File(file)]
	if len(edits) == 0 {
		t.Fatalf("action %q has no edits for %s", act.Title, file)
	}
	content := []byte(src)
	order := make([]int, len(edits))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := positionToOffset(content, edits[order[i]].Range.Start), positionToOffset(content, edits[order[j]].Range.Start)
		if a != b {
			return a > b
		}
		return order[i] > order[j]
	})
	out := src
	for _, i := range order {
		out = apply(t, out, edits[i])
	}
	return out
}

// mintedID extracts the id the action's edits declare, checking its shape.
func mintedID(t *testing.T, act protocol.CodeAction, file string) string {
	t.Helper()
	idOf := regexp.MustCompile(`id = "([^"]*)"`)
	for _, e := range act.Edit.Changes[uri.File(file)] {
		if m := idOf.FindStringSubmatch(e.NewText); m != nil {
			if !uuidV4.MatchString(m[1]) {
				t.Errorf("minted id %q is no UUID v4", m[1])
			}
			return m[1]
		}
	}
	t.Fatalf("no id in edits %+v", act.Edit.Changes)
	return ""
}

// identityAfter analyzes the annotated source and returns the identity of the
// named element, failing on any error the annotation introduced.
func identityAfter(t *testing.T, file, src, fqn string) *identity.Info {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File(file).Filename()
	ws.Open(name, []byte(src), 1)
	for _, d := range ws.Diagnostics(name) {
		if d.Severity == passes.SeverityError {
			t.Errorf("annotated source reports %s: %s", d.Code, d.Message)
		}
	}
	syms := ws.LookupQualified(fqn)
	if len(syms) != 1 {
		t.Fatalf("%s resolves to %d symbols in\n%s", fqn, len(syms), src)
	}
	info, ok := ws.IdentityOf(name, syms[0])
	if !ok {
		t.Fatalf("no identity for %s", fqn)
	}
	return info
}

const scopedSrc = "package Vehicles {\n    @IdentityMetadata::ProjectRef { projectId = \"p-1\"; }\n\n    // the chassis\n    part def Chassis {\n        attribute mass : ScalarValues::Real;\n    }\n    part def Wheel {\n        @IdentityMetadata::ElementId { id = \"wheel-id\"; }\n    }\n    /* two per vehicle */ part def Axle;\n    part front : Wheel;\n}\n"

func TestIdentityActionOfferedOnUnannotatedDeclaration(t *testing.T) {
	const file = "/tmp/identity_offered.sysml"
	act := mintAction(t, file, scopedSrc, cursorAt(t, scopedSrc, "Chassis"))
	if act.Title != "Annotate 'Chassis' with a minted element id" {
		t.Errorf("title = %q", act.Title)
	}
	id := mintedID(t, act, file)
	got := applyAll(t, scopedSrc, act, file)
	want := strings.Replace(scopedSrc, "    part def Chassis {\n", "    part def Chassis {\n        @IdentityMetadata::ElementId { id = \""+id+"\"; }\n", 1)
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	info := identityAfter(t, file, got, "Vehicles::Chassis")
	if !info.Annotated || !info.Declared || info.EffectiveID != id {
		t.Errorf("identity after = %+v, want the minted id %s declared", info, id)
	}
	if info.Scope == nil || info.Scope.ProjectID != "p-1" {
		t.Errorf("scope after = %+v, want the existing ProjectRef", info.Scope)
	}
}

// selection is the range from the start of the line holding from through to.
func selection(t *testing.T, src, from, to string) protocol.Range {
	t.Helper()
	start, end := strings.Index(src, from), strings.Index(src, to)
	if start < 0 || end < 0 {
		t.Fatalf("markers %q, %q not in source", from, to)
	}
	start = strings.LastIndexByte(src[:start], '\n') + 1
	return protocol.Range{Start: offsetToPosition([]byte(src), start), End: offsetToPosition([]byte(src), end+len(to))}
}

func TestIdentityActionOfferedOnSelectedHeader(t *testing.T) {
	const file = "/tmp/identity_selected.sysml"
	for _, tc := range []struct{ from, to, name string }{
		{"part def Chassis", "Chassis {", "Chassis"},
		{"// the chassis", "def Chassis", "Chassis"},
		{"/* two per vehicle */", "Axle;", "Axle"},
		{"part def Chassis", "Chassis {\n", "Chassis"},
		{"part def Axle", "Axle;\n", "Axle"},
	} {
		act := mintAction(t, file, scopedSrc, selection(t, scopedSrc, tc.from, tc.to))
		if want := "Annotate '" + tc.name + "' with a minted element id"; act.Title != want {
			t.Errorf("selection %q..%q: title = %q, want %q", tc.from, tc.to, act.Title, want)
		}
	}
	for _, tc := range []struct{ from, to string }{
		{"part def Chassis", "attribute mass"},
		{"part def Axle", "part front"},
	} {
		if acts := identityActionsFor(t, file, scopedSrc, selection(t, scopedSrc, tc.from, tc.to)); len(acts) != 0 {
			t.Errorf("selection %q..%q: actions = %+v, want none", tc.from, tc.to, acts)
		}
	}
	// Blank and note lines of the body name no other declaration, so a selection
	// running through them still means the header; the first member ends that.
	const noted = "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"p\"; }\n    part def Frame {\n\n        // weight\n        //* rough */\n        attribute w;\n    }\n}\n"
	if act := mintAction(t, file, noted, selection(t, noted, "part def Frame", "rough */\n")); act.Title != "Annotate 'Frame' with a minted element id" {
		t.Errorf("selection through body notes: title = %q", act.Title)
	}
	if acts := identityActionsFor(t, file, noted, selection(t, noted, "part def Frame", "attribute")); len(acts) != 0 {
		t.Errorf("selection through to the first member: actions = %+v, want none", acts)
	}
}

func TestIdentityActionNotOfferedOnAnnotatedDeclaration(t *testing.T) {
	const file = "/tmp/identity_annotated.sysml"
	if acts := identityActionsFor(t, file, scopedSrc, cursorAt(t, scopedSrc, "Wheel {")); len(acts) != 0 {
		t.Errorf("actions on an annotated element = %+v, want none", acts)
	}
}

func TestIdentityActionMintsFreshIDs(t *testing.T) {
	const file = "/tmp/identity_fresh.sysml"
	rng := cursorAt(t, scopedSrc, "Chassis")
	first := mintedID(t, mintAction(t, file, scopedSrc, rng), file)
	second := mintedID(t, mintAction(t, file, scopedSrc, rng), file)
	if first == second {
		t.Errorf("two requests minted the same id %s", first)
	}
}

func TestIdentityActionAppendsAboutFormForBodilessDeclaration(t *testing.T) {
	const file = "/tmp/identity_about.sysml"
	act := mintAction(t, file, scopedSrc, cursorAt(t, scopedSrc, "Axle"))
	id := mintedID(t, act, file)
	got := applyAll(t, scopedSrc, act, file)
	want := scopedSrc + "metadata : IdentityMetadata::ElementId about Vehicles::Axle { id = \"" + id + "\"; }\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	info := identityAfter(t, file, got, "Vehicles::Axle")
	if !info.Declared || info.EffectiveID != id || len(info.Declarations) != 1 || !info.Declarations[0].About {
		t.Errorf("identity after = %+v, want one about-form declaration of %s", info, id)
	}
}

func TestIdentityActionAppendsAfterMissingFinalNewline(t *testing.T) {
	const file = "/tmp/identity_nonl.sysml"
	src := strings.TrimSuffix(scopedSrc, "\n")
	act := mintAction(t, file, src, cursorAt(t, src, "front"))
	id := mintedID(t, act, file)
	got := applyAll(t, src, act, file)
	want := src + "\nmetadata : IdentityMetadata::ElementId about Vehicles::front { id = \"" + id + "\"; }\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	if info := identityAfter(t, file, got, "Vehicles::front"); info.EffectiveID != id {
		t.Errorf("effective id = %q, want %s", info.EffectiveID, id)
	}
}

func TestIdentityActionQuotesUnrestrictedNames(t *testing.T) {
	const file = "/tmp/identity_quoted.sysml"
	const src = "package 'My Project' {\n    @IdentityMetadata::ProjectRef { projectId = \"p\"; }\n    part def 'Front Wheel';\n}\n"
	act := mintAction(t, file, src, cursorAt(t, src, "'Front Wheel'"))
	id := mintedID(t, act, file)
	got := applyAll(t, src, act, file)
	want := src + "metadata : IdentityMetadata::ElementId about 'My Project'::'Front Wheel' { id = \"" + id + "\"; }\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	if info := identityAfter(t, file, got, "My Project::Front Wheel"); info.EffectiveID != id {
		t.Errorf("effective id = %q, want %s", info.EffectiveID, id)
	}
}

func TestIdentityActionInlinePlacements(t *testing.T) {
	const file = "/tmp/identity_inline.sysml"
	const head = "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"p\"; }\n"
	cases := []struct {
		name, decl, want string
	}{
		{"empty body", "    part def A {}\n", "    part def A { @IdentityMetadata::ElementId { id = \"ID\"; } }\n"},
		{"empty multi-line body", "    part def A {\n    }\n", "    part def A { @IdentityMetadata::ElementId { id = \"ID\"; } }\n"},
		{"body of notes only", "    part def A { // todo\n    }\n", "    part def A { @IdentityMetadata::ElementId { id = \"ID\"; } // todo\n    }\n"},
		{"one-line body", "    part def A { attribute x; }\n", "    part def A { @IdentityMetadata::ElementId { id = \"ID\"; } attribute x; }\n"},
		{"members on own lines", "    part def A {\n        // first\n        attribute x;\n    }\n", "    part def A {\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        // first\n        attribute x;\n    }\n"},
		{"note on the brace line", "    part def A { // todo\n        attribute x;\n    }\n", "    part def A { // todo\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        attribute x;\n    }\n"},
		{"multi-line note before member", "    part def A {\n        //* why\n           x */\n        attribute x;\n    }\n", "    part def A {\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        //* why\n           x */\n        attribute x;\n    }\n"},
		{"comment member", "    part def A {\n        /* doc */\n    }\n", "    part def A {\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        /* doc */\n    }\n"},
		{"usage body", "    part def T;\n    part a : T {\n        part b : T;\n    }\n", "    part def T;\n    part a : T {\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        part b : T;\n    }\n"},
		{"braces in header", "    part def T;\n    part a : T = new T() {\n        part b : T;\n    }\n", "    part def T;\n    part a : T = new T() {\n        @IdentityMetadata::ElementId { id = \"ID\"; }\n        part b : T;\n    }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := head + tc.decl + "}\n"
			marker := "part def A"
			fqn := "P::A"
			if strings.Contains(tc.decl, "part a") {
				marker, fqn = "part a", "P::a"
			}
			act := mintAction(t, file, src, cursorAt(t, src, marker))
			id := mintedID(t, act, file)
			got := applyAll(t, src, act, file)
			want := head + strings.ReplaceAll(tc.want, "ID", id) + "}\n"
			if got != want {
				t.Errorf("applied =\n%s\nwant\n%s", got, want)
			}
			if info := identityAfter(t, file, got, fqn); info.EffectiveID != id {
				t.Errorf("effective id = %q, want %s", info.EffectiveID, id)
			}
		})
	}
}

func TestIdentityActionAnnotatesEveryBodyKind(t *testing.T) {
	const file = "/tmp/identity_bodies.sysml"
	const head = "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"p\"; }\n"
	cases := []struct {
		name, decl, marker, fqn string
	}{
		{"enumeration", "    enum def Color { red; green; }\n", "enum def Color", "P::Color"},
		{"action with statements", "    action def Go {\n        first start;\n        then action step;\n        then done;\n    }\n", "action def Go", "P::Go"},
		{"state", "    state def Machine {\n        entry; then off;\n        state off;\n    }\n", "state def Machine", "P::Machine"},
		{"nested state", "    state def Machine {\n        state off {\n            entry; then idle;\n            state idle;\n        }\n    }\n", "state off", "P::Machine::off"},
		{"nested package", "    package Inner {\n        part def A;\n    }\n", "package Inner", "P::Inner"},
		{"port", "    port def Plug {\n        in attribute v;\n    }\n", "port def Plug", "P::Plug"},
		{"requirement", "    requirement def Safe {\n        doc /* safe */\n    }\n", "requirement def Safe", "P::Safe"},
		{"calculation", "    calc def Sum {\n        in a : ScalarValues::Real;\n        return : ScalarValues::Real = a;\n    }\n", "calc def Sum", "P::Sum"},
		{"constraint", "    constraint def Positive {\n        in x : ScalarValues::Real;\n        x > 0\n    }\n", "constraint def Positive", "P::Positive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := head + tc.decl + "}\n"
			act := mintAction(t, file, src, cursorAt(t, src, tc.marker))
			id := mintedID(t, act, file)
			got := applyAll(t, src, act, file)
			if !strings.Contains(got, identity.ElementIdInline(id)) {
				t.Errorf("applied =\n%s\nwant an inline annotation", got)
			}
			if info := identityAfter(t, file, got, tc.fqn); info.EffectiveID != id {
				t.Errorf("effective id = %q, want %s in\n%s", info.EffectiveID, id, got)
			}
		})
	}
}

func TestIdentityActionOnlyOnDeclarationHeader(t *testing.T) {
	const file = "/tmp/identity_header.sysml"
	for _, marker := range []string{"attribute mass", "ScalarValues::Real", "wheel-id", "        @IdentityMetadata::ElementId"} {
		rng := cursorAt(t, scopedSrc, marker)
		acts := identityActionsFor(t, file, scopedSrc, rng)
		for _, act := range acts {
			if strings.Contains(act.Title, "'Wheel'") || strings.Contains(act.Title, "'Vehicles'") {
				t.Errorf("at %q: offered %q for the enclosing declaration", marker, act.Title)
			}
		}
	}
	if acts := identityActionsFor(t, file, scopedSrc, cursorAt(t, scopedSrc, "the chassis")); len(acts) != 0 {
		t.Errorf("actions in trivia = %+v, want none", acts)
	}
	if acts := identityActionsFor(t, file, scopedSrc, wholeFile); len(acts) != 0 {
		t.Errorf("actions for a range spanning bodies = %+v, want none", acts)
	}
}

func TestIdentityActionWithoutProjectRefBindsRoot(t *testing.T) {
	const file = "/tmp/identity_unscoped.sysml"
	const src = "package P {\n    part def A {\n        attribute x;\n    }\n}\n"
	act := mintAction(t, file, src, cursorAt(t, src, "part def A"))
	if act.Title != "Annotate 'A' with a minted element id and bind 'P' to a project" {
		t.Errorf("title = %q", act.Title)
	}
	id := mintedID(t, act, file)
	got := applyAll(t, src, act, file)
	want := "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"<projectId>\"; }\n    part def A {\n        @IdentityMetadata::ElementId { id = \"" + id + "\"; }\n        attribute x;\n    }\n}\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	info := identityAfter(t, file, got, "P::A")
	if info.EffectiveID != id || info.Scope == nil || info.Scope.ProjectID != "<projectId>" {
		t.Errorf("identity after = %+v, want %s bound to the placeholder project", info, id)
	}
}

func TestIdentityActionWithoutProjectRefOnRootItself(t *testing.T) {
	const file = "/tmp/identity_root.sysml"
	const src = "package P {\n    part def A;\n}\n"
	acts := identityActionsFor(t, file, src, cursorAt(t, src, "package P"))
	if len(acts) != 2 {
		t.Fatalf("actions = %+v, want the binding and the minting one", acts)
	}
	bind, mint := acts[0], acts[1]
	if bind.Title != "Bind 'P' to a project" {
		t.Errorf("companion title = %q", bind.Title)
	}
	if got := applyAll(t, src, bind, file); got != "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"<projectId>\"; }\n    part def A;\n}\n" {
		t.Errorf("bound =\n%s", got)
	}
	id := mintedID(t, mint, file)
	got := applyAll(t, src, mint, file)
	want := "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"<projectId>\"; }\n    @IdentityMetadata::ElementId { id = \"" + id + "\"; }\n    part def A;\n}\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	info := identityAfter(t, file, got, "P")
	if info.EffectiveID != id || info.Scope == nil || info.Scope.Symbol != info.Symbol {
		t.Errorf("identity after = %+v, want %s scoped by its own ProjectRef", info, id)
	}
}

func TestIdentityActionWithoutProjectRefOnBodilessRoot(t *testing.T) {
	const file = "/tmp/identity_bodiless_root.sysml"
	const src = "part def X;\n"
	act := mintAction(t, file, src, cursorAt(t, src, "X"))
	id := mintedID(t, act, file)
	got := applyAll(t, src, act, file)
	want := src + "metadata : IdentityMetadata::ProjectRef about X { projectId = \"<projectId>\"; }\nmetadata : IdentityMetadata::ElementId about X { id = \"" + id + "\"; }\n"
	if got != want {
		t.Errorf("applied =\n%s\nwant\n%s", got, want)
	}
	info := identityAfter(t, file, got, "X")
	if info.EffectiveID != id || info.Scope == nil {
		t.Errorf("identity after = %+v, want %s in a project scope", info, id)
	}
}

func TestIdentityActionBoundRootOffersNoBinding(t *testing.T) {
	const file = "/tmp/identity_bound_root.sysml"
	acts := identityActionsFor(t, file, scopedSrc, cursorAt(t, scopedSrc, "package Vehicles"))
	if len(acts) != 1 || acts[0].Title != "Annotate 'Vehicles' with a minted element id" {
		t.Errorf("actions on a bound root = %+v, want only the minting one", acts)
	}
}

func TestIdentityActionHonorsKindFilter(t *testing.T) {
	const file = "/tmp/identity_only.sysml"
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File(file).Filename()
	ws.Open(name, []byte(scopedSrc), 1)
	for _, tc := range []struct {
		only []protocol.CodeActionKind
		want int
	}{
		{[]protocol.CodeActionKind{protocol.QuickFix}, 0},
		{[]protocol.CodeActionKind{protocol.Refactor}, 1},
		{[]protocol.CodeActionKind{protocol.RefactorRewrite}, 1},
		{nil, 1},
	} {
		acts, err := s.CodeAction(context.Background(), &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Range:        cursorAt(t, scopedSrc, "Chassis"),
			Context:      protocol.CodeActionContext{Only: tc.only},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(acts) != tc.want {
			t.Errorf("only=%v: actions = %+v, want %d", tc.only, acts, tc.want)
		}
	}
}

// A library element's id is fixed by the norm, so nothing is offered to mint one.
func TestIdentityActionNotOfferedOnLibraryDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	const file = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	doc := ws.LibraryDocument(file)
	if doc == nil {
		t.Fatalf("library document %q not bundled", file)
	}
	at := strings.Index(string(doc.Content), "datatype Real ") + len("datatype ")
	acts, err := s.identityActions(file, doc, source.Span{Offset: at, Len: len("Real")})
	if err != nil {
		t.Fatalf("identityActions err = %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("actions on ScalarValues::Real = %+v, want none", acts)
	}
}
