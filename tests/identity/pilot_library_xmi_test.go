package identity_test

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
)

// TestPilotLibraryXMI asserts, in both directions, that the ids derived here are the
// ones the pilot's XMI (scripts/download-pilot-library-xmi.sh) carries; nothing is recorded.
const (
	pilotLibraryXMIRoot = "../../build/pilot-library-xmi"
	pilotLibraryXMIEnv  = "OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI"
)

// pilotOnlyElements are named elements the pilot serializes but this implementation
// derives no id for; kept exact, so the gate fails once one is derived. Empty:
// every named library element the pilot serializes is derived.
var pilotOnlyElements = map[string]string{}

// referenceNamedKinds are the usage kinds named after the feature they reference
// rather than one they redefine (`perform a`, `exhibit s`, `event e` and the like).
var referenceNamedKinds = map[string]bool{
	"sysml:PerformActionUsage":      true,
	"sysml:ExhibitStateUsage":       true,
	"sysml:IncludeUseCaseUsage":     true,
	"sysml:AssertConstraintUsage":   true,
	"sysml:SatisfyRequirementUsage": true,
	"sysml:EventOccurrenceUsage":    true,
	"sysml:ReferenceUsage":          true,
}

// xmiNode is the recursive shape of the pilot's XMI; a relationship's feature ends
// are attributes within a file and href children across files.
type xmiNode struct {
	Type              string    `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	ElementID         string    `xml:"elementId,attr"`
	DeclaredName      *string   `xml:"declaredName,attr"`
	ShortName         *string   `xml:"declaredShortName,attr"`
	IsEnd             bool      `xml:"isEnd,attr"`
	RedefinedFeature  string    `xml:"redefinedFeature,attr"`
	ReferencedFeature string    `xml:"referencedFeature,attr"`
	RedefinedHref     xmiHref   `xml:"redefinedFeature"`
	ReferencedHref    xmiHref   `xml:"referencedFeature"`
	Relationships     []xmiNode `xml:"ownedRelationship"`
	Elements          []xmiNode `xml:"ownedRelatedElement"`
}

type xmiHref struct {
	Href string `xml:"href,attr"`
}

// xmiElement is one element of the XMI that the pilot names, with the path of
// names that qualifies it and the id of the relationship that owns it.
type xmiElement struct {
	file         string
	path         string
	lang         identity.Language
	id           string
	membershipID string
}

// pilotLibrary is the downloaded XMI read whole: every element by id, so
// naming features resolve across files, and the relationship owning each.
type pilotLibrary struct {
	files []string
	roots []*xmiNode
	byID  map[string]*xmiNode
	owner map[*xmiNode]string
}

func TestPilotLibraryXMI(t *testing.T) {
	lib := readPilotLibraryXMI(t)
	named := lib.namedElements(t)
	var wrong []string
	for _, el := range named {
		// The path the pilot names an element by must derive to the id it wrote.
		if got := identity.ElementID(el.lang, el.path); got != el.id {
			wrong = append(wrong, fmt.Sprintf("%s: the pilot serializes %s, %s derives to %s",
				el.path, el.id, el.lang, got))
		}
		if got := identity.OwningMembershipID(el.lang, el.path); got != el.membershipID {
			wrong = append(wrong, fmt.Sprintf("%s/owningMembership: the pilot serializes %s, %s derives to %s",
				el.path, el.membershipID, el.lang, got))
		}
	}

	ours := identity.LibraryCatalog(libs.NewModelIndex()).Elements()
	if len(ours) == 0 {
		t.Fatal("the bundled library yields no normative identities")
	}
	derived := map[string]bool{}
	for _, el := range ours {
		derived[el.ID] = true
		node, ok := lib.byID[el.ID]
		switch {
		case !ok:
			wrong = append(wrong, fmt.Sprintf("%s: derived %s, the pilot serializes no element with that id", el.FQN, el.ID))
		case lib.owningRelationship(node) != el.OwningMembershipID:
			wrong = append(wrong, fmt.Sprintf("%s/owningMembership: derived %s, the pilot serializes %s",
				el.FQN, el.OwningMembershipID, lib.owningRelationship(node)))
		}
	}
	for _, el := range named {
		known, listed := pilotOnlyElements[el.path]
		listed = listed && known == el.id
		switch {
		case derived[el.id] && listed:
			wrong = append(wrong, fmt.Sprintf("%s: derived now, so no longer pilot-only; drop it from the list", el.path))
		case !derived[el.id] && !listed:
			wrong = append(wrong, fmt.Sprintf("%s: the pilot serializes %s, nothing derived", el.path, el.id))
		}
	}
	sort.Strings(wrong)
	if len(wrong) != 0 {
		limit := min(len(wrong), 40)
		t.Fatalf("%d identities differ from the pilot's XMI (first %d):\n  %s",
			len(wrong), limit, strings.Join(wrong[:limit], "\n  "))
	}
	t.Logf("pilot library XMI: %d derived and %d named elements agree with their owning memberships across %d files (%d known pilot-only)",
		len(ours), len(named), len(lib.files), len(pilotOnlyElements))
}

// readPilotLibraryXMI reads every downloaded XMI file, skipping (or failing,
// when CI requires the download) if there are none.
func readPilotLibraryXMI(t *testing.T) *pilotLibrary {
	t.Helper()
	lib := &pilotLibrary{byID: map[string]*xmiNode{}, owner: map[*xmiNode]string{}}
	err := filepath.WalkDir(pilotLibraryXMIRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".sysmlx") || strings.HasSuffix(path, ".kermlx")) {
			lib.files = append(lib.files, path)
		}
		return nil
	})
	if os.IsNotExist(err) || (err == nil && len(lib.files) == 0) {
		const hint = "pilot library XMI not downloaded (run ./scripts/download-pilot-library-xmi.sh)"
		if os.Getenv(pilotLibraryXMIEnv) != "" {
			t.Fatalf("%s=%s but %s", pilotLibraryXMIEnv, os.Getenv(pilotLibraryXMIEnv), hint)
		}
		fmt.Fprintf(os.Stderr, "\n!!! GATE NOT RUN: %s SKIPPED - %s.\n"+
			"!!! CI sets %s=1, where an absent download fails instead of skipping.\n\n",
			t.Name(), hint, pilotLibraryXMIEnv)
		t.Skip(hint)
	}
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lib.files)
	for _, file := range lib.files {
		root := decodePilotXMI(t, file)
		lib.roots = append(lib.roots, root)
		lib.index(t, file, root, "")
	}
	return lib
}

func decodePilotXMI(t *testing.T, file string) *xmiNode {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	// The files declare encoding="ASCII", a subset of UTF-8; read them as is.
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	root := &xmiNode{}
	if err := dec.Decode(root); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return root
}

// index records every node of a file by its library-unique id, which is what lets
// an href resolve by fragment, and each element's owning relationship.
func (lib *pilotLibrary) index(t *testing.T, file string, node *xmiNode, owner string) {
	t.Helper()
	if node.ElementID != "" {
		if prior, dup := lib.byID[node.ElementID]; dup && prior != node {
			t.Fatalf("%s: id %s serialized twice", file, node.ElementID)
		}
		lib.byID[node.ElementID] = node
	}
	lib.owner[node] = owner
	for i := range node.Relationships {
		lib.index(t, file, &node.Relationships[i], "")
	}
	for i := range node.Elements {
		lib.index(t, file, &node.Elements[i], node.ElementID)
	}
}

// owningRelationship is the id of the relationship whose related element node
// is, "" for a root namespace.
func (lib *pilotLibrary) owningRelationship(node *xmiNode) string { return lib.owner[node] }

// namedElements lists the elements the pilot gives a qualified name: those it
// names, declared or effectively, under owners it names all the way up.
func (lib *pilotLibrary) namedElements(t *testing.T) []xmiElement {
	t.Helper()
	var out []xmiElement
	for i, root := range lib.roots {
		file := lib.files[i]
		lang := identity.SysML
		if strings.Contains(file, "Kernel Libraries") {
			lang = identity.KerML
		}
		var walk func(owner *xmiNode, path string)
		walk = func(owner *xmiNode, path string) {
			for r := range owner.Relationships {
				rel := &owner.Relationships[r]
				for e := range rel.Elements {
					el := &rel.Elements[e]
					name, ok := lib.effectiveName(el, map[*xmiNode]bool{})
					if !ok {
						continue // no qualified name here, nor below
					}
					qn := name
					if path != "" {
						qn = path + "::" + name
					}
					out = append(out, xmiElement{file: file, path: qn, lang: lang, id: el.ElementID, membershipID: rel.ElementID})
					walk(el, qn)
				}
			}
		}
		walk(root, "")
	}
	return out
}

// effectiveName is a declared name or short name, else the effective name of the
// feature that names node (KerML 7.3.4.4, SysML 7.5.5).
func (lib *pilotLibrary) effectiveName(node *xmiNode, visited map[*xmiNode]bool) (string, bool) {
	if node.DeclaredName != nil {
		return *node.DeclaredName, *node.DeclaredName != ""
	}
	if node.ShortName != nil {
		return *node.ShortName, *node.ShortName != ""
	}
	if visited[node] {
		return "", false
	}
	visited[node] = true
	naming, ok := lib.namingFeature(node)
	if !ok {
		return "", false
	}
	return lib.effectiveName(naming, visited)
}

func (lib *pilotLibrary) namingFeature(node *xmiNode) (*xmiNode, bool) {
	for i := range node.Relationships {
		rel := &node.Relationships[i]
		switch {
		case lib.referenceNamed(node) && rel.Type == "sysml:ReferenceSubsetting":
			return lib.resolve(rel.ReferencedFeature, rel.ReferencedHref)
		case rel.Type == "sysml:Redefinition":
			return lib.resolve(rel.RedefinedFeature, rel.RedefinedHref)
		}
	}
	return nil, false
}

// referenceNamed reports whether node is named after the feature it references: a
// reference-named kind that is not a connector end, or a required constraint (`require c`).
func (lib *pilotLibrary) referenceNamed(node *xmiNode) bool {
	if node.IsEnd {
		return false
	}
	if referenceNamedKinds[node.Type] {
		return true
	}
	owner, ok := lib.byID[lib.owner[node]]
	return ok && node.Type == "sysml:ConstraintUsage" && owner.Type == "sysml:RequirementConstraintMembership"
}

// resolve finds the element an in-file id or a cross-file href names.
func (lib *pilotLibrary) resolve(id string, href xmiHref) (*xmiNode, bool) {
	if id == "" {
		if _, fragment, ok := strings.Cut(href.Href, "#"); ok {
			id = fragment
		}
	}
	node, ok := lib.byID[id]
	return node, ok
}
