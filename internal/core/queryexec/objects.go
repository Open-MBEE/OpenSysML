package queryexec

import (
	"errors"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Object rows are the runtime objects a session holds; where an element row
// reads the model, an object row reads the session's current state.

// evaluateObjects enumerates every object the session holds that is of the
// requested type, roots first in the session's order, then the objects they hold.
func (e *executor) evaluateObjects(expression queryplan.Expression) (sequence, error) {
	if err := e.requireRuntime(expression); err != nil {
		return sequence{}, err
	}
	typeName, err := e.stringArgument(expression, "type")
	if err != nil {
		return sequence{}, err
	}
	target := e.resolveClassification(typeName)
	if target == nil && !query.IsMetamodelTypeName(typeName) {
		return sequence{}, &Error{
			Kind:      ErrorUnknownClassification,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    typeName,
			Origin:    expression.Origin(),
		}
	}
	var result sequence
	err = e.eachSessionObject(expression, func(row Value) {
		if e.objectIsA(row, typeName, target) {
			result.values = append(result.values, row)
		}
	})
	return result, err
}

// requireRuntime refuses an operation over a session's objects when the
// execution has no session.
func (e *executor) requireRuntime(expression queryplan.Expression) error {
	if e.context.Runtime != nil {
		return nil
	}
	return &Error{
		Kind:      ErrorNoRuntime,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Origin:    expression.Origin(),
	}
}

// eachSessionObject visits every object the session holds once, roots first in
// the session's order, then breadth-first the objects they hold.
func (e *executor) eachSessionObject(expression queryplan.Expression, visit func(row Value)) error {
	seen := make(map[int64]struct{})
	queue := make([]Value, 0, len(e.context.Roots))
	for _, root := range e.context.Roots {
		if root.Object == nil {
			continue
		}
		if _, duplicate := seen[root.Object.ID]; duplicate {
			continue
		}
		seen[root.Object.ID] = struct{}{}
		queue = append(queue, ObjectValue(root.Object, root.Label))
	}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if !e.consumeVisit() {
			return e.budgetError(expression)
		}
		visit(next)
		children, err := e.heldObjects(expression, next)
		if err != nil {
			return err
		}
		for _, child := range children {
			inst, _, _ := child.Object()
			if _, duplicate := seen[inst.ID]; duplicate {
				continue
			}
			seen[inst.ID] = struct{}{}
			queue = append(queue, child)
		}
	}
	return nil
}

// objectDeclaration is the element an object stands for in the model: the usage
// its owner holds it as, or the declaration it was materialized from.
func objectDeclaration(inst *runtime.Instance) *symbols.Symbol {
	if usage := inst.HeldUnder(); usage != nil {
		return usage
	}
	return inst.Type
}

// objectTypes returns the types an object is of: its declaration, the type it was
// materialized from, and the classifiers a behavior gave it, each once.
func objectTypes(inst *runtime.Instance) []*symbols.Symbol {
	types := inst.Types()
	out := make([]*symbols.Symbol, 0, len(types)+1)
	if decl := objectDeclaration(inst); decl != nil {
		out = append(out, decl)
	}
	for _, typ := range types {
		if typ == nil || (len(out) > 0 && symbols.SameElement(typ, out[0])) {
			continue
		}
		out = append(out, typ)
	}
	return out
}

// objectIsA reports whether an object row is of a type: declared by an element
// of that metamodel kind, or typed by the classification or one conforming to it.
func (e *executor) objectIsA(row Value, typeName string, target *symbols.Symbol) bool {
	inst, _, _ := row.Object()
	for _, typ := range objectTypes(inst) {
		if query.MetamodelTypeNameOf(typ) == typeName {
			return true
		}
	}
	return target != nil && e.objectConforms(inst, target)
}

// objectConforms reports whether any type of an object is, or conforms to, target.
func (e *executor) objectConforms(inst *runtime.Instance, target *symbols.Symbol) bool {
	for _, typ := range objectTypes(inst) {
		if symbols.SameElement(typ, target) || e.context.Model.Conforms(typ, target) {
			return true
		}
	}
	return false
}

// heldObjects returns the objects an object row holds, each labelled under the
// row (`car.engine`, `car.wheels[2]`); an unreadable feature is a typed error.
func (e *executor) heldObjects(expression queryplan.Expression, row Value) ([]Value, error) {
	inst, label, _ := row.Object()
	held, err := e.context.Runtime.HeldObjects(inst)
	if err != nil {
		var unread *runtime.HeldObjectsError
		if errors.As(err, &unread) {
			return nil, e.unevaluable(expression, source.NameText(unread.Feature), row, unread.Err)
		}
		return nil, e.unevaluable(expression, "", row, err)
	}
	out := make([]Value, 0, len(held))
	for _, child := range held {
		out = append(out, ObjectValue(child.Instance, label+"."+child.Segment))
	}
	return out, nil
}

// objectOwner returns the object holding an object row, labelled as the row's
// label minus its last segment, and whether the row is held at all.
func (e *executor) objectOwner(row Value) (Value, bool) {
	inst, label, _ := row.Object()
	owner, _ := inst.Owner()
	if owner == nil {
		return Value{}, false
	}
	if _, ok := e.context.Runtime.Instance(owner.ID); !ok {
		return Value{}, false
	}
	return ObjectValue(owner, ownerLabel(label, owner)), true
}

// ownerLabel derives an owner's label from its held object's: the label up to
// the last `.`, or the owner's identity when the label starts at the object.
func ownerLabel(label string, owner *runtime.Instance) string {
	if held, sep, _ := splitLabel(label); sep == "." && held != "" {
		return held
	}
	return "#" + strconv.FormatInt(owner.ID, 10)
}

