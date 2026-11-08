package naming

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Table is the qualified-name side table of one document: the name of every
// member node, by which the export, the query API and the symbol facts address it.
type Table struct {
	// FQN is the qualified name of each member node, which is how a succession
	// end the notation leaves unnamed addresses the member it binds.
	FQN map[ast.Node]string
	// Positional is the name a graph stating no effective names gives a member
	// (a declared name claimed among declared names, else its position).
	Positional map[ast.Node]string
	// Declared holds every qualified name a member of the document claimed.
	Declared map[string]bool
	// Absorbed marks the FinalNode a member-attached `then` introduces: the
	// target end of the following succession, written as that edge, not itself.
	Absorbed map[ast.Node]bool
	// Payloads holds the PayloadFeature each flow's `of` clause declares, keyed
	// by the flow: the declared feature of `of p : T`, or one made up for `of T`.
	Payloads        map[*ast.Usage]*ast.Usage
	PayloadFeatures map[*ast.Usage]bool

	positionalTaken map[string]bool
	consumed        map[ast.Node]bool
	declSym         map[ast.Node]*symbols.Symbol
	model           *semantics.Model
	file            *source.SourceFile
}

// PayloadCollision is the error of a flow payload identified as a body member
// is too, which no naming can tell apart.
type PayloadCollision struct {
	Payload *ast.Usage
	Name    string
}

func (e *PayloadCollision) Error() string {
	return fmt.Sprintf("the payload at offset %d is identified as %s, which a body member is named too", e.Payload.Span().Offset, e.Name)
}

// Build names every member of root. The identity table says which annotations
// were consumed into element ids, which then take no position among members.
func Build(file *source.SourceFile, root *ast.RootNamespace, model *semantics.Model, ids *identity.Table) (*Table, error) {
	t := &Table{
		FQN:             map[ast.Node]string{},
		Positional:      map[ast.Node]string{},
		Declared:        map[string]bool{},
		Absorbed:        map[ast.Node]bool{},
		Payloads:        map[*ast.Usage]*ast.Usage{},
		PayloadFeatures: map[*ast.Usage]bool{},
		positionalTaken: map[string]bool{},
		consumed:        map[ast.Node]bool{},
		declSym:         map[ast.Node]*symbols.Symbol{},
		model:           model,
		file:            file,
	}
	if ids != nil {
		for _, sym := range ids.Symbols() {
			info, ok := ids.Info(sym)
			if !ok {
				continue
			}
			if sym.Decl != nil {
				t.declSym[sym.Decl] = sym
			}
			if info.Annotated {
				for _, d := range info.Declarations {
					t.consumed[d.Node] = true
				}
			}
			if info.Scope != nil {
				for _, d := range info.Scope.Declarations {
					t.consumed[d.Node] = true
				}
			}
		}
	}
	if err := t.collect(root.Members, "", ""); err != nil {
		return nil, err
	}
	return t, nil
}

// Skip reports whether node is an identity annotation consumed into element
// ids rather than a member of the model.
func (t *Table) Skip(node ast.Node) bool { return t.consumed[node] }

// Kept filters members to the ones that take a position: every member but a
// consumed identity annotation.
func (t *Table) Kept(members []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if node, _ := UnwrapMember(member); node != nil && t.Skip(node) {
			continue
		}
		out = append(out, member)
	}
	return out
}

// EffectiveName is MemberName completed by the model: a feature its clause
// leaves nameless is named by the feature it implicitly redefines (KerML 1.1 §8.3.3.3).
func (t *Table) EffectiveName(node ast.Node) string {
	if name := MemberName(node); name != "" {
		return name
	}
	// Only a usage or a subject is so identified: a Membership (`first start;`) names nothing,
	// a chain-referencing `require q.k;` stays positional, a body's node keys its parameter.
	switch node.(type) {
	case *ast.Usage, *ast.SubjectMember:
	default:
		return ""
	}
	if sym := t.declSym[node]; sym != nil && sym.IsFeature() && t.model != nil {
		return t.model.EffectiveNameOf(sym)
	}
	return ""
}

