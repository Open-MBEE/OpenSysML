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
