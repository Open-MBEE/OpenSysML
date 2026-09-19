package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// framePose is where a target frame sits in its source frame: a source-coordinate
// point maps as `v_source = origin + basis · v_target`, so transform applies the inverse.
type framePose struct {
	origin []float64
	basis  [][]float64
}

// identityPose is the pose of a target frame coincident with its source.
func identityPose(n int) framePose {
	pose := framePose{origin: make([]float64, n), basis: make([][]float64, n)}
	for i := range pose.basis {
		pose.basis[i] = make([]float64, n)
		pose.basis[i][i] = 1
	}
	return pose
}

// reorients reports a pose whose basis is not the source's own: one that mixes
// the axes, as a pure translation does not.
func (p framePose) reorients() bool {
	for i, row := range p.basis {
		for j, x := range row {
			if (i == j && x != 1) || (i != j && x != 0) {
				return true
			}
		}
	}
	return false
}

// transformVector is VectorCalculations::transform: sourceVector, over the
// transformation's source frame, as coordinates in its target frame.
func transformVector(name string, ctx *Context, args []Value) (Value, error) {
	if args[0].Kind != ValCoordinateTransformation {
		return Value{}, fmt.Errorf("%w: function %s parameter %q requires a coordinate transformation, got %s",
			ErrTypeMismatch, name, "transformation", describeValue(args[0]))
	}
	t := args[0].CoordinateTransformation()
	if t.Source == nil || t.Target == nil {
		return Value{}, fmt.Errorf("%w: function %s: %s states no source or no target", ErrNoValue, name, t.Name())
	}
	vq := args[1].VectorQuantity()
	if vq == nil {
		return Value{}, fmt.Errorf("%w: function %s parameter %q requires a vector quantity over %s, got %s",
			ErrTypeMismatch, name, "sourceVector", t.Source.Name(), describeValue(args[1]))
	}
	if vq.Frame == nil {
		return Value{}, fmt.Errorf("%w: function %s: sourceVector.mRef must be %s, the source of %s, but %s is written over the unit %s and no coordinate frame",
			ErrTypeMismatch, name, t.Source.Name(), t.Name(), vq.format(semantics.FormatConst), sharedUnitText(vq))
	}
	if !vq.Frame.equal(t.Source) {
		return Value{}, fmt.Errorf("%w: function %s: sourceVector.mRef is %s, not %s, the source of %s",
			ErrTypeMismatch, name, vq.Frame.Name(), t.Source.Name(), t.Name())
	}
	if err := t.sameDimensions("function " + name); err != nil {
		return Value{}, err
	}
	pose, err := ctx.poseOf(name, t)
	if err != nil {
		return Value{}, err
	}
	target, err := ctx.applyPose(name, t, pose, vq)
	if err != nil {
		return Value{}, err
	}
	return NewFramedVectorQuantityValue(target, t.Target), nil
}

// sharedUnitText names the one unit a frameless vector quantity carries, if one.
func sharedUnitText(vq *VectorQuantity) string {
	if unit, ok := vq.sharedUnit(); ok {
		return unit.String()
	}
	return "per axis"
}

// poseOf computes the target frame's pose in the source frame from whichever
// shape the transformation has; an unrecognized subtype has no pose to apply.
func (ctx *Context) poseOf(name string, t *CoordinateTransformation) (framePose, error) {
	n := len(t.Source.Axes)
	switch {
	case t.Placement != nil:
		return ctx.placementPose(name, t, n)
	case t.Sequence != nil:
		return ctx.sequencePose(name, t, n)
	case t.Affine != nil:
		return affinePose(name, t, n)
	}
	return framePose{}, fmt.Errorf("%w: function %s: %s is a %s, a CoordinateTransformation of no shape the library gives a meaning: not a CoordinateFramePlacement, TranslationRotationSequence or AffineTransformationMatrix3d",
		ErrUnevaluableLibraryFunction, name, t.Name(), symbolText(t.Type))
}

