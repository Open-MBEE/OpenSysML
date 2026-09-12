package runtime

import "fmt"

// Bring answers, for an object of another context, the object of this one that
// stands for it; an error when none does.
type Bring func(id int64) (*Instance, error)

// NotPortableError reports a value bound to the run that made it, which no other
// context can hold.
type NotPortableError struct {
	Kind   ValueKind
	Reason string
}

func (e *NotPortableError) Error() string {
	return fmt.Sprintf("a %s %s, which no other context carries", e.Kind, e.Reason)
}

// Carry is v as a value of ctx: a number, string, quantity or the like as it is;
// every object it names is the one bring answers for it; a function is read again
// here, bound to what bring answers for its self. A deferred expression closed over
// its environment, or a function over a behavior body, cannot be carried.
func (ctx *Context) Carry(v Value, bring Bring) (Value, error) {
	c := &carrying{ctx: ctx, bring: bring}
	return c.value(v)
}

// unknownKindReason is the refusal of a kind Carry does not dispatch;
// TestEveryValueKindIsDispatched checks no kind meets it.
const unknownKindReason = "is of a kind no context carries over"

// carrying carries values into ctx, objects through bring.
type carrying struct {
	ctx   *Context
	bring Bring
}

// object is the id here of an object of the other context; 0 stands for none.
func (c *carrying) object(id int64) (int64, error) {
	if id == 0 {
		return 0, nil
	}
	inst, err := c.bring(id)
	if err != nil {
		return 0, err
	}
	return inst.ID, nil
}

func (c *carrying) values(vals []Value) ([]Value, error) {
	out := make([]Value, 0, len(vals))
	for _, v := range vals {
		carried, err := c.value(v)
		if err != nil {
			return nil, err
		}
		out = append(out, carried)
	}
	return out, nil
}

func (c *carrying) value(v Value) (Value, error) {
	switch v.Kind {
	case ValInvalid, ValConst, ValNull, ValString, ValQuantity, ValEnumLiteral, ValComplex, ValMeasurementRef,
		ValMetaobject:
		return v, nil
	case ValInstance:
		id, err := c.object(v.Instance)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValInstance, Instance: id}, nil
	case ValVariant:
		id, err := c.object(v.Instance)
		if err != nil {
			return Value{}, err
		}
		return NewVariantValue(v.Variant(), id), nil
	case ValSequence:
		seq := v.Sequence()
		if seq == nil {
			return v, nil
		}
		elements, err := c.values(seq.elements)
		if err != nil {
			return Value{}, err
		}
		return NewSequenceValue(&Sequence{elements: elements, elementUnit: seq.elementUnit}), nil
	case ValSet:
		set := v.Set()
		if set == nil {
			return v, nil
		}
		out := NewSetIn(c.ctx)
		for _, e := range set.Elements() {
			carried, err := c.value(e)
			if err != nil {
				return Value{}, err
			}
			out.Add(carried)
		}
		return NewSetValue(out), nil
	case ValExpr:
		if ev, ok := v.ref.(*exprValue); ok && ev != nil && ev.env != nil {
			return Value{}, &NotPortableError{Kind: v.Kind, Reason: "closes over the environment it was written in"}
		}
		return v, nil
	case ValArray:
		a := v.Array()
		elements, err := c.values(a.Elements)
		if err != nil {
			return Value{}, err
		}
		object, err := c.object(a.Object)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValArray, ref: &Array{Dimensions: a.Dimensions, Elements: elements, Object: object}}, nil
	case ValVector:
		vec := v.Vector()
		object, err := c.object(vec.Object)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValVector, ref: &Vector{Elements: vec.Elements, Object: object}}, nil
	case ValVectorQuantity:
		vq := v.VectorQuantity()
		frame, err := c.frame(vq.Frame)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValVectorQuantity, ref: &VectorQuantity{Num: vq.Num, Units: vq.Units, Frame: frame}}, nil
	case ValTensorQuantity:
		tq := *v.TensorQuantity()
		mref, err := c.object(tq.MRef)
		if err != nil {
			return Value{}, err
		}
		tq.MRef = mref
		return Value{Kind: ValTensorQuantity, ref: &tq}, nil
	case ValCoordinateFrame:
		frame, err := c.frame(v.CoordinateFrame())
		if err != nil {
			return Value{}, err
		}
		return NewCoordinateFrameValue(frame), nil
	case ValCoordinateTransformation:
		t, err := c.transformation(v.CoordinateTransformation())
		if err != nil {
			return Value{}, err
		}
		return NewCoordinateTransformationValue(t), nil
	case ValFunction:
		return c.function(v)
	}
	return Value{}, &NotPortableError{Kind: v.Kind, Reason: unknownKindReason}
}

// function reads the calc a function value is of again here, bound to the object
// bring answers for its self; one closing over a behavior body cannot be.
func (c *carrying) function(v Value) (Value, error) {
	fn := v.function()
	if fn == nil || fn.shape == nil {
		return v, nil
	}
	if len(fn.enclosing) > 0 {
		return Value{}, &NotPortableError{Kind: v.Kind, Reason: fmt.Sprintf("%s closes over the bindings of the behavior body it was read in", fn.shape.Name)}
	}
	shape, err := c.ctx.calcInterfaceOf(fn.shape.Sym)
	if err != nil {
		return Value{}, err
	}
	carried := &functionValue{shape: shape, scope: fn.scope}
	carried.library, _ = c.ctx.libraryFunctionFor(fn.shape.Sym)
	if fn.self != nil {
		if carried.self, err = c.bring(fn.self.ID); err != nil {
			return Value{}, err
		}
	}
	return Value{Kind: ValFunction, ref: carried}, nil
}

func (c *carrying) frame(f *CoordinateFrame) (*CoordinateFrame, error) {
	if f == nil {
		return nil, nil
	}
	out := *f
	var err error
	if out.Object, err = c.object(f.Object); err != nil {
		return nil, err
	}
	if out.Transformation, err = c.transformation(f.Transformation); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *carrying) transformation(t *CoordinateTransformation) (*CoordinateTransformation, error) {
	if t == nil {
		return nil, nil
	}
	out := *t
	var err error
	if out.Object, err = c.object(t.Object); err != nil {
		return nil, err
	}
	if out.Source, err = c.frame(t.Source); err != nil {
		return nil, err
	}
	if out.Target, err = c.frame(t.Target); err != nil {
		return nil, err
	}
	if t.Placement != nil {
		placement := FramePlacement{}
		if placement.Origin, err = c.value(t.Placement.Origin); err != nil {
			return nil, err
		}
		if placement.BasisDirections, err = c.values(t.Placement.BasisDirections); err != nil {
			return nil, err
		}
		out.Placement = &placement
	}
	if len(t.Sequence) > 0 {
		out.Sequence = make([]FrameStep, len(t.Sequence))
		for i, step := range t.Sequence {
			if out.Sequence[i], err = c.step(step); err != nil {
				return nil, err
			}
		}
	}
	return &out, nil
}

func (c *carrying) step(step FrameStep) (FrameStep, error) {
	out := step
	var err error
	if out.Object, err = c.object(step.Object); err != nil {
		return FrameStep{}, err
	}
	if out.Translation, err = c.vector(step.Translation); err != nil {
		return FrameStep{}, err
	}
	if out.Axis, err = c.vector(step.Axis); err != nil {
		return FrameStep{}, err
	}
	return out, nil
}

func (c *carrying) vector(v *Value) (*Value, error) {
	if v == nil {
		return nil, nil
	}
	carried, err := c.value(*v)
	if err != nil {
		return nil, err
	}
	return &carried, nil
}
