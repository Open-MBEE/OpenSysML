package solve

import (
	"errors"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Refused is an object-holding feature of the element whose objects' own
// assertions a query about the element leaves out, and why: validating an
// object checks them, the query does not state them.
type Refused struct {
	// Feature is the holding feature; Path names it from the element, through
	// the features holding it (`engine.cylinder`).
	Feature *symbols.Symbol
	Path    string
	Reason  string
}

// frame is where a condition is read from: the features holding the object
// stating it, outermost first (none for the element's own conditions), the
// effective features of each object down that path (levels[0] the element's,
// levels[i] those of the object path[i-1] holds), and the variants the holding
// variations must select for the condition to apply.
type frame struct {
	path       []*symbols.Symbol
	levels     []map[string]*symbols.Symbol
	selections []selection
}

// selection is a variant a holding feature, named by its chain from the
// element, must select for a condition to apply: one a variant states.
type selection struct {
	holder    []*symbols.Symbol
	variant   *symbols.Symbol
	composite *symbols.Symbol
}

// heldGroup is the conditions the objects one held feature holds carry — stated
// by the feature, its type or a variant of either — each with its frame.
type heldGroup struct {
	feature *symbols.Symbol
	path    []*symbols.Symbol
	conds   []runtime.Condition
	bound   []map[*symbols.Symbol][]*symbols.Symbol
	frames  []frame
}

// heldCollector gathers the groups below an element the way validating one of
// its objects walks them (runtime.Context.ValidateObject): depth-bounded, a type
// recurring on the path cut, a feature that may hold more than one object
// refused by name rather than read as one.
type heldCollector struct {
	ctx     *runtime.Context
	groups  []heldGroup
	refused []Refused
	onPath  map[*symbols.Symbol]bool
}

// heldGroups collects the conditions the objects held below sym carry, and the
// held features whose conditions are refused before translation; levels0 maps
// the names sym's own conditions read.
func heldGroups(ctx *runtime.Context, sym *symbols.Symbol, levels0 map[string]*symbols.Symbol) ([]heldGroup, []Refused, error) {
	c := &heldCollector{ctx: ctx, onPath: map[*symbols.Symbol]bool{sym: true}}
	if err := c.collect(sym, nil, []map[string]*symbols.Symbol{levels0}, nil, 0); err != nil {
		return nil, nil, err
	}
	return c.groups, c.refused, nil
}

func (c *heldCollector) collect(
	stating *symbols.Symbol,
	path []*symbols.Symbol,
	levels []map[string]*symbols.Symbol,
	selections []selection,
	depth int,
) error {
	for _, held := range c.ctx.HeldFeatures(stating) {
		feature := held.Feature.Symbol
		fpath := append(append([]*symbols.Symbol(nil), path...), feature)
		sub := &heldCollector{ctx: c.ctx, onPath: map[*symbols.Symbol]bool{}}
		for typ := range c.onPath {
			sub.onPath[typ] = true
		}
		if held.Composite != nil {
			sub.onPath[held.Composite] = true
		}
		group := heldGroup{feature: feature, path: fpath}
		seen := map[*symbols.Symbol]bool{}
		state := func(candidate, variant *symbols.Symbol, recurse bool) error {
			if candidate == nil {
				return nil
			}
			own := append(append([]map[string]*symbols.Symbol(nil), levels...), effectiveFeatures(c.ctx, candidate))
			under := selections
			if variant != nil {
				under = append(append([]selection(nil), selections...), selection{holder: fpath, variant: variant, composite: held.Composite})
			}
			for _, a := range c.ctx.AssertionsOf(candidate, candidate.OwnerScope) {
				if seen[a.Symbol] {
					continue
				}
				seen[a.Symbol] = true
				conds, negated, subjects, err := assertedConditions(c.ctx, a)
				if err != nil {
					return err
				}
				if cond, states := assertedCondition(conds, negated); states {
					group.conds = append(group.conds, cond)
					group.bound = append(group.bound, subjects)
					group.frames = append(group.frames, frame{path: fpath, levels: own, selections: under})
				}
			}
			if !recurse {
				return nil
			}
			return sub.collect(candidate, fpath, own, under, depth+1)
		}
		// The feature's own effective features include its type's, so the type's
		// held features are walked once, through the feature.
		if err := state(feature, nil, true); err != nil {
			return err
		}
		if err := state(held.Composite, nil, false); err != nil {
			return err
		}
		for _, variant := range held.Variants {
			if err := state(variant, variant, true); err != nil {
				return err
			}
		}
		if len(group.conds) == 0 && len(sub.groups) == 0 && len(sub.refused) == 0 {
			continue
		}
		if reason := c.unreachable(held, depth); reason != "" {
			c.refused = append(c.refused, Refused{Feature: feature, Path: pathName(fpath), Reason: reason})
			continue
		}
		if len(group.conds) > 0 {
			c.groups = append(c.groups, group)
		}
		c.groups = append(c.groups, sub.groups...)
		c.refused = append(c.refused, sub.refused...)
	}
	return nil
}

// unreachable says why the objects held under a feature are not read as one
// object each: the feature may hold several, or validation itself would not
// reach them; empty when they are.
func (c *heldCollector) unreachable(held runtime.HeldFeature, depth int) string {
	if !singleValued(c.ctx.Semantics().EffectiveMultiplicityOf(held.Feature.Symbol)) {
		return "it may hold more than one object"
	}
	if held.Composite != nil && c.onPath[held.Composite] {
		return fmt.Sprintf("its type %s holds an object of its own type, which validation does not follow", held.Composite.Name)
	}
	if depth >= runtime.HeldObjectDepth {
		return fmt.Sprintf("it is held %d objects deep, further than validation reaches", depth+1)
	}
	return ""
}

// pathName writes a chain of holding features as the element's conditions
// would name it.
func pathName(path []*symbols.Symbol) string {
	name := ""
	for i, step := range path {
		if i > 0 {
			name += "."
		}
		name += source.QualifiedNameText(step.Name)
	}
	return name
}

// refusalOf is why a group's conditions cannot be translated, found by
// translating them apart from the query so a refusal leaves no trace in it;
// empty when they translate. Any other failure is the caller's.
func (t *translator) refusalOf(group heldGroup) (string, error) {
	trial := newTranslator(t.ctx, t.subject)
	trial.features = t.features
	trial.bound, trial.frames = group.bound, group.frames
	err := trial.translate(group.conds)
	if err == nil {
		return "", nil
	}
	var refused *NotTranslatableError
	if errors.As(err, &refused) {
		reason := refused.Construct + " is " + ErrNotTranslatable.Error()
		if refused.Reason != "" {
			reason += ": " + refused.Reason
		}
		return reason, nil
	}
	if errors.Is(err, ErrNotTranslatable) {
		return err.Error(), nil
	}
	return "", err
}

// selected is the condition under which the current frame's conditions apply:
// each holding variation selects the variant stating them, nil for one applying
// always. A holder is read as the variable its selection is, declared here if
// no condition reads it.
func (t *translator) selected() (*Term, error) {
	terms, err := t.selectedTerms()
	if err != nil {
		return nil, err
	}
	return conjunction(terms), nil
}

// selectedTerms is one `holder == variant` term per selection of the current frame.
func (t *translator) selectedTerms() ([]*Term, error) {
	var terms []*Term
	for _, s := range t.frame.selections {
		holder := s.holder[len(s.holder)-1]
		written := "variation `" + pathName(s.holder) + "`"
		for _, step := range s.holder {
			if !singleValued(t.model.EffectiveMultiplicityOf(step)) {
				return nil, t.refuse(holder.Decl, written, "it may hold more than one object")
			}
		}
		sort, _, err := t.sortOf(holder.Decl, holder.OwnerScope, holder, written)
		if err != nil {
			return nil, err
		}
		valueSort, value, ok := t.valueOf(s.variant)
		if !ok || valueSort.Name != sort.Name {
			return nil, t.refuse(s.variant.Decl, "variant `"+s.variant.Name+"`", "it is no variant of "+written)
		}
		v := t.declare(variableName(t, s.holder), sort, holder, "")
		t.read(v)
		if v.Root == nil && v.Steps == nil {
			v.Steps = append(v.Steps, s.holder...)
			if len(v.Steps) <= 2 {
				v.Root = v.Steps[0]
			}
		}
		terms = append(terms, Binary(OpEq, Bool, VarTerm(v), ValueTerm(sort, value)))
	}
	return terms, nil
}

// framed translates cond where its frame applies it: a condition a variant
// states holds only of an object of that variant, so it is stated as the
// implication `holder == variant ⇒ cond`, evaluated only under the selection.
func (t *translator) framed(cond runtime.Condition) (*Term, error) {
	terms, err := t.selectedTerms()
	if err != nil {
		return nil, err
	}
	sel := conjunction(terms)
	if sel == nil {
		return t.condition(cond)
	}
	before := make(map[string]bool, len(t.vars))
	for name := range t.vars {
		before[name] = true
	}
	term, err := t.under(sel, func() (*Term, error) { return t.condition(cond) })
	if err != nil {
		return nil, err
	}
	t.liven(terms, before)
	return Not(And(sel, Not(term))), nil
}

// liven records under which selections the variables the condition introduced
// exist: a feature declared for one variant is a variation point only where the
// holding variation selects that variant (SysML v2 §7.6.7), so a variable of it
// is live under that selection alone; one every variant has is always live.
func (t *translator) liven(terms []*Term, before map[string]bool) {
	for name, v := range t.vars {
		if before[name] && v.Live == nil {
			continue
		}
		var guards []*Term
		for i, s := range t.frame.selections {
			if t.declaredForVariant(v.Symbol, s) {
				guards = append(guards, terms[i])
			}
		}
		guard := conjunction(guards)
		switch {
		case guard == nil:
			v.Live = nil
		case before[name]:
			v.Live = Or(v.Live, guard)
		default:
			v.Live = guard
		}
	}
}

// declaredForVariant reports whether a feature is declared for one variant alone:
// within the variant's body, or within a type of the variant that the holding
// feature's own types do not share, so that no other variant has it.
func (t *translator) declaredForVariant(sym *symbols.Symbol, s selection) bool {
	if sym == nil || s.variant == nil {
		return false
	}
	common := map[*symbols.Symbol]bool{}
	for _, shared := range []*symbols.Symbol{s.composite, s.holder[len(s.holder)-1]} {
		if shared == nil {
			continue
		}
		common[shared] = true
		for _, typ := range t.model.AllSupertypes(shared) {
			common[typ] = true
		}
	}
	own := map[*symbols.Symbol]bool{s.variant: true}
	for _, typ := range t.model.AllSupertypes(s.variant) {
		if !common[typ] {
			own[typ] = true
		}
	}
	for a := sym; a != nil; a = a.Owner() {
		if own[a] {
			return true
		}
	}
	return false
}

// selectionLabel writes the selections a frame applies its conditions under.
func (t *translator) selectionLabel() string {
	label := ""
	for i, s := range t.frame.selections {
		if i > 0 {
			label += " and "
		}
		label += pathName(s.holder) + " == " + source.QualifiedNameText(s.variant.Name)
	}
	return label
}

// featureNamed resolves a simple name as the object stating the current
// condition does: its own effective features first, then each enclosing
// object's, the element's last. The chain returned holds the features from the
// element down to the object whose feature it is.
func (t *translator) featureNamed(name string) (*symbols.Symbol, []*symbols.Symbol, bool) {
	levels := t.frame.levels
	if len(levels) == 0 {
		sym, ok := t.features[name]
		return sym, nil, ok
	}
	for i := len(levels) - 1; i >= 0; i-- {
		if sym, ok := levels[i][name]; ok {
			return sym, append([]*symbols.Symbol(nil), t.frame.path[:i]...), true
		}
	}
	return nil, nil, false
}

// enclosingHolding reads a name resolved lexically to a feature of an enclosing
// object from that object, as the evaluator does (KerML §7.4.8.1): the nearest
// object on the path whose effective feature is sym or redefines it, read
// through the chain holding that object. A name of nothing on the path is sym itself.
func (t *translator) enclosingHolding(sym *symbols.Symbol) (*symbols.Symbol, []*symbols.Symbol) {
	levels := t.frame.levels
	for i := len(levels) - 1; i >= 0; i-- {
		for _, feat := range levels[i] {
			if feat == sym || t.redefines(feat, sym) {
				return feat, append([]*symbols.Symbol(nil), t.frame.path[:i]...)
			}
		}
	}
	return sym, nil
}

// redefines reports whether feat redefines sym, directly or through a chain.
func (t *translator) redefines(feat, sym *symbols.Symbol) bool {
	for _, redefined := range t.model.AllRedefinedFeatures(feat) {
		if redefined == sym {
			return true
		}
	}
	return false
}