func (t *Table) collect(members []ast.Node, owner, position string) error {
	kept := t.Kept(members)
	for i, member := range kept {
		node, _ := UnwrapMember(member)
		if _, done := node.(*ast.FinalNode); done && i+1 < len(kept) {
			if next, _ := UnwrapMember(kept[i+1]); next != nil {
				if edge, ok := next.(*ast.SuccessionEdge); ok && edge.TargetMember == node {
					t.Absorbed[node] = true
				}
			}
		}
		declared, children := DeclaredNameAndMembers(node)
		name := t.EffectiveName(node)
		fqn := Qualify(owner, name, i)
		if fqn == "" {
			continue
		}
		// A member whose own name is taken is identified by its position.
		if name != "" && t.Declared[fqn] {
			fqn = Qualify(owner, "", i)
		}
		t.FQN[node] = fqn
		positional := Qualify(position, declared, i)
		if declared != "" && t.positionalTaken[positional] {
			positional = Qualify(position, "", i)
		}
		t.positionalTaken[positional] = true
		if positional != fqn {
			t.Positional[node] = positional
		}
		// A trigger's payload is a parameter of the transition's trigger action.
		if transition, ok := node.(*ast.TransitionMember); ok {
			if payload := TriggerPayload(t.file, transition); payload != nil && payload == transition.Trigger {
				t.FQN[payload] = Qualify(fqn+"::"+TriggerSegment, ParamName(payload), 0)
			}
		}
		t.Declared[fqn] = true
		if err := t.collect(children, fqn, positional); err != nil {
			return err
		}
		if err := t.collectPrefixes(node, fqn, len(t.Kept(children))); err != nil {
			return err
		}
		t.collectCrossFeature(node, fqn)
		if err := t.collectPayload(node, fqn); err != nil {
			return err
		}
	}
	return nil
}

// collectPayload names the payload feature a flow's `of` clause declares, which
// the flow owns after its body members and prefixes; `of T` is named by position.
func (t *Table) collectPayload(node ast.Node, owner string) error {
	u, ok := node.(*ast.Usage)
	if !ok {
		return nil
	}
	payload := t.PayloadOf(u)
	if payload == nil {
		return nil
	}
	index := t.CrossFeatureIndex(u)
	fqn := Qualify(owner, payload.Ident.Name, index)
	// A payload whose name a body member took first is identified by its position.
	if payload.Ident.Name != "" && t.Declared[fqn] {
		fqn = Qualify(owner, "", index)
	}
	if t.Declared[fqn] {
		return &PayloadCollision{Payload: payload, Name: fqn}
	}
	t.FQN[payload] = fqn
	t.Declared[fqn] = true
	return nil
}

// PayloadOf returns the PayloadFeature a flow's `of` clause declares, or nil:
// `of T` and `of T[1]` state only its typing, so a feature is made up to carry them.
func (t *Table) PayloadOf(u *ast.Usage) *ast.Usage {
	if u.FlowEnds == nil || u.FlowEnds.Payload == nil && u.FlowEnds.PayloadDecl == nil {
		return nil
	}
	if payload, ok := t.Payloads[u]; ok {
		return payload
	}
	payload := u.FlowEnds.PayloadDecl
	if payload == nil {
		payload = u.FlowEnds.ImpliedPayload()
	}
	t.Payloads[u] = payload
	t.PayloadFeatures[payload] = true
	return payload
}

// collectCrossFeature names the cross feature an end declares ahead of itself
// (`end x1 [m] feature x`), which it owns after its prefixes.
func (t *Table) collectCrossFeature(node ast.Node, owner string) {
	u, ok := node.(*ast.Usage)
	if !ok || u.CrossFeature == nil {
		return
	}
	cross := u.CrossFeature
	fqn := Qualify(owner, cross.Ident.Name, t.CrossFeatureIndex(u))
	if t.Declared[fqn] {
		fqn = Qualify(owner, "", t.CrossFeatureIndex(u))
	}
	t.FQN[cross] = fqn
	t.Declared[fqn] = true
}

// CrossFeatureIndex is the position an end's cross feature takes among the
// members it owns: after its kept body members and its prefix annotations.
func (t *Table) CrossFeatureIndex(u *ast.Usage) int {
	index := len(t.Kept(BodyMembers(u)))
	for _, prefix := range u.Prefixes {
		if prefix != nil && !t.Skip(prefix) {
			index++
		}
	}
	return index
}

// collectPrefixes names a declaration's `#M` prefix annotations, which take the
// positions after its kept body members, and the members of their bodies.
func (t *Table) collectPrefixes(node ast.Node, owner string, index int) error {
	for _, prefix := range DeclaredPrefixes(node) {
		if prefix == nil || t.Skip(prefix) {
			continue
		}
		fqn := Qualify(owner, "", index)
		t.FQN[prefix] = fqn
		t.Declared[fqn] = true
		if err := t.collect(prefix.Body, fqn, fqn); err != nil {
			return err
		}
		index++
	}
	return nil
}
