package querytranslate

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func isMetaclass(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolMetaclass || sym.Kind == symbols.SymbolMetadataDef)
}

func inMetamodel(sym *symbols.Symbol) bool {
	fqn := symbols.FQNOf(sym)
	return isMetaclass(sym) && (strings.HasPrefix(fqn, "KerML::") || strings.HasPrefix(fqn, "SysML::"))
}

// The table built from the metaclasses' own features, classified on demand,
// knows every property and kind a walk of each metaclass's inherited members does.
func TestTableAgreesWithInheritedMemberWalk(t *testing.T) {
	index := libs.NewModelIndex()
	index.ExpandWildcardImports()
	model := semantics.NewModel(resolve.New(index))
	table := NewTable(index, model)
	walked := NewTable(index, model)
	walked.features = map[string][]*symbols.Symbol{}
	for _, pkg := range queryexec.MetaclassPackages {
		for _, owner := range index.LookupQualified(pkg) {
			if owner.Scope == nil {
				continue
			}
			for _, metaclass := range owner.Scope.Members() {
				if !isMetaclass(metaclass) {
					continue
				}
				for _, member := range model.MembersOfIncludingRedefined(metaclass) {
					// A feature of the library's own metaclass (Metaobjects::Metaobject::self) is not the metamodel's.
					if member == nil || !member.IsFeature() || member.OwnerScope == nil || !inMetamodel(member.OwnerScope.Owner()) {
						continue
					}
					if name := symbols.LastSegment(member.Name); name != "" {
						walked.features[name] = append(walked.features[name], member)
					}
				}
			}
		}
	}
	names := table.MetamodelNames()
	if len(names) < 200 {
		t.Fatalf("table knows %d metamodel properties, want the library's hundreds", len(names))
	}
	for _, name := range walked.MetamodelNames() {
		if !table.Metamodel(name) {
			t.Errorf("%s: the inherited-member walk knows it, the table does not", name)
		}
	}
	for _, name := range names {
		if !walked.Metamodel(name) {
			t.Errorf("%s: the table knows it, the inherited-member walk does not", name)
			continue
		}
		if got, want := table.kind(name), walked.kind(name); got != want {
			t.Errorf("%s: kind %d, the inherited-member walk says %d", name, got, want)
		}
	}
	// A declaration decides over the evaluator's default for the name: documentation
	// and payloadArgument hold elements, so a query selects into them and cannot order by them.
	for name, kind := range map[string]valueKind{"isAbstract": kindBoolean, "name": kindText, "owner": kindElement,
		"documentation": kindElement, "payloadArgument": kindElement, "ownedRelationship": kindElement,
		"isComposite": kindBoolean, "aliasIds": kindText, "elementId": kindText} {
		if got := table.kind(name); got != kind {
			t.Errorf("kind(%s) = %d, want %d", name, got, kind)
		}
	}
}
