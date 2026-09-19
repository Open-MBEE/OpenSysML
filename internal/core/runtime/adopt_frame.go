package runtime

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// planFrame rebinds what a frame itself names — its declaration, its type and the
// units of its axes — so a frame read before the re-analysis keeps its identity.
// The frames and values its transformation relates are visited by walkValue.
func (a *adoption) planFrame(f *CoordinateFrame) error {
	if f.Decl != nil {
		if _, err := a.rebind(f.Decl, "the coordinate frame "+f.Name()+" it holds"); err != nil {
			return err
		}
	}
	if f.Type != nil {
		if _, err := a.rebind(f.Type, "the type of the coordinate frame "+f.Name()); err != nil {
			return err
		}
	}
	for _, axis := range f.Axes {
		if err := a.planUnit(axis); err != nil {
			return err
		}
	}
	if f.Scale == nil {
		return nil
	}
	if err := a.planUnit(f.Scale.Unit); err != nil {
		return err
	}
	if f.Scale.Mapping != nil {
		for _, q := range []Quantity{f.Scale.Mapping.Mapped, f.Scale.Mapping.Reference} {
			if err := a.planUnit(q.Unit); err != nil {
				return err
			}
		}
	}
	return nil
}

// planTransformation rebinds what a transformation itself names: its declaration
// and its type. The frames and placement values it relates are visited by walkValue.
func (a *adoption) planTransformation(t *CoordinateTransformation) error {
	if t.Decl != nil {
		if _, err := a.rebind(t.Decl, "the coordinate transformation "+t.Name()+" it holds"); err != nil {
			return err
		}
	}
	if t.Type != nil {
		if _, err := a.rebind(t.Type, "the type of the coordinate transformation "+t.Name()); err != nil {
			return err
		}
	}
	return nil
}

// placementValues are the values a transformation's placement is stated by: the
// origin and basis directions, or each step's vector and angle.
func (t *CoordinateTransformation) placementValues() []Value {
	var out []Value
	if t.Placement != nil {
		out = append(out, t.Placement.Origin)
		out = append(out, t.Placement.BasisDirections...)
	}
	for _, step := range t.Sequence {
		if step.Translation != nil {
			out = append(out, *step.Translation)
		}
		if step.Axis != nil {
			out = append(out, *step.Axis)
		}
		if step.Angle != nil {
			out = append(out, NewQuantityValue(step.Angle))
		}
	}
	return out
}

// rewriteFrame is the frame as this context holds it, every symbol it names
// rebound; a frame reached twice (through its own transformation) is one frame.
func (a *adoption) rewriteFrame(f *CoordinateFrame, seen map[*CoordinateFrame]*CoordinateFrame) *CoordinateFrame {
	if f == nil {
		return nil
	}
	if out, ok := seen[f]; ok {
		return out
	}
	out := *f
	seen[f] = &out
	out.Decl = a.reboundSymbol(f.Decl)
	out.Type = a.reboundSymbol(f.Type)
	out.Axes = make([]Unit, len(f.Axes))
	for i, axis := range f.Axes {
		out.Axes[i] = a.rewriteUnit(axis)
	}
	if f.Scale != nil {
		scale := *f.Scale
		scale.Unit = a.rewriteUnit(f.Scale.Unit)
		if f.Scale.Mapping != nil {
			mapping := *f.Scale.Mapping
			mapping.Mapped.Unit = a.rewriteUnit(mapping.Mapped.Unit)
			mapping.Reference.Unit = a.rewriteUnit(mapping.Reference.Unit)
			scale.Mapping = &mapping
		}
		out.Scale = &scale
	}
	out.Transformation = a.rewriteTransformation(f.Transformation, seen)
	return &out
}

// rewriteTransformation is the transformation as this context holds it.
func (a *adoption) rewriteTransformation(t *CoordinateTransformation, seen map[*CoordinateFrame]*CoordinateFrame) *CoordinateTransformation {
	if t == nil {
		return nil
	}
	out := *t
	out.Decl = a.reboundSymbol(t.Decl)
	out.Type = a.reboundSymbol(t.Type)
	out.Source = a.rewriteFrame(t.Source, seen)
	out.Target = a.rewriteFrame(t.Target, seen)
	if t.Placement != nil {
		placement := FramePlacement{Origin: a.rewrite(t.Placement.Origin)}
		for _, dir := range t.Placement.BasisDirections {
			placement.BasisDirections = append(placement.BasisDirections, a.rewrite(dir))
		}
		out.Placement = &placement
	}
	if t.Sequence != nil {
		out.Sequence = make([]FrameStep, len(t.Sequence))
		for i, step := range t.Sequence {
			out.Sequence[i] = step
			if step.Translation != nil {
				v := a.rewrite(*step.Translation)
				out.Sequence[i].Translation = &v
			}
			if step.Axis != nil {
				v := a.rewrite(*step.Axis)
				out.Sequence[i].Axis = &v
			}
			if step.Angle != nil {
				angle := *step.Angle
				angle.Unit = a.rewriteUnit(angle.Unit)
				out.Sequence[i].Angle = &angle
			}
		}
	}
	if t.Affine != nil {
		affine := *t.Affine
		out.Affine = &affine
	}
	return &out
}

// reboundSymbol is the symbol the plan rebound sym to, or sym itself where the
// plan named nothing for it.
func (a *adoption) reboundSymbol(sym *symbols.Symbol) *symbols.Symbol {
	if found, ok := a.rebound[sym]; ok {
		return found
	}
	return sym
}
