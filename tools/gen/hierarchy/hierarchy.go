// Package hierarchy orders the metamodel's metaclasses parents first and computes
// their C3 linearizations, for the generators that emit a type per metaclass.
package hierarchy

import (
	"fmt"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Hierarchy is a validated generalization hierarchy.
type Hierarchy struct {
	// Order lists every metaclass after all of its parents, ties broken by name.
	Order []ontology.Class
	// MRO is each metaclass's C3 linearization, the metaclass itself first.
	MRO map[string][]string
}

// New orders and linearizes classes, refusing a repeated name, an unknown
// parent, a cycle or parents whose orders C3 cannot reconcile.
func New(classes []ontology.Class) (*Hierarchy, error) {
	byName := make(map[string]ontology.Class, len(classes))
	for _, class := range classes {
		if _, duplicate := byName[class.Name]; duplicate {
			return nil, fmt.Errorf("metaclass %s is declared more than once", class.Name)
		}
		byName[class.Name] = class
	}
	order, err := topologicalOrder(classes, byName)
	if err != nil {
		return nil, err
	}
	h := &Hierarchy{Order: order, MRO: make(map[string][]string, len(classes))}
	for _, class := range order {
		if _, err := h.linearize(class.Name, byName, map[string]bool{}); err != nil {
			return nil, err
		}
	}
	return h, nil
}

// IsAncestorOrSelf reports whether ancestor is class or one of its superclasses.
func (h *Hierarchy) IsAncestorOrSelf(class, ancestor string) bool {
	return slices.Contains(h.MRO[class], ancestor)
}

func topologicalOrder(classes []ontology.Class, byName map[string]ontology.Class) ([]ontology.Class, error) {
	children := make(map[string][]string, len(classes))
	remaining := make(map[string]int, len(classes))
	for _, class := range classes {
		remaining[class.Name] = len(class.Parents)
		for _, parent := range class.Parents {
			if _, ok := byName[parent]; !ok {
				return nil, fmt.Errorf("%s has unknown metaclass parent %s", class.Name, parent)
			}
			children[parent] = append(children[parent], class.Name)
		}
	}
	var ready []string
	for name, count := range remaining {
		if count == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	order := make([]ontology.Class, 0, len(classes))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		order = append(order, byName[name])
		for _, child := range children[name] {
			remaining[child]--
			if remaining[child] == 0 {
				ready = append(ready, child)
			}
		}
		sort.Strings(ready)
	}
	if len(order) != len(classes) {
		return nil, fmt.Errorf("metaclass inheritance contains a cycle")
	}
	return order, nil
}

func (h *Hierarchy) linearize(name string, byName map[string]ontology.Class, visiting map[string]bool) ([]string, error) {
	if mro, ok := h.MRO[name]; ok {
		return mro, nil
	}
	if visiting[name] {
		return nil, fmt.Errorf("C3 linearization failed for %s: inheritance cycle", name)
	}
	visiting[name] = true
	class := byName[name]
	sequences := make([][]string, 0, len(class.Parents)+1)
	for _, parent := range class.Parents {
		parentMRO, err := h.linearize(parent, byName, visiting)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, append([]string(nil), parentMRO...))
	}
	sequences = append(sequences, append([]string(nil), class.Parents...))
	result, err := c3Merge(name, class.Parents, sequences)
	if err != nil {
		return nil, err
	}
	delete(visiting, name)
	h.MRO[name] = result
	return result, nil
}

// c3Merge is the C3 merge of the parents' linearizations and the parent list.
func c3Merge(name string, parents []string, sequences [][]string) ([]string, error) {
	result := []string{name}
	for {
		sequences = nonEmpty(sequences)
		if len(sequences) == 0 {
			return result, nil
		}
		candidate := c3Candidate(sequences)
		if candidate == "" {
			return nil, fmt.Errorf("C3 linearization failed for %s: inconsistent parents %v", name, parents)
		}
		result = append(result, candidate)
		for i, sequence := range sequences {
			if sequence[0] == candidate {
				sequences[i] = sequence[1:]
			}
		}
	}
}

func nonEmpty(sequences [][]string) [][]string {
	active := sequences[:0]
	for _, sequence := range sequences {
		if len(sequence) != 0 {
			active = append(active, sequence)
		}
	}
	return active
}

// c3Candidate is the first head that no sequence carries in its tail, or "".
func c3Candidate(sequences [][]string) string {
	for _, sequence := range sequences {
		if !inTails(sequence[0], sequences) {
			return sequence[0]
		}
	}
	return ""
}

func inTails(head string, sequences [][]string) bool {
	for _, other := range sequences {
		if slices.Contains(other[1:], head) {
			return true
		}
	}
	return false
}
