package grammar

import (
	"sort"
	"strconv"
	"strings"
)

// Action is an Xtext type-setting action in a production body.
type Action struct {
	Metaclass string `json:"metaclass"`
	Feature   string `json:"feature,omitempty"`
	Op        string `json:"op,omitempty"`
	Line      int    `json:"line"`
}

// Assignment describes a feature assignment and the metaclasses that can own it.
type Assignment struct {
	Feature  string   `json:"feature"`
	Op       string   `json:"op"`
	Line     int      `json:"line"`
	Value    string   `json:"value"`
	CrossRef []string `json:"crossRef,omitempty"`
	Owners   []string `json:"owners"`
}

// Shape is the extracted object-creation and notation shape of a production.
type Shape struct {
	Grammar          string       `json:"grammar"`
	Name             string       `json:"name"`
	Kind             Kind         `json:"kind"`
	Line             int          `json:"line"`
	Returns          string       `json:"returns"`
	ReturnsDefaulted bool         `json:"returnsDefaulted"`
	Datatype         bool         `json:"datatype"`
	Actions          []Action     `json:"actions,omitempty"`
	Assignments      []Assignment `json:"assignments,omitempty"`
	Delegates        []string     `json:"delegates,omitempty"`
	Fragments        []string     `json:"fragments,omitempty"`
	Creates          []string     `json:"creates,omitempty"`
	Anchors          []string     `json:"anchors,omitempty"`
}

type productionKey struct {
	grammar string
	name    string
}

type shapeInfo struct {
	production Production
	returns    string
	datatype   bool
}