// placementPose reads origin and basisDirections: the origin as the target's
// origin in source coordinates, each direction normalized to a basis vector.
func (ctx *Context) placementPose(name string, t *CoordinateTransformation, n int) (framePose, error) {
	pose := identityPose(n)
	origin, err := ctx.sourceComponents(name, t, "origin", t.Placement.Origin)
	if err != nil {
		return framePose{}, err
	}
	pose.origin = origin
	dirs := t.Placement.BasisDirections
	if len(dirs) == 0 {
		return pose, nil
	}
	if len(dirs) != n {
		return framePose{}, fmt.Errorf("%w: function %s: %s states %d basisDirections over %s of %d axes; a placement states none or one per axis",
			ErrMultiplicityViolation, name, t.Name(), len(dirs), t.Source.Name(), n)
	}
	for j, dir := range dirs {
		column, err := ctx.sourceComponents(name, t, fmt.Sprintf("basisDirections#(%d)", j+1), dir)
		if err != nil {
			return framePose{}, err
		}
		if !normalize(column) {
			return framePose{}, fmt.Errorf("%w: function %s: %s: basisDirections#(%d) is the zero vector, which points nowhere",
				semantics.ErrArithmeticDomain, name, t.Name(), j+1)
		}
		for i := range column {
			pose.basis[i][j] = column[i]
		}
	}
	return pose, nil
}

// sequencePose applies translations and rotations in order to the pose of a
// frame starting coincident with the source.
func (ctx *Context) sequencePose(name string, t *CoordinateTransformation, n int) (framePose, error) {
	pose := identityPose(n)
	for i, step := range t.Sequence {
		what := fmt.Sprintf("elements#(%d)", i+1)
		if step.Translation != nil {
			shift, err := ctx.sourceComponents(name, t, what+".translationVector", *step.Translation)
			if err != nil {
				return framePose{}, err
			}
			for k := range shift {
				pose.origin[k] += shift[k]
			}
			continue
		}
		if n != 3 {
			return framePose{}, fmt.Errorf("%w: function %s: %s: %s rotates about an axis, which a frame of %d axes has no meaning for",
				ErrUnevaluableLibraryFunction, name, t.Name(), what, n)
		}
		axis, err := ctx.sourceComponents(name, t, what+".axisDirection", *step.Axis)
		if err != nil {
			return framePose{}, err
		}
		if !normalize(axis) {
			return framePose{}, fmt.Errorf("%w: function %s: %s: %s.axisDirection is the zero vector, which points nowhere",
				semantics.ErrArithmeticDomain, name, t.Name(), what)
		}
		angle, ok := angleArgument(ctx, NewQuantityValue(step.Angle))
		if !ok {
			return framePose{}, fmt.Errorf("%w: function %s: %s: %s.angle is %s, not an angular measure",
				ErrIncommensurableUnits, name, t.Name(), what, step.Angle.String())
		}
		rotation := rodrigues(axis, angle.AsReal())
		if step.Intrinsic {
			pose.basis = matMul(pose.basis, rotation)
		} else {
			pose.basis = matMul(rotation, pose.basis)
		}
	}
	return pose, nil
}

// affinePose reads the 3×3 rotation and the translation, in the source's axis units.
func affinePose(name string, t *CoordinateTransformation, n int) (framePose, error) {
	if n != 3 {
		return framePose{}, fmt.Errorf("%w: function %s: %s is an AffineTransformationMatrix3d over %s of %d axes; it asserts source.dimensions == 3",
			ErrMultiplicityViolation, name, t.Name(), t.Source.Name(), n)
	}
	pose := identityPose(3)
	for i := 0; i < 3; i++ {
		pose.origin[i] = t.Affine.Translation[i]
		for j := 0; j < 3; j++ {
			pose.basis[i][j] = t.Affine.Rotation[3*i+j]
		}
	}
	return pose, nil
}

// sourceComponents reads a vector stated by the transformation as magnitudes in
// the source frame's axis units: one written over another frame is in the wrong
// frame, one over a unit converts each component to its axis.
func (ctx *Context) sourceComponents(name string, t *CoordinateTransformation, what string, val Value) ([]float64, error) {
	vq := val.VectorQuantity()
	if vq == nil {
		return nil, fmt.Errorf("%w: function %s: %s: %s is %s, not a vector quantity over %s",
			ErrTypeMismatch, name, t.Name(), what, describeValue(val), t.Source.Name())
	}
	if vq.Frame != nil && !vq.Frame.equal(t.Source) {
		return nil, fmt.Errorf("%w: function %s: %s: %s is a vector quantity in %s, not a vector quantity over %s, its source",
			ErrTypeMismatch, name, t.Name(), what, vq.Frame.Name(), t.Source.Name())
	}
	axes := t.Source.Axes
	if vq.Dimension() != len(axes) {
		return nil, fmt.Errorf("%w: function %s: %s: %s has %d components over %s of %d axes",
			ErrMultiplicityViolation, name, t.Name(), what, vq.Dimension(), t.Source.Name(), len(axes))
	}
	out := make([]float64, len(axes))
	for i := range axes {
		converted, err := semantics.ConvertQuantity(*vq.component(i), axes[i])
		if err != nil {
			return nil, fmt.Errorf("%w: function %s: %s: %s: %w", ErrIncommensurableUnits, name, t.Name(), what, err)
		}
		out[i] = converted.Num.AsReal()
	}
	return out, nil
}