// lastSegment returns the segment an object row is named by within its owner:
// `wheels[2]` of `Demo::car.wheels[2]`, `car` of `Demo::car`.
func lastSegment(label string) string {
	_, _, segment := splitLabel(label)
	return segment
}

// splitLabel cuts an object label at its last `.` or `::` outside a quoted name,
// whose text may hold either: the part before, the separator (empty for none), the rest.
func splitLabel(label string) (before, sep, segment string) {
	last, width := -1, 0
	for i := 0; i < len(label); i++ {
		switch label[i] {
		case '\'':
			for i++; i < len(label) && label[i] != '\''; i++ {
				if label[i] == '\\' {
					i++
				}
			}
		case '.':
			last, width = i, 1
		case ':':
			if i+1 < len(label) && label[i+1] == ':' {
				last, width = i, 2
				i++
			}
		}
	}
	if last < 0 {
		return "", "", label
	}
	return label[:last], label[last : last+width], label[last+width:]
}

// objectPropertyValues reads a property of an object row: identity, naming and
// `type` from the session, other metadata from its declaration, else a feature.
func (e *executor) objectPropertyValues(row Value, property string) ([]Value, bool, error) {
	inst, label, _ := row.Object()
	origin := row.Origin()
	text := func(values ...string) []Value {
		out := make([]Value, 0, len(values))
		for _, value := range values {
			out = append(out, valueAt(StringValue(value), origin))
		}
		return out
	}
	switch property {
	case query.PropertyID:
		return text("#" + strconv.FormatInt(inst.ID, 10)), true, nil
	case query.PropertyName:
		return text(lastSegment(label)), true, nil
	case query.PropertyQualifiedName:
		return text(label), true, nil
	case query.PropertyOwner:
		owner, held := e.objectOwner(row)
		if !held {
			return nil, true, nil
		}
		_, ownerText, _ := owner.Object()
		return text(ownerText), true, nil
	case query.PropertyElementType:
		return text(e.objectTypeNames(inst)...), true, nil
	}
	if isQueryableProperty(property) {
		decl := objectDeclaration(inst)
		if decl == nil {
			return nil, true, nil
		}
		values, present := e.reader.Values(decl, property)
		if !present {
			return nil, true, nil
		}
		result := make([]Value, 0, len(values))
		for _, value := range values {
			result = append(result, valueAt(typedPropertyValue(property, value, decl), origin))
		}
		return result, true, nil
	}
	return e.objectFeatureValues(row, property)
}

// objectTypeNames names an object's types: a definition by its qualified name,
// a usage by the type it declares, or itself when it declares none.
func (e *executor) objectTypeNames(inst *runtime.Instance) []string {
	var names []string
	for _, typ := range objectTypes(inst) {
		if typ.Kind.IsFeature() {
			if declared, ok := e.reader.Values(typ, query.PropertyElementType); ok && len(declared) > 0 {
				names = append(names, declared...)
				continue
			}
		}
		names = append(names, symbols.FQNOf(typ))
	}
	return names
}

// objectFeatureValues reads the value a feature of an object row holds now;
// held objects stay object values, and a feature the object lacks is absent.
func (e *executor) objectFeatureValues(row Value, property string) ([]Value, bool, error) {
	inst, label, _ := row.Object()
	name, ok := objectFeatureName(e.context.Runtime, inst, property)
	if !ok {
		return nil, false, nil
	}
	fv, err := inst.GetFeatureValue(e.context.Runtime, name)
	if err != nil {
		return nil, true, e.unevaluable(queryplan.Expression{}, property, row, err)
	}
	if fv == nil {
		return nil, true, nil
	}
	segment := label + "." + source.NameText(name)
	held := fv.Value
	if fv.Values.Kind != runtime.ValInvalid {
		held = fv.Values
	}
	values, err := e.objectCellValues(row, property, held, segment)
	return values, true, err
}

// collectionElements returns the elements of a collection feature value.
func collectionElements(value runtime.Value) []runtime.Value {
	switch {
	case value.Kind == runtime.ValSequence && value.Sequence() != nil:
		return value.Sequence().Elements()
	case value.Kind == runtime.ValSet && value.Set() != nil:
		return value.Set().Elements()
	}
	return nil
}

// objectFeatureName finds the feature of an object a property names, by its
// declared name or the text of a quoted one, in the object's feature order.
func objectFeatureName(ctx *runtime.Context, inst *runtime.Instance, property string) (string, bool) {
	if _, ok := inst.FeatureValues[property]; ok {
		return property, true
	}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if source.NameText(of.Name) == property {
			return of.Name, true
		}
	}
	return "", false
}

// objectCellValues converts one held value to cell values: an object stays an
// object value under its label, null is absent, else as a declared value.
func (e *executor) objectCellValues(row Value, property string, value runtime.Value, label string) ([]Value, error) {
	if id, isObject := value.Object(); isObject {
		if e.context.Runtime.HoldsNoValue(value) {
			return nil, nil
		}
		child, ok := e.context.Runtime.Instance(id)
		if !ok {
			return nil, e.unevaluable(queryplan.Expression{}, property, row, notAValue(value))
		}
		return []Value{ObjectValue(child, label)}, nil
	}
	if value.Kind != runtime.ValSequence && value.Kind != runtime.ValSet {
		return e.cellValues(value, property, row)
	}
	var result []Value
	for i, element := range collectionElements(value) {
		values, err := e.objectCellValues(row, property, element, label+"["+strconv.Itoa(i+1)+"]")
		if err != nil {
			return nil, err
		}
		result = append(result, values...)
	}
	return result, nil
}
