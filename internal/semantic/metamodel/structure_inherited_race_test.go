package metamodel

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Concurrent structure-mode reads of one type must not race on the shared inherited-membership
// memo (run under -race); a sibling is read first so only the evaluator's memos are cold.
func TestInheritedPropertiesOfOneTypeReadConcurrently(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "race.sysml", "package P {}")
	warm, owner := Element{Aspect: "warm"}, Element{Aspect: "owner"}
	evaluator = New(evaluator.resolver, evaluator.model, Options{Structure: testStructure{
		metaclasses:        map[ElementKey]string{warm.Key(): "PartDefinition", owner.Key(): "PartDefinition"},
		ownedRelationships: map[ElementKey][]Element{warm.Key(): {}, owner.Key(): {}},
	}})
	properties := []string{"inheritedMembership", "inheritedFeature", "feature", "featureMembership"}
	for _, property := range properties {
		if _, ok := evaluator.Property(warm, "Type", property); !ok {
			t.Fatalf("Type::%s is not computable", property)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		for _, property := range properties {
			wg.Add(1)
			go func() {
				defer wg.Done()
				value, ok := evaluator.Property(owner, "Type", property)
				if !ok || !value.Success || len(value.Values) == 0 {
					t.Errorf("Type::%s = %v, %v: want the implied Parts::Part members", property, value.Success, ok)
				}
			}()
		}
	}
	wg.Wait()
}