// applyPose is the target coordinates of a source vector: `basis⁻¹ (v − origin)`,
// computed in the source axis units and answered in the target's.
func (ctx *Context) applyPose(name string, t *CoordinateTransformation, pose framePose, vq *VectorQuantity) ([]semantics.Value, error) {
	n := vq.Dimension()
	v := make([]float64, n)
	for i := range v {
		v[i] = vq.Num[i].AsReal() - pose.origin[i]
	}
	if pose.reorients() {
		// A basis change mixes the axes, so they must share one unit to mix.
		for i := 1; i < n; i++ {
			if !(&MeasurementRef{Unit: t.Source.Axes[0]}).equal(&MeasurementRef{Unit: t.Source.Axes[i]}) {
				return nil, fmt.Errorf("%w: function %s: %s reorients %s, whose axes %s and %s are in different units and cannot mix",
					ErrIncommensurableUnits, name, t.Name(), t.Source.Name(), t.Source.Axes[0], t.Source.Axes[i])
			}
		}
		inverse, ok := invert(pose.basis)
		if !ok {
			return nil, fmt.Errorf("%w: function %s: %s: its basis directions are linearly dependent and span no frame",
				semantics.ErrArithmeticDomain, name, t.Name())
		}
		v = matVec(inverse, v)
	}
	out := make([]semantics.Value, n)
	for i := range out {
		num, err := semantics.RealResult(v[i])
		if err != nil {
			return nil, fmt.Errorf("function %s: %w", name, err)
		}
		q, err := semantics.ConvertQuantity(Quantity{Num: num, Unit: t.Source.Axes[i]}, t.Target.Axes[i])
		if err != nil {
			return nil, fmt.Errorf("%w: function %s: %s: axis %d of %s to axis %d of %s: %w",
				ErrIncommensurableUnits, name, t.Name(), i+1, t.Source.Name(), i+1, t.Target.Name(), err)
		}
		out[i] = q.Num
	}
	return out, nil
}

// normalize scales v to unit length; false for the zero vector, which has no direction.
func normalize(v []float64) bool {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return false
	}
	norm := math.Sqrt(sum)
	for i := range v {
		v[i] /= norm
	}
	return true
}

// rodrigues is the right-handed rotation by angle radians about the unit axis.
func rodrigues(axis []float64, angle float64) [][]float64 {
	c, s := math.Cos(angle), math.Sin(angle)
	x, y, z := axis[0], axis[1], axis[2]
	t := 1 - c
	return [][]float64{
		{c + x*x*t, x*y*t - z*s, x*z*t + y*s},
		{y*x*t + z*s, c + y*y*t, y*z*t - x*s},
		{z*x*t - y*s, z*y*t + x*s, c + z*z*t},
	}
}

// matMul is the product of two square matrices.
func matMul(a, b [][]float64) [][]float64 {
	n := len(a)
	out := make([][]float64, n)
	for i := range out {
		out[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

// matVec is the product of a square matrix and a vector.
func matVec(m [][]float64, v []float64) []float64 {
	out := make([]float64, len(v))
	for i, row := range m {
		for j, x := range row {
			out[i] += x * v[j]
		}
	}
	return out
}

// invert is the inverse of a square matrix by Gauss–Jordan elimination with
// partial pivoting; false for a singular matrix.
func invert(m [][]float64) ([][]float64, bool) {
	n := len(m)
	a := make([][]float64, n)
	inv := identityPose(n).basis
	for i := range a {
		a[i] = append([]float64(nil), m[i]...)
	}
	for col := 0; col < n; col++ {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(a[pivot][col]) < 1e-12 {
			return nil, false
		}
		a[col], a[pivot] = a[pivot], a[col]
		inv[col], inv[pivot] = inv[pivot], inv[col]
		p := a[col][col]
		for j := 0; j < n; j++ {
			a[col][j] /= p
			inv[col][j] /= p
		}
		for r := 0; r < n; r++ {
			if r == col || a[r][col] == 0 {
				continue
			}
			f := a[r][col]
			for j := 0; j < n; j++ {
				a[r][j] -= f * a[col][j]
				inv[r][j] -= f * inv[col][j]
			}
		}
	}
	return inv, true
}