// Shapes extracts one production shape per grammar declaration, preserving
// grammar and declaration order.
func Shapes(grammars []*Grammar) []Shape {
	resolver := newAnalyzer(grammars, nil)
	infos := map[productionKey]shapeInfo{}
	candidates := map[productionKey]bool{}
	for _, g := range grammars {
		for _, p := range g.Productions {
			key := productionKey{grammar: p.Grammar, name: p.Name}
			info := shapeInfo{production: p}
			infos[key] = info
			if p.Kind == KindRule || p.Kind == KindFragment {
				hasAssignment, hasAction := exprFeatures(p.Body)
				if !hasAssignment && !hasAction &&
					(p.Returns == "" || strings.HasPrefix(p.Returns, "Ecore::")) {
					candidates[key] = true
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for key := range candidates {
			if datatypeCallsOnly(infos[key].production, resolver, candidates) {
				continue
			}
			delete(candidates, key)
			changed = true
		}
	}
	for key, info := range infos {
		info.datatype = candidates[key]
		if info.production.Returns != "" {
			info.returns = info.production.Returns
		} else if info.datatype || info.production.Kind == KindTerminal || info.production.Kind == KindEnum {
			info.returns = "Ecore::EString"
		} else {
			info.returns = "SysML::" + info.production.Name
		}
		infos[key] = info
	}

	creates := map[productionKey][]string{}
	for changed := true; changed; {
		changed = false
		for key, info := range infos {
			if info.production.Kind != KindRule || info.datatype {
				continue
			}
			produced := map[string]bool{}
			states := interpretShape(info.production.Body, newStates(""), info, info.returns, infos, resolver, creates, nil, produced, map[productionKey]bool{})
			next := unionMetaclasses(finalMetaclasses(states), sortedOwnerSet(produced))
			if !sameStrings(creates[key], next) {
				creates[key] = next
				changed = true
			}
		}
	}

	out := make([]Shape, 0, len(infos))
	for _, g := range grammars {
		for _, p := range g.Productions {
			key := productionKey{grammar: p.Grammar, name: p.Name}
			info := infos[key]
			shape := Shape{
				Grammar:          p.Grammar,
				Name:             p.Name,
				Kind:             p.Kind,
				Line:             p.Line,
				Returns:          info.returns,
				ReturnsDefaulted: p.Returns == "",
				Datatype:         info.datatype,
			}
			ownerSets := map[int]map[string]bool{}
			start := newStates("")
			if p.Kind == KindFragment {
				start = newStates(info.returns)
			}
			interpretShape(p.Body, start, info, info.returns, infos, resolver, creates, ownerSets, nil, map[productionKey]bool{})
			appendShapeContents(&shape, p.Body, ownerSets, info, infos, resolver)
			if p.Kind == KindRule && !info.datatype {
				shape.Creates = append([]string(nil), creates[key]...)
			}
			shape.Anchors = shapeAnchors(p, info, infos, resolver, map[productionKey]bool{})
			out = append(out, shape)
		}
	}
	return out
}

func exprFeatures(e expr) (assignment, action bool) {
	switch v := e.(type) {
	case assignExpr:
		return true, false
	case actionExpr:
		return false, true
	case seqExpr:
		for _, item := range v.Items {
			a, x := exprFeatures(item)
			assignment, action = assignment || a, action || x
		}
	case altExpr:
		for _, item := range v.Items {
			a, x := exprFeatures(item)
			assignment, action = assignment || a, action || x
		}
	case optExpr:
		return exprFeatures(v.Item)
	}
	return assignment, action
}

func datatypeCallsOnly(p Production, resolver *analyzer, candidates map[productionKey]bool) bool {
	valid := true
	var walk func(expr)
	walk = func(e expr) {
		switch v := e.(type) {
		case refExpr:
			called, ok := resolver.lookup(p.Grammar, v.Name)
			if !ok {
				valid = false
				return
			}
			if called.Kind == KindTerminal || called.Kind == KindEnum {
				return
			}
			if called.Kind == KindRule || called.Kind == KindFragment {
				if candidates[productionKey{grammar: called.Grammar, name: called.Name}] {
					return
				}
			}
			valid = false
		case seqExpr:
			for _, item := range v.Items {
				walk(item)
			}
		case altExpr:
			for _, item := range v.Items {
				walk(item)
			}
		case optExpr:
			walk(v.Item)
		case crossRefExpr:
			valid = false
		}
	}
	walk(p.Body)
	return valid
}

type stateSet map[string]bool

func newStates(values ...string) stateSet {
	out := stateSet{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func unionStates(left, right stateSet) stateSet {
	out := newStates()
	for state := range left {
		out[state] = true
	}
	for state := range right {
		out[state] = true
	}
	return out
}

func sameStates(left, right stateSet) bool {
	if len(left) != len(right) {
		return false
	}
	for state := range left {
		if !right[state] {
			return false
		}
	}
	return true
}

func interpretShape(e expr, states stateSet, owner shapeInfo, defaultReturns string, infos map[productionKey]shapeInfo,
	resolver *analyzer, creates map[productionKey][]string, owners map[int]map[string]bool,
	produced map[string]bool, activeFragments map[productionKey]bool,
) stateSet {
	switch v := e.(type) {
	case nil, litExpr, crossRefExpr:
		return states
	case actionExpr:
		if produced != nil {
			produced[v.Type] = true
		}
		if v.Feature != "" && owners != nil {
			owners[v.id] = map[string]bool{v.Type: true}
		}
		return newStates(v.Type)
	case assignExpr:
		if owners != nil {
			targets := owners[v.id]
			if targets == nil {
				targets = map[string]bool{}
				owners[v.id] = targets
			}
			for state := range states {
				if state == "" {
					state = defaultReturns
					if produced != nil {
						produced[state] = true
					}
				}
				targets[state] = true
			}
		}
		out := newStates()
		for state := range states {
			if state == "" {
				state = defaultReturns
				if produced != nil {
					produced[state] = true
				}
			}
			out[state] = true
		}
		return out
	case refExpr:
		called, ok := resolver.lookup(owner.production.Grammar, v.Name)
		if !ok {
			return states
		}
		key := productionKey{grammar: called.Grammar, name: called.Name}
		info := infos[key]
		if called.Kind == KindFragment {
			if activeFragments[key] {
				return states
			}
			activeFragments[key] = true
			out := interpretShape(called.Body, states, info, defaultReturns, infos, resolver, creates, owners, produced, activeFragments)
			delete(activeFragments, key)
			return out
		}
		if called.Kind == KindRule && !info.datatype {
			if produced != nil {
				for _, created := range creates[key] {
					produced[created] = true
				}
			}
			return newStates(creates[key]...)
		}
		return states
	case seqExpr:
		out := states
		for _, item := range v.Items {
			out = interpretShape(item, out, owner, defaultReturns, infos, resolver, creates, owners, produced, activeFragments)
		}
		return out
	case altExpr:
		out := newStates()
		for _, item := range v.Items {
			out = unionStates(out, interpretShape(item, newStatesFrom(states), owner, defaultReturns, infos, resolver, creates, owners, produced, activeFragments))
		}
		return out
	case optExpr:
		before := newStatesFrom(states)
		current := newStatesFrom(states)
		for {
			next := unionStates(before, interpretShape(v.Item, newStatesFrom(current), owner, defaultReturns, infos, resolver, creates, owners, produced, activeFragments))
			if sameStates(next, current) {
				return next
			}
			current = next
		}
	}
	return states
}

func newStatesFrom(states stateSet) stateSet {
	out := newStates()
	for state := range states {
		out[state] = true
	}
	return out
}

func finalMetaclasses(states stateSet) []string {
	var out []string
	for state := range states {
		if state != "" {
			out = append(out, state)
		}
	}
	sort.Strings(out)
	return out
}

func unionMetaclasses(left, right []string) []string {
	set := map[string]bool{}
	for _, value := range left {
		set[value] = true
	}
	for _, value := range right {
		set[value] = true
	}
	return sortedOwnerSet(set)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func appendShapeContents(shape *Shape, e expr, owners map[int]map[string]bool, info shapeInfo,
	infos map[productionKey]shapeInfo, resolver *analyzer,
) {
	switch v := e.(type) {
	case actionExpr:
		shape.Actions = append(shape.Actions, Action{Metaclass: v.Type, Feature: v.Feature, Op: v.Op, Line: v.Line})
		if v.Feature != "" {
			shape.Assignments = append(shape.Assignments, Assignment{
				Feature: v.Feature, Op: v.Op, Line: v.Line, Value: "current",
				Owners: []string{v.Type},
			})
		}
	case assignExpr:
		shape.Assignments = append(shape.Assignments, Assignment{
			Feature: v.Feature, Op: v.Op, Line: v.Line, Value: exprValue(v.Value),
			CrossRef: crossRefTypes(v.Value), Owners: sortedOwnerSet(owners[v.id]),
		})
	case refExpr:
		called, ok := resolver.lookup(info.production.Grammar, v.Name)
		if !ok {
			return
		}
		key := productionKey{grammar: called.Grammar, name: called.Name}
		calledInfo := infos[key]
		if called.Kind == KindFragment {
			shape.Fragments = appendUnique(shape.Fragments, called.Name)
		} else if called.Kind == KindRule && !calledInfo.datatype {
			shape.Delegates = appendUnique(shape.Delegates, called.Name)
		}
	case seqExpr:
		for _, item := range v.Items {
			appendShapeContents(shape, item, owners, info, infos, resolver)
		}
	case altExpr:
		for _, item := range v.Items {
			appendShapeContents(shape, item, owners, info, infos, resolver)
		}
	case optExpr:
		appendShapeContents(shape, v.Item, owners, info, infos, resolver)
	}
}

func sortedOwnerSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for owner := range set {
		out = append(out, owner)
	}
	sort.Strings(out)
	return out
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func exprValue(e expr) string {
	switch v := e.(type) {
	case litExpr:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(v.Value, `\`, `\\`), "'", `\'`) + "'"
	case refExpr:
		return v.Name
	case crossRefExpr:
		if v.Terminal == "" {
			return "[" + v.Type + "]"
		}
		return "[" + v.Type + " | " + v.Terminal + "]"
	case altExpr:
		values := make([]string, len(v.Items))
		for i, item := range v.Items {
			values[i] = exprValue(item)
		}
		return strings.Join(values, " | ")
	case seqExpr:
		values := make([]string, len(v.Items))
		for i, item := range v.Items {
			values[i] = exprValue(item)
		}
		return strings.Join(values, " ")
	case optExpr:
		return exprValue(v.Item)
	case assignExpr:
		return exprValue(v.Value)
	default:
		return strconv.Quote("")
	}
}

func crossRefTypes(e expr) []string {
	var out []string
	var walk func(expr)
	walk = func(e expr) {
		switch v := e.(type) {
		case crossRefExpr:
			out = appendUnique(out, v.Type)
		case seqExpr:
			for _, item := range v.Items {
				walk(item)
			}
		case altExpr:
			for _, item := range v.Items {
				walk(item)
			}
		case optExpr:
			walk(v.Item)
		}
	}
	walk(e)
	return out
}

func shapeAnchors(p Production, info shapeInfo, infos map[productionKey]shapeInfo,
	resolver *analyzer, active map[productionKey]bool,
) []string {
	var out []string
	var walk func(expr, string, bool)
	walk = func(e expr, grammar string, expandCalls bool) {
		switch v := e.(type) {
		case litExpr:
			out = appendUnique(out, v.Value)
		case refExpr:
			if !expandCalls {
				return
			}
			called, ok := resolver.lookup(grammar, v.Name)
			if !ok {
				return
			}
			key := productionKey{grammar: called.Grammar, name: called.Name}
			calledInfo := infos[key]
			if (called.Kind != KindFragment && !calledInfo.datatype) || active[key] {
				return
			}
			active[key] = true
			walk(called.Body, called.Grammar, true)
			delete(active, key)
		case assignExpr:
			walk(v.Value, grammar, false)
		case seqExpr:
			for _, item := range v.Items {
				walk(item, grammar, expandCalls)
			}
		case altExpr:
			for _, item := range v.Items {
				walk(item, grammar, expandCalls)
			}
		case optExpr:
			walk(v.Item, grammar, expandCalls)
		}
	}
	walk(p.Body, p.Grammar, true)
	return out
}
