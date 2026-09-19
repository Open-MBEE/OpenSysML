package semantics

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	fqnCoordinateFrame            = "MeasurementReferences::CoordinateFrame"
	fqnVectorMeasurementReference = "MeasurementReferences::VectorMeasurementReference"
	fqnScalarMeasurementReference = "MeasurementReferences::ScalarMeasurementReference"
	fqnVectorQuantityValue        = "Quantities::VectorQuantityValue"
	fqnArray                      = "Collections::Array"
	memberMRefs                   = "mRefs"
	memberDimensions              = "dimensions"
)

// ComposedFrame is what a frame composed by 'CoordinateFrame*' or 'CoordinateFrame/'
// (`spatialCF / s`) is known to be: its dimensions and axis dimensions, when fixed.
type ComposedFrame struct {
	Dimensions    []int64
	HasDimensions bool
	// AxisDimensions is one per axis, or the one every axis measures in when Uniform;
	// nil when the axes are not statically known.
	AxisDimensions []UnitTerm
	Uniform        bool
}

// IsCoordinateFrame reports whether typ is CoordinateFrame or a specialization:
// a frame, or a scale, since an IntervalScale is a one-dimensional frame.
func (m *Model) IsCoordinateFrame(typ *symbols.Symbol) bool {
	if m == nil || typ == nil {
		return false
	}
	frame := m.libSymbol(fqnCoordinateFrame)
	return frame != nil && m.Conforms(typ, frame)
}

// IsVectorReference reports whether sym is a VectorMeasurementReference and not a
// scalar one: a coordinate frame, but not a unit or a scale.
func (m *Model) IsVectorReference(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	vector, scalar := m.libSymbol(fqnVectorMeasurementReference), m.libSymbol(fqnScalarMeasurementReference)
	return vector != nil && scalar != nil && m.Conforms(sym, vector) && !m.Conforms(sym, scalar)
}

// FixedDimensions is the `dimensions` sym fixes by a constant on Array's feature or
// a redefinition of it (`'3dCoordinateFrame'`: `:>> dimensions = 3`); else false.
func (m *Model) FixedDimensions(sym *symbols.Symbol) ([]int64, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	array := m.libSymbol(fqnArray)
	if array == nil {
		return nil, false
	}
	dimensions, ok := m.LookupMember(array, memberDimensions)
	if !ok || dimensions == nil {
		return nil, false
	}
	for _, member := range m.MembersOfIncludingRedefined(sym) {
		if !IsShapeFeature(member) || !m.restates(member, dimensions) {
			continue
		}
		if fixed, ok := m.constantIntegers(member); ok {
			return fixed, true
		}
	}
	return nil, false
}

// restates reports whether member is feature or redefines it, transitively.
func (m *Model) restates(member, feature *symbols.Symbol) bool {
	return member == feature || slices.Contains(m.AllRedefinedFeatures(member), feature)
}

