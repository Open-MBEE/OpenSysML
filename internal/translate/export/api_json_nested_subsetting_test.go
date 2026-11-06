package export

import (
	"slices"
	"strings"
	"testing"
)

// A composite usage nested in a type implicitly subsets the library feature
// SysML v2 §8.3.6.4 names for its kind and its owner's (a part in a part
// definition is one of Items::Item::subparts, an item one of subitems, a
// performed action one of Parts::Part::performedActions), and the element form
// inherits that feature's members. A referential usage beside it subsets
// nothing by nesting and inherits through its kind base alone.
func TestAPIJSONNestedUsageInheritsThroughItsImpliedSubsetting(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def Vehicle {
			part eng;
			ref part spare;
			item cargo;
			ref item loose;
			perform action drive;
			ref action idle;
		}
		part vehicle : Vehicle { part wheel; }
	}`, IDQualifiedName)
	var references []APIJSONLibraryReference
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{LibraryReferences: func(refs []APIJSONLibraryReference) {
		references = refs
	}})
	if err != nil {
		t.Fatal(err)
	}
	named := make(map[string]string, len(references))
	for _, reference := range references {
		named[reference.ID] = reference.QualifiedName + reference.LibraryPath
	}
	elements := booleansElements(t, document)
	for _, c := range []struct{ nested, referential, through string }{
		{"Probe__Vehicle__eng", "Probe__Vehicle__spare", "Items::Item::subparts"},
		{"Probe__vehicle__wheel", "Probe__Vehicle__spare", "Items::Item::subparts"},
		{"Probe__Vehicle__cargo", "Probe__Vehicle__loose", "Items::Item::subitems"},
		{"Probe__Vehicle__drive", "Probe__Vehicle__idle", "Parts::Part::performedActions"},
	} {
		for _, property := range []string{"inheritedMembership", "member", "inheritedFeature"} {
			nested := referenceIDs(t, elements[c.nested], property)
			referential := referenceIDs(t, elements[c.referential], property)
			if len(nested) <= len(referential) {
				t.Errorf("%s.%s has %d entries, %s's %d: want the nested usage to add %s's members", c.nested, property, len(nested), c.referential, len(referential), c.through)
			}
			if property == "inheritedMembership" {
				continue
			}
			var added []string
			for _, id := range nested {
				if !slices.Contains(referential, id) {
					added = append(added, named[id])
				}
			}
			if !slices.ContainsFunc(added, func(name string) bool {
				return strings.HasPrefix(name, c.through+"/") || strings.HasPrefix(name, c.through+"::")
			}) {
				t.Errorf("%s.%s adds %v over %s's, none owned by %s", c.nested, property, added, c.referential, c.through)
			}
		}
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the graph changed through the element form:\nmissing: %v\nadded: %v", missing, added)
	}
}

// A requirement's `assume` and `require` constraints are ConstraintUsages owned
// through a RequirementConstraintMembership whose kind tells them apart, and
// SysML v2 §8.3.20.4 has them subset RequirementCheck::assumptions and
// ::constraints; the element form inherits through those, not through the
// occurrence rule a plain nested constraint follows.
func TestAPIJSONRequirementConstraintsInheritThroughTheirCheckCollections(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		requirement def Safe {
			assume constraint a;
			require constraint b;
			ref constraint plain;
		}
	}`, IDQualifiedName)
	var references []APIJSONLibraryReference
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{LibraryReferences: func(refs []APIJSONLibraryReference) {
		references = refs
	}})
	if err != nil {
		t.Fatal(err)
	}
	named := make(map[string]string, len(references))
	for _, reference := range references {
		named[reference.ID] = reference.QualifiedName + reference.LibraryPath
	}
	elements := booleansElements(t, document)
	// The two collections own nothing but their `doc`, so the member that tells
	// them apart is that Documentation, in `member`, not a feature.
	plain := referenceIDs(t, elements["Probe__Safe__plain"], "member")
	for _, c := range []struct{ constraint, through, other string }{
		{"Probe__Safe__a", "assumptions", "constraints"},
		{"Probe__Safe__b", "constraints", "assumptions"},
	} {
		var added []string
		for _, id := range referenceIDs(t, elements[c.constraint], "member") {
			if !slices.Contains(plain, id) {
				added = append(added, named[id])
			}
		}
		// The collection's members are keyed under the RequirementCheck feature or
		// the RequirementConstraintCheck redefinition of it; either names the kind.
		owned := func(collection string) bool {
			return slices.ContainsFunc(added, func(name string) bool {
				return strings.HasPrefix(name, "Requirements::") && (strings.Contains(name, "::"+collection+"/") || strings.Contains(name, "::"+collection+"::"))
			})
		}
		if !owned(c.through) {
			t.Errorf("%s.member adds %v over plain's, none owned by %s", c.constraint, added, c.through)
		}
		if owned(c.other) {
			t.Errorf("%s.member adds members owned by %s, the other kind's collection", c.constraint, c.other)
		}
	}
}