// constantIntegers folds the value a feature states — its own, else the nearest
// redefined one's — to whole numbers; false when only evaluation settles it.
func (m *Model) constantIntegers(member *symbols.Symbol) ([]int64, bool) {
	value := usageValue(member)
	for _, redefined := range m.AllRedefinedFeatures(member) {
		if value != nil {
			break
		}
		value = usageValue(redefined)
	}
	if value == nil {
		return nil, false
	}
	var elements []ast.Node
	switch n := value.(type) {
	case *ast.NullExpr:
	case *ast.SequenceExpr:
		elements = n.Elements
	default:
		elements = []ast.Node{value}
	}
	out := make([]int64, 0, len(elements))
	for _, e := range elements {
		c, ok := m.Eval(e)
		if !ok {
			return nil, false
		}
		n, ok := c.WholeNumber()
		if !ok {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// axisDimension is the dimension a frame type's `mRefs` admits on every axis
// (CartesianVelocity3dCoordinateFrame: SpeedUnit[3], L/T); false when unfixed.
func (m *Model) axisDimension(frame *symbols.Symbol) (UnitTerm, bool) {
	mRefs, ok := m.LookupMember(frame, memberMRefs)
	if !ok || mRefs == nil {
		return UnitTerm{}, false
	}
	for _, typ := range m.declaredTypes(mRefs, map[*symbols.Symbol]bool{}) {
		if !m.IsMeasurementUnit(typ) {
			continue
		}
		if dim, ok := m.dimensionOf(typ); ok {
			return dim, true
		}
	}
	return UnitTerm{}, false
}

// ComposedFrameConforms judges a composed frame — a CoordinateFrame — against a
// declared type: by type, else, for a frame type, by its fixed dimensions and axes.
func (m *Model) ComposedFrameConforms(f ComposedFrame, want *symbols.Symbol) Conformance {
	if m == nil || want == nil {
		return conformanceUnknown()
	}
	frame := m.libSymbol(fqnCoordinateFrame)
	if frame == nil {
		return conformanceUnknown()
	}
	c := m.typeConformance(frame, want)
	if !c.Known || c.Holds {
		return c
	}
	c.Found = "a coordinate frame composed from another's axes"
	if !m.Conforms(want, frame) {
		return c
	}
	if fixed, ok := m.FixedDimensions(want); ok && f.HasDimensions && !slices.Equal(fixed, f.Dimensions) {
		c.Found = fmt.Sprintf("a coordinate frame of dimensions %v, where %s fixes dimensions %v",
			f.Dimensions, leafName(want.Name), fixed)
		return c
	}
	if axis, ok := m.axisDimension(want); ok {
		for i, got := range f.AxisDimensions {
			if got.Commensurable(axis) {
				continue
			}
			which := fmt.Sprintf("axis %d measures", i+1)
			if f.Uniform {
				which = "axes measure"
			}
			c.Found = fmt.Sprintf("a coordinate frame whose %s in dimension %s, where %s admits %s",
				which, Dimension{Term: got}, leafName(want.Name), Dimension{Term: axis})
			return c
		}
	}
	c.Holds = true
	return c
}

// CoordinateFrameExprConformance judges `cf * u` and `cf / u`, cf a coordinate
// frame and u a measurement unit, as the frame composed; false for another expression.
func (m *Model) CoordinateFrameExprConformance(scope *symbols.Scope, e *ast.OperatorExpr, want *symbols.Symbol) (Conformance, bool) {
	if want == nil {
		return conformanceUnknown(), false
	}
	composed, ok := m.composedFrameExpr(scope, e)
	if !ok {
		return conformanceUnknown(), false
	}
	return m.ComposedFrameConforms(composed, want), true
}

// CoordinateFrameExprType is the type of `cf * u` or `cf / u`, cf a coordinate frame
// and u a measurement unit: CoordinateFrame, as the calcs declare; else nil.
func (m *Model) CoordinateFrameExprType(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.Symbol {
	if _, ok := m.composedFrameExpr(scope, e); !ok {
		return nil
	}
	return m.libSymbol(fqnCoordinateFrame)
}

// composedFrameExpr reduces `cf * u` or `cf / u` to the frame it composes: cf's
// dimensions, each axis times or over u; false when e is not such an expression.
func (m *Model) composedFrameExpr(scope *symbols.Scope, e *ast.OperatorExpr) (ComposedFrame, bool) {
	if m == nil || e == nil || len(e.Operands) != 2 || (e.Operator != ast.OpMul && e.Operator != ast.OpDiv) {
		return ComposedFrame{}, false
	}
	base, ok := m.frameOperand(scope, e.Operands[0])
	if !ok {
		return ComposedFrame{}, false
	}
	unit, ok := m.measurementRefOperand(scope, e.Operands[1])
	if !ok {
		return ComposedFrame{}, false
	}
	composed := ComposedFrame{Dimensions: base.Dimensions, HasDimensions: base.HasDimensions, Uniform: base.Uniform}
	unitDim, ok := m.dimensionOfUnitTerm(unit)
	if !ok || base.AxisDimensions == nil {
		return composed, true
	}
	composed.AxisDimensions = make([]UnitTerm, len(base.AxisDimensions))
	for i, axis := range base.AxisDimensions {
		if e.Operator == ast.OpMul {
			composed.AxisDimensions[i] = axis.Times(unitDim)
		} else {
			composed.AxisDimensions[i] = axis.DividedBy(unitDim)
		}
	}
	return composed, true
}

// frameOperand is the frame an operand names (a feature typed by a CoordinateFrame)
// or composes (`cf * u`), as far as declarations fix it; false for another operand.
func (m *Model) frameOperand(scope *symbols.Scope, node ast.Node) (ComposedFrame, bool) {
	switch n := node.(type) {
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return ComposedFrame{}, false
		}
		if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
			sym = alias
		}
		typ := m.featureResultType(sym)
		if !m.IsCoordinateFrame(typ) {
			return ComposedFrame{}, false
		}
		return m.declaredFrame(sym, typ), true
	case *ast.OperatorExpr:
		return m.composedFrameExpr(scope, n)
	}
	return ComposedFrame{}, false
}

// declaredFrame is the frame a feature of a frame type holds, as far as declarations
// fix it: its dimensions, and the one dimension of every axis when the type fixes it.
func (m *Model) declaredFrame(sym, typ *symbols.Symbol) ComposedFrame {
	f := ComposedFrame{}
	f.Dimensions, f.HasDimensions = m.FixedDimensions(sym)
	if !f.HasDimensions {
		f.Dimensions, f.HasDimensions = m.FixedDimensions(typ)
	}
	if axis, ok := m.axisDimension(typ); ok {
		f.AxisDimensions, f.Uniform = []UnitTerm{axis}, true
	}
	return f
}
