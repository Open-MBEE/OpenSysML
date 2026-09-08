package grpc

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SymbolToProtoIn converts a Symbol to protobuf SymbolInfo in an existing
// conversion context.
func SymbolToProtoIn(sym *symbols.Symbol, sc *SymbolContext) *pb.SymbolInfo {
	defer sc.Lock()()

	idx := sc.Index
	info := &pb.SymbolInfo{
		Id:       idx.GetFQN(sym), // Fully qualified name
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}

	// Extract metadata from AST node
	extractMetadata(sym, info.Metadata)

	// Add visibility to metadata
	info.Metadata["visibility"] = visibilityToString(sym.Visibility)

	// Collect child IDs
	if sym.Scope != nil {
		var childIDs []string
		for _, childSym := range sym.Scope.AllMembers() {
			childIDs = append(childIDs, idx.GetFQN(childSym))
		}
		info.ChildIds = childIDs
	}

	// Static type facts: the resolved type, the declared multiplicity and every
	// generalization edge. These are what a client needs to reconstruct the
	// element's type without re-deriving it from the metadata strings.
	info.TypeInfo = sc.typeInfoOf(sym)
	info.Multiplicity = sc.multiplicityOf(sym)
	info.Specializations = sc.specializationsOf(sym)

	// The attributes the element has, own and inherited, with their resolved
	// types and constant default values.
	info.Attributes, info.WithheldLibraryAttributes = sc.attributesOf(sym)

	return info
}

// int32Clamp narrows a line or column number to the proto's int32, saturating
// rather than wrapping: a position past 2^31 would otherwise be reported as a
// negative one.
func int32Clamp(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}

// DiagnosticToProto converts a passes.Diagnostic to protobuf.
func DiagnosticToProto(diag passes.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: diag.Severity.String(),
		Message:  diag.Message,
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}

// RunNoteDiagnosticsToProto converts what a run noted about itself — its choice
// points and the guards it could not evaluate — to informational diagnostics,
// located in their model document; one outside the model carries no span.
func RunNoteDiagnosticsToProto(notes []runtime.RunNote, model *CachedModel) []*pb.Diagnostic {
	if len(notes) == 0 {
		return nil
	}
	pbDiags := make([]*pb.Diagnostic, 0, len(notes))
	for _, n := range notes {
		diag := n.Diagnostic()
		file, _ := n.Location()
		if sf := model.document(file); sf != nil {
			pbDiags = append(pbDiags, DiagnosticToProto(diag, sf))
			continue
		}
		pbDiags = append(pbDiags, &pb.Diagnostic{Severity: diag.Severity.String(), Message: diag.Message})
	}
	return pbDiags
}

// ParserDiagnosticToProto converts a parser.Diagnostic to protobuf.
func ParserDiagnosticToProto(diag parser.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: "error", // Parser diagnostics are always errors
		Message:  diag.Message,
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}

// extractMetadata populates the metadata map from the symbol's AST node.
// Extracts: multiplicity, type, direction, abstract.
func extractMetadata(sym *symbols.Symbol, meta map[string]string) {
	if sym.Decl == nil {
		return
	}

	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		// Multiplicity
		if decl.Multiplicity != nil {
			meta["multiplicity"] = formatMultiplicity(decl.Multiplicity)
		}
		// Type (first typing relationship)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelTyping {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["type"] = formatQualifiedName(qn)
					break
				}
			}
		}
		// Direction
		if decl.Direction != ast.DirNone {
			meta["direction"] = decl.Direction.String()
		}
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}

	case *ast.Definition:
		// Abstract
		if decl.IsAbstract {
			meta["abstract"] = "true"
		}
		// Type (first specializes relationship for definitions)
		for _, rel := range decl.Relationships {
			if rel.Kind == ast.RelSpecializes {
				if qn, ok := rel.Target.(*ast.QualifiedName); ok {
					meta["specializes"] = formatQualifiedName(qn)
					break
				}
			}
		}
	}
}

// formatMultiplicity renders Multiplicity as "lower..upper" or "value".
func formatMultiplicity(m *ast.Multiplicity) string {
	if !m.IsRange {
		return formatMultiplicityBound(m.Lower)
	}
	lower := formatMultiplicityBound(m.Lower)
	upper := formatMultiplicityBound(m.Upper)
	return lower + ".." + upper
}

// formatMultiplicityBound renders a multiplicity bound node as a string.
func formatMultiplicityBound(n ast.Node) string {
	if n == nil {
		return ""
	}
	switch v := n.(type) {
	case *ast.LiteralInteger:
		return v.Value
	case *ast.LiteralInfinity:
		return "*"
	default:
		return "?"
	}
}

// formatQualifiedName renders QualifiedName as "A::B::C".
func formatQualifiedName(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return joinParts(parts, "::")
}

// joinParts joins parts with separator.
func joinParts(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

// visibilityToString converts ast.Visibility to string.
func visibilityToString(v ast.Visibility) string {
	switch v {
	case ast.VisibilityPublic:
		return "public"
	case ast.VisibilityPrivate:
		return "private"
	case ast.VisibilityProtected:
		return "protected"
	case ast.VisibilityDefault:
		return "default"
	default:
		return "default"
	}
}

// ValueToProto converts runtime.Value to protobuf Value. An enumeration literal
// is named by its declaration, which idx supplies the qualified name of.
func ValueToProto(val runtime.Value, idx *symbols.Index) *pb.Value {
	return ValueToProtoIn(nil, val, idx)
}

// ValueToProtoIn converts a value read from a live context, which is what tells
// an object holding no value from one with features: the former crosses as the
// unset arm, as every other surface reads it. rt may be nil for a value no
// context materialized.
func ValueToProtoIn(rt *runtime.Context, val runtime.Value, idx *symbols.Index) *pb.Value {
	if rt != nil && rt.HoldsNoValue(val) {
		return &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
	}
	switch val.Kind {
	case runtime.ValConst:
		// Map semantics.Value to protobuf based on type
		switch val.Const.Kind {
		case semantics.ValInt:
			return &pb.Value{Kind: &pb.Value_IntValue{IntValue: val.Const.Int}}
		case semantics.ValReal:
			return &pb.Value{Kind: &pb.Value_RealValue{RealValue: val.Const.Real}}
		case semantics.ValBool:
			return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val.Const.Bool}}
		case semantics.ValInfinity:
			return &pb.Value{Kind: &pb.Value_StringValue{StringValue: "*"}}
		default:
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported const kind"}}
		}
	case runtime.ValString:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: val.Str()}}
	case runtime.ValNull:
		return &pb.Value{Kind: &pb.Value_Null{Null: ""}}
	case runtime.ValInstance:
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: val.Instance}}
	case runtime.ValSequence:
		// Recursively convert sequence elements
		var pbElements []*pb.Value
		if val.Sequence() != nil {
			for _, elem := range val.Sequence().Elements() {
				pbElements = append(pbElements, ValueToProtoIn(rt, elem, idx))
			}
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: pbElements}}}
	case runtime.ValVariant:
		// The wire Value has no variant form: the object a selected variant
		// materialized is reported by identity, a valueless selection as unsupported.
		if id, ok := val.Object(); ok {
			return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: id}}
		}
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: variant selection"}}
	case runtime.ValQuantity:
		pq := QuantityToProto(val.Quantity())
		if pq == nil {
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: quantity with a non-numeric magnitude"}}
		}
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: pq}}
	case runtime.ValEnumLiteral:
		lit := enumLiteralToProto(val, idx)
		if lit == nil {
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: unresolved enumeration literal"}}
		}
		return &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: lit}}
	case runtime.ValComplex:
		return &pb.Value{Kind: &pb.Value_Complex{Complex: ComplexToProto(val.Complex())}}
	case runtime.ValArray:
		return &pb.Value{Kind: &pb.Value_Array{Array: arrayToProto(rt, val.Array(), idx)}}
	case runtime.ValVector:
		return &pb.Value{Kind: &pb.Value_Vector{Vector: vectorToProto(val.Vector())}}
	case runtime.ValVectorQuantity:
		pvq := vectorQuantityToProto(val.VectorQuantity())
		if pvq == nil {
			return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: vector quantity with a non-numeric component"}}
		}
		return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: pvq}}
	case runtime.ValMeasurementRef:
		return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: MeasurementRefToProto(val.MeasurementRef())}}
	case runtime.ValTensorQuantity:
		// No wire arm carries dimensions above one with a unit per component.
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: " + val.Kind.String() + " " + runtime.FormatValue(val)}}
	case runtime.ValCoordinateFrame, runtime.ValCoordinateTransformation:
		// No wire arm carries a frame's axes or a transformation's placement.
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported: " + val.Kind.String() + " " + runtime.FormatValue(val)}}
	default:
		return &pb.Value{Kind: &pb.Value_Null{Null: "unsupported"}}
	}
}

// enumLiteralToProto names a literal by the declaration it is, which is its
// identity, and by the enumeration declaring it. Nil for an unresolved literal.
func enumLiteralToProto(val runtime.Value, idx *symbols.Index) *pb.EnumLiteral {
	if val.Literal() == nil {
		return nil
	}
	lit := &pb.EnumLiteral{
		LiteralId: idx.GetFQN(val.Literal()),
		Name:      val.LiteralText(),
	}
	if enum := semantics.EnumerationOwning(val.Literal()); enum != nil {
		lit.EnumerationId = idx.GetFQN(enum)
	}
	return lit
}

// ComplexToProto marshals a complex number as its rectangular parts.
func ComplexToProto(z complex128) *pb.Complex {
	return &pb.Complex{Real: real(z), Imaginary: imag(z)}
}

// ProtoToComplex is the complex number a Complex message carries; an empty
// message is zero, as every proto3 default is.
func ProtoToComplex(pc *pb.Complex) complex128 {
	return complex(pc.GetReal(), pc.GetImaginary())
}

// arrayToProto marshals an array as its dimensions and its row-major elements,
// each converted as any value is.
func arrayToProto(rt *runtime.Context, a *runtime.Array, idx *symbols.Index) *pb.Array {
	pa := &pb.Array{Dimensions: slices.Clone(a.Dimensions)}
	for _, elem := range a.Elements {
		pa.Elements = append(pa.Elements, ValueToProtoIn(rt, elem, idx))
	}
	return pa
}

// vectorToProto marshals a vector's components, Integer and Real kept apart.
func vectorToProto(v *runtime.Vector) *pb.Vector {
	pv := &pb.Vector{}
	for _, elem := range v.Elements {
		pv.Components = append(pv.Components, ValueToProto(runtime.Value{Kind: runtime.ValConst, Const: elem}, nil))
	}
	return pv
}

// vectorQuantityToProto marshals a vector quantity as one Quantity per axis;
// nil if a component's magnitude is not a number.
func vectorQuantityToProto(vq *runtime.VectorQuantity) *pb.VectorQuantity {
	pvq := &pb.VectorQuantity{}
	for i := range vq.Num {
		pq := QuantityToProto(&runtime.Quantity{Num: vq.Num[i], Unit: vq.Units[i]})
		if pq == nil {
			return nil
		}
		pvq.Components = append(pvq.Components, pq)
	}
	return pvq
}

// MeasurementRefToProto marshals a bare reference: the unit as written, what it
// reduces to, and the one declaration it names when it names one.
func MeasurementRefToProto(ref *runtime.MeasurementRef) *pb.MeasurementRef {
	pm := &pb.MeasurementRef{Unit: ref.Unit.Text, UnitTerm: unitTermToProto(ref.Unit.Term)}
	if decl := ref.Declaration(); decl != nil {
		pm.UnitId = symbols.FQNOf(decl)
	}
	return pm
}

// QuantityToProto marshals a quantity: the magnitude in the unit written, plus
// what that unit reduces to. Reports nil for a magnitude that is not a number.
func QuantityToProto(q *runtime.Quantity) *pb.Quantity {
	if q == nil {
		return nil
	}
	pq := &pb.Quantity{Unit: q.Unit.Text, UnitTerm: unitTermToProto(q.Unit.Term)}
	switch q.Num.Kind {
	case semantics.ValInt:
		pq.Magnitude = &pb.Quantity_IntMagnitude{IntMagnitude: q.Num.Int}
	case semantics.ValReal:
		pq.Magnitude = &pb.Quantity_RealMagnitude{RealMagnitude: q.Num.Real}
	default:
		return nil
	}
	return pq
}

// unitTermToProto marshals a unit's reduction to base units, naming each base
// unit by qualified name: a symbol pointer means nothing to another process.
func unitTermToProto(term semantics.UnitTerm) *pb.UnitTerm {
	pt := &pb.UnitTerm{ScaleNum: term.Scale.Num, ScaleDen: term.Scale.Den}
	for _, f := range term.Factors {
		pt.Factors = append(pt.Factors, &pb.UnitFactor{
			UnitId:   symbols.FQNOf(f.Unit),
			Exponent: f.Exponent,
		})
	}
	return pt
}

var (
	// ErrQuantityNeedsIndex reports a quantity converted from the wire without
	// the model's symbols, which its base units can only be resolved against.
	ErrQuantityNeedsIndex = errors.New("a quantity needs the model's symbols to be converted from the wire")

	// ErrUnknownBaseUnit reports a base unit the model does not declare, so no
	// unit can be rebuilt from the reduction naming it.
	ErrUnknownBaseUnit = errors.New("unknown base unit")

	// ErrNotAMeasurementUnit reports a reduction naming a symbol that is not a
	// measurement unit, which nothing is measured in.
	ErrNotAMeasurementUnit = errors.New("not a measurement unit")

	// ErrUnitScaleUnusable reports a unit reduction whose scale is zero, undefined
	// or not finite, which no magnitude can be converted through.
	ErrUnitScaleUnusable = errors.New("unit scale is not a usable ratio")

	// ErrUnitExponentUnusable reports a unit factor raised to a power that is not
	// a finite number, which measures no dimension.
	ErrUnitExponentUnusable = errors.New("unit exponent is not a finite number")

	// ErrUnitNotReduced reports a named unit sent without its reduction, over
	// which alone commensurability is decided.
	ErrUnitNotReduced = errors.New("unit carries no reduction to base units")

	// ErrUnitTextMismatch reports a unit whose text names units that do not
	// reduce to the unit_term sent with it, so the two describe different units.
	ErrUnitTextMismatch = errors.New("unit as written does not reduce to its unit_term")
	// ErrUnsetNotAccepted reports the unset arm arriving as an input. It reports
	// that a feature value holds no value, which is something to read, not to supply.
	ErrUnsetNotAccepted = errors.New("unset is not a value a caller can supply")

	// ErrArrayDimensionNotPositive reports an array sent with a dimension of no
	// extent, which Collections::Array declares as dimensions: Positive.
	ErrArrayDimensionNotPositive = errors.New("array dimension is not positive")

	// ErrArrayShapeMismatch reports an array whose elements do not fill its
	// dimensions, so no row-major reading of them has that shape.
	ErrArrayShapeMismatch = errors.New("array elements do not fill its dimensions")

	// ErrVectorComponentNotNumeric reports a vector component sent as something
	// other than an Integer or a Real, which a numerical vector has none of.
	ErrVectorComponentNotNumeric = errors.New("vector component is not a number")

	// ErrVectorQuantityEmpty reports a vector quantity of no components, whose
	// num is Number[1..*].
	ErrVectorQuantityEmpty = errors.New("vector quantity has no components")

	// ErrMeasurementRefNeedsIndex reports a measurement reference converted from
	// the wire without the model's symbols, which its units resolve against.
	ErrMeasurementRefNeedsIndex = errors.New("a measurement reference needs the model's symbols to be converted from the wire")

	// ErrMeasurementRefEmpty reports a measurement reference sent with no unit
	// text, no reduction and no declaration: nothing it could refer to.
	ErrMeasurementRefEmpty = errors.New("measurement reference names no unit")

	// ErrUnknownMeasurementUnit reports a unit_id the model does not declare
	// uniquely, so no reference can name the declaration.
	ErrUnknownMeasurementUnit = errors.New("unknown measurement unit")

	// ErrUnitIDMismatch reports a unit_id whose declaration the unit text sent
	// with it does not spell, so the two name different units.
	ErrUnitIDMismatch = errors.New("unit as written does not spell unit_id")
)

// ValueCarriesMeasurementRef reports whether a value, or any value nested in
// it, is a MeasurementRef: the kind measurement_refs governs.
func ValueCarriesMeasurementRef(pv *pb.Value) bool {
	return valueCarries(pv, func(v *pb.Value) bool {
		_, ok := v.GetKind().(*pb.Value_MeasurementRef)
		return ok
	})
}

// ValueCarriesComplex reports whether a value, or any value nested in it, is a
// Complex: the kind the complex_values capability governs.
func ValueCarriesComplex(pv *pb.Value) bool {
	return valueCarries(pv, func(v *pb.Value) bool {
		_, ok := v.GetKind().(*pb.Value_Complex)
		return ok
	})
}

// ValueCarriesStructured reports whether a value, or any value nested in it, is
// an Array, a Vector or a VectorQuantity: the kinds structured_values governs.
func ValueCarriesStructured(pv *pb.Value) bool {
	return valueCarries(pv, func(v *pb.Value) bool {
		switch v.GetKind().(type) {
		case *pb.Value_Array, *pb.Value_Vector, *pb.Value_VectorQuantity:
			return true
		}
		return false
	})
}

// valueCarries reports whether is holds of pv or of any value nested in it.
func valueCarries(pv *pb.Value, is func(*pb.Value) bool) bool {
	if is(pv) {
		return true
	}
	for _, nested := range nestedValues(pv) {
		if valueCarries(nested, is) {
			return true
		}
	}
	return false
}

// nestedValues lists the Values a value holds directly: a sequence's elements,
// an array's elements and a vector's components.
func nestedValues(pv *pb.Value) []*pb.Value {
	switch k := pv.GetKind().(type) {
	case *pb.Value_Sequence:
		return k.Sequence.GetElements()
	case *pb.Value_Array:
		return k.Array.GetElements()
	case *pb.Value_Vector:
		return k.Vector.GetComponents()
	}
	return nil
}

// ProtoToValueIn converts a protobuf Value to a runtime.Value in the model idx
// and sem describe, resolving a quantity's base units against them. Inverse of
// ValueToProto.
func ProtoToValueIn(pv *pb.Value, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pv == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	switch k := pv.GetKind().(type) {
	case *pb.Value_Unset:
		return runtime.Value{}, ErrUnsetNotAccepted
	case *pb.Value_Quantity:
		return ProtoToQuantity(k.Quantity, idx, sem)
	case *pb.Value_EnumLiteral:
		return enumLiteralFromProto(k.EnumLiteral, idx)
	case *pb.Value_Sequence:
		seq := runtime.NewSequence()
		if k.Sequence != nil {
			for _, elem := range k.Sequence.Elements {
				val, err := ProtoToValueIn(elem, idx, sem)
				if err != nil {
					return runtime.Value{}, err
				}
				seq.Append(val)
			}
		}
		return runtime.NewSequenceValue(seq), nil
	case *pb.Value_Array:
		return protoToArray(k.Array, idx, sem)
	case *pb.Value_Vector:
		return protoToVector(k.Vector)
	case *pb.Value_VectorQuantity:
		return protoToVectorQuantity(k.VectorQuantity, idx, sem)
	case *pb.Value_MeasurementRef:
		return ProtoToMeasurementRef(k.MeasurementRef, idx, sem)
	default:
		return protoToScalar(pv), nil
	}
}

// protoToArray rebuilds an array, refusing a shape its elements do not fill
// rather than reading them under some other shape.
func protoToArray(pa *pb.Array, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	dimensions := slices.Clone(pa.GetDimensions())
	if err := CheckArrayShape(dimensions, len(pa.GetElements())); err != nil {
		return runtime.Value{}, err
	}
	elements := make([]runtime.Value, 0, len(pa.GetElements()))
	for _, elem := range pa.GetElements() {
		val, err := ProtoToValueIn(elem, idx, sem)
		if err != nil {
			return runtime.Value{}, err
		}
		elements = append(elements, val)
	}
	return runtime.NewArrayValue(dimensions, elements), nil
}

// CheckArrayShape reports whether count elements fill dimensions in row-major
// order: every dimension positive and their product (one for rank 0) count.
func CheckArrayShape(dimensions []int64, count int) error {
	size := int64(1)
	for i, d := range dimensions {
		if d < 1 {
			return fmt.Errorf("%w: dimension %d is %d", ErrArrayDimensionNotPositive, i+1, d)
		}
		if size > math.MaxInt64/d {
			return fmt.Errorf("%w: flattenedSize of dimensions %v exceeds the Integer range", ErrArrayShapeMismatch, dimensions)
		}
		size *= d
	}
	if size != int64(count) {
		return fmt.Errorf("%w: %d elements under dimensions %v (flattenedSize %d)",
			ErrArrayShapeMismatch, count, dimensions, size)
	}
	return nil
}

// protoToVector rebuilds a vector from components that are each an Integer or
// a Real, refusing any other arm rather than reading it as a number.
func protoToVector(pv *pb.Vector) (runtime.Value, error) {
	components := make([]semantics.Value, 0, len(pv.GetComponents()))
	for i, comp := range pv.GetComponents() {
		num, err := protoToNumber(comp)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("%w: component %d", err, i+1)
		}
		components = append(components, num)
	}
	return runtime.NewVectorValue(components), nil
}

// protoToNumber is the Integer or Real a value holds; any other arm is an error.
func protoToNumber(pv *pb.Value) (semantics.Value, error) {
	switch k := pv.GetKind().(type) {
	case *pb.Value_IntValue:
		return semantics.Value{Kind: semantics.ValInt, Int: k.IntValue}, nil
	case *pb.Value_RealValue:
		return semantics.Value{Kind: semantics.ValReal, Real: k.RealValue}, nil
	}
	return semantics.Value{}, ErrVectorComponentNotNumeric
}

// protoToVectorQuantity rebuilds a vector quantity axis by axis, each
// component read exactly as a scalar Quantity is.
func protoToVectorQuantity(pvq *pb.VectorQuantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if len(pvq.GetComponents()) == 0 {
		return runtime.Value{}, ErrVectorQuantityEmpty
	}
	num := make([]semantics.Value, 0, len(pvq.GetComponents()))
	units := make([]runtime.Unit, 0, len(pvq.GetComponents()))
	for i, comp := range pvq.GetComponents() {
		if comp == nil {
			return runtime.Value{}, fmt.Errorf("%w: component %d carries no quantity", ErrVectorComponentNotNumeric, i+1)
		}
		val, err := ProtoToQuantity(comp, idx, sem)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("component %d: %w", i+1, err)
		}
		q := val.Quantity()
		num = append(num, q.Num)
		units = append(units, q.Unit)
	}
	return runtime.NewVectorQuantityValue(num, units), nil
}

// ProtoToQuantity rebuilds a quantity from the wire: the magnitude as sent, in
// the unit as written — read as the product of the named units it composes, so
// an operation over it cancels and merges them — over the base units idx
// resolves its reduction to.
func ProtoToQuantity(pq *pb.Quantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pq == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	if (idx == nil || sem == nil) && len(pq.GetUnitTerm().GetFactors()) > 0 {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrQuantityNeedsIndex, pq.GetUnit())
	}
	if pq.GetUnitTerm() == nil && pq.GetUnit() != "" {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrUnitNotReduced, pq.GetUnit())
	}
	term, err := protoToUnitTerm(pq.GetUnitTerm(), idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}

	var num semantics.Value
	switch m := pq.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		num = semantics.Value{Kind: semantics.ValInt, Int: m.IntMagnitude}
	case *pb.Quantity_RealMagnitude:
		num = semantics.Value{Kind: semantics.ValReal, Real: m.RealMagnitude}
	default:
		return runtime.Value{}, fmt.Errorf("quantity in %q carries no magnitude", pq.GetUnit())
	}

	product, term, err := unitProductOfText(pq.GetUnit(), term, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	text := pq.GetUnit()
	if text == "" && !product.IsEmpty() {
		text = product.String()
	}
	unit := runtime.Unit{Text: text, Product: product, Term: term}
	return runtime.NewQuantityValue(&runtime.Quantity{Num: num, Unit: unit}), nil
}

// ProtoToMeasurementRef rebuilds a bare reference from the wire: by the
// declaration unit_id names, checked against the unit text and reduction sent
// with it, or else by the unit text read exactly as a Quantity's is.
func ProtoToMeasurementRef(pm *pb.MeasurementRef, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pm.GetUnit() == "" && pm.GetUnitTerm() == nil && pm.GetUnitId() == "" {
		return runtime.Value{}, ErrMeasurementRefEmpty
	}
	if (idx == nil || sem == nil) && (len(pm.GetUnitTerm().GetFactors()) > 0 || pm.GetUnitId() != "") {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrMeasurementRefNeedsIndex, pm.GetUnit())
	}
	if pm.GetUnitTerm() == nil {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrUnitNotReduced, cmp.Or(pm.GetUnit(), pm.GetUnitId()))
	}
	term, err := protoToUnitTerm(pm.GetUnitTerm(), idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	if pm.GetUnitId() != "" {
		return declaredMeasurementRef(pm.GetUnitId(), pm.GetUnit(), term, idx, sem)
	}
	product, term, err := unitProductOfText(pm.GetUnit(), term, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	text := pm.GetUnit()
	if text == "" && !product.IsEmpty() {
		text = product.String()
	}
	return runtime.NewMeasurementRefValue(runtime.Unit{Text: text, Product: product, Term: term}), nil
}

// declaredMeasurementRef is the reference to the unit declaration id names, refused
// unless it is a measurement unit reducing to term that text, if any, spells.
func declaredMeasurementRef(id, text string, term semantics.UnitTerm, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	matches := idx.LookupQualified(id)
	if len(matches) != 1 {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrUnknownMeasurementUnit, id)
	}
	unit, ok := sem.MeasurementUnitOf(matches[0])
	if !ok {
		return runtime.Value{}, fmt.Errorf("%w: %s", ErrNotAMeasurementUnit, id)
	}
	declared, err := sem.UnitTermOf(unit)
	if err != nil {
		return runtime.Value{}, fmt.Errorf("%w: %s: %w", ErrUnitNotReduced, id, err)
	}
	if !reducesTo(declared, term) {
		return runtime.Value{}, fmt.Errorf("%w: %s reduces to %s, unit_term is %s", ErrUnitTextMismatch, id, declared, term)
	}
	if text != "" && !textSpellsUnit(text, unit, idx, sem) {
		return runtime.Value{}, fmt.Errorf("%w: %s is not %s", ErrUnitIDMismatch, text, id)
	}
	return runtime.DeclaredMeasurementRef(unit, text, declared), nil
}

// textSpellsUnit reports whether unit text is one name for the declaration unit:
// its symbol or name (`km`, `kilometre`), or a qualified name resolving to it.
func textSpellsUnit(text string, unit *symbols.Symbol, idx *symbols.Index, sem *semantics.Model) bool {
	p := parser.New(source.New("<unit>", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return false
	}
	spellings := []string{lexer.NameText(unit.Name)}
	if unit.ShortName != "" {
		spellings = append(spellings, lexer.NameText(unit.ShortName))
	}
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if len(qn.Parts) == 1 {
			return unit, slices.Contains(spellings, lexer.NameText(qn.Parts[0].Text))
		}
		matches := idx.LookupQualified(semantics.QualifiedNameText(qn))
		if len(matches) != 1 {
			return nil, false
		}
		sym, ok := sem.MeasurementUnitOf(matches[0])
		return unit, ok && sym == unit
	})
	if err != nil || len(product.Powers) != 1 {
		return false
	}
	return product.Powers[0].Unit == unit && product.Powers[0].Exponent == 1
}

// unitProductOfText reads unit text as a product of the model's units that reduces
// to term; a short name is the one unit so named that fits. Text that does not read
// so keeps the factors it can name, the rest opaque (see partialUnitProduct); text
// that is no unit expression is one opaque unit. The reduction returned is the
// model's where the text is read in full, else term as sent.
func unitProductOfText(text string, term semantics.UnitTerm, idx *symbols.Index, sem *semantics.Model) (semantics.UnitProduct, semantics.UnitTerm, error) {
	if text == "" {
		return unnamedUnitProduct(term), term, nil
	}
	opaque := semantics.OpaqueUnitProduct(text, term)
	if idx == nil || sem == nil {
		return opaque, term, nil
	}
	p := parser.New(source.New("<unit>", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return opaque, term, nil
	}
	unitAt := func(fqn string) (*symbols.Symbol, bool) {
		matches := idx.LookupQualified(fqn)
		if len(matches) != 1 {
			return nil, false
		}
		return sem.MeasurementUnitOf(matches[0])
	}
	var short []*ast.QualifiedName
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if sym, ok := unitAt(semantics.QualifiedNameText(qn)); ok {
			return sym, true
		}
		if len(qn.Parts) == 1 && !slices.Contains(short, qn) {
			short = append(short, qn)
		}
		return nil, false
	})
	if err != nil {
		return opaque, term, nil
	}
	if len(short) == 0 {
		implied, ok := impliedTerm(product, sem)
		if !ok {
			return partialUnitProduct(expr, opaque, term, unitAt, idx, sem), term, nil
		}
		if !reducesTo(implied, term) {
			return semantics.UnitProduct{}, semantics.UnitTerm{}, fmt.Errorf("%w: %s reduces to %s, unit_term is %s",
				ErrUnitTextMismatch, text, implied, term)
		}
		return product, implied, nil
	}

	readings := shortUnitReadings(short, idx, sem)
	var matches []shortUnitReading
	for _, reading := range readings {
		product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
			if sym, ok := unitAt(semantics.QualifiedNameText(qn)); ok {
				return sym, true
			}
			sym, ok := reading.units[qn]
			return sym, ok
		})
		if err != nil {
			continue
		}
		implied, ok := impliedTerm(product, sem)
		if !ok || !reducesTo(implied, term) {
			continue
		}
		// Two readings of one product, `m*m` as A::m·B::m and as B::m·A::m, are one reading.
		if slices.ContainsFunc(matches, func(m shortUnitReading) bool { return m.product.Equal(product) }) {
			continue
		}
		reading.product, reading.term = product, implied
		matches = append(matches, reading)
	}
	if len(matches) > 1 {
		matches = slices.DeleteFunc(matches, func(r shortUnitReading) bool {
			return !r.besideBaseUnits(term)
		})
	}
	if len(matches) != 1 {
		return partialUnitProduct(expr, opaque, term, unitAt, idx, sem), term, nil
	}
	return matches[0].product, matches[0].term, nil
}

// partialUnitProduct reads a unit text name by name — a qualified name or a short name
// one unit bears is that unit, any other an opaque factor — so the units read still cancel.
// Text whose every name reads, yet contradicts term, is opaque as a whole.
func partialUnitProduct(
	expr ast.Node,
	opaque semantics.UnitProduct,
	term semantics.UnitTerm,
	unitAt func(string) (*symbols.Symbol, bool),
	idx *symbols.Index,
	sem *semantics.Model,
) semantics.UnitProduct {
	unreadNames := map[string]int{}
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if sym, ok := unitAt(semantics.QualifiedNameText(qn)); ok {
			return sym, true
		}
		if len(qn.Parts) == 1 {
			if units := unitsNamed(semantics.QualifiedNameText(qn), idx, sem); len(units) == 1 {
				return units[0], true
			}
		}
		unreadNames[semantics.QualifiedNameText(qn)]++
		return nil, false
	})
	if err != nil {
		return opaque
	}
	// One unread name written twice is two units the text cannot tell apart.
	for _, n := range unreadNames {
		if n > 1 {
			return opaque
		}
	}
	known := semantics.UnitTerm{Scale: semantics.UnitScale(1)}
	var unread []int
	for i, f := range product.Powers {
		if f.Unit == nil {
			unread = append(unread, i)
			continue
		}
		factor, err := sem.UnitTermOf(f.Unit)
		if err != nil {
			return opaque
		}
		known = known.Times(factor.Pow(f.Exponent))
	}
	if len(unread) == 0 {
		return opaque
	}
	// A lone opaque factor is what term leaves once the units read are taken out.
	if len(unread) == 1 {
		f := &product.Powers[unread[0]]
		reduces := term.DividedBy(known).Pow(1 / f.Exponent)
		f.DimensionOne, f.Reduces = reduces.Dimensionless(), &reduces
	}
	return product
}

// shortUnitReading is one assignment of a unit text's short names, occurrence by
// occurrence, to units: `m*m` may name two units both written m.
type shortUnitReading struct {
	units   map[*ast.QualifiedName]*symbols.Symbol
	product semantics.UnitProduct
	term    semantics.UnitTerm
}

// besideBaseUnits reports whether every unit read is declared beside a base unit of term.
func (r shortUnitReading) besideBaseUnits(term semantics.UnitTerm) bool {
	namespaces := baseUnitNamespaces(term)
	for _, sym := range r.units {
		if !slices.Contains(namespaces, namespaceOf(sym)) {
			return false
		}
	}
	return true
}

// maxShortUnitReadings bounds the readings tried for one unit text.
const maxShortUnitReadings = 1024

// shortUnitReadings enumerates, in a fixed order, every assignment of the short
// name occurrences to units so named; none if a name has no unit or there are too many.
func shortUnitReadings(names []*ast.QualifiedName, idx *symbols.Index, sem *semantics.Model) []shortUnitReading {
	candidates := make([][]*symbols.Symbol, len(names))
	total := 1
	for i, qn := range names {
		candidates[i] = unitsNamed(semantics.QualifiedNameText(qn), idx, sem)
		total *= len(candidates[i])
		if total == 0 || total > maxShortUnitReadings {
			return nil
		}
	}
	readings := make([]shortUnitReading, 0, total)
	for k := range total {
		units := make(map[*ast.QualifiedName]*symbols.Symbol, len(names))
		rem := k
		for i, qn := range names {
			units[qn] = candidates[i][rem%len(candidates[i])]
			rem /= len(candidates[i])
		}
		readings = append(readings, shortUnitReading{units: units})
	}
	return readings
}

// unitsNamed lists, once each in qualified-name order, the units under a short name.
func unitsNamed(name string, idx *symbols.Index, sem *semantics.Model) []*symbols.Symbol {
	var units []*symbols.Symbol
	for _, fqn := range idx.FQNsEndingIn(name, math.MaxInt) {
		for _, sym := range idx.LookupQualified(fqn) {
			if unit, ok := sem.MeasurementUnitOf(sym); ok && !slices.Contains(units, unit) {
				units = append(units, unit)
			}
		}
	}
	return units
}

// impliedTerm reduces a product of resolved units; false if one is unresolved or unreducible.
func impliedTerm(product semantics.UnitProduct, sem *semantics.Model) (semantics.UnitTerm, bool) {
	implied := semantics.UnitTerm{Scale: semantics.UnitScale(1)}
	for _, f := range product.Powers {
		if f.Unit == nil {
			return semantics.UnitTerm{}, false
		}
		factor, err := sem.UnitTermOf(f.Unit)
		if err != nil {
			return semantics.UnitTerm{}, false
		}
		implied = implied.Times(factor.Pow(f.Exponent))
	}
	return implied, true
}

// reducesTo reports whether two reductions are one unit: commensurable at one scale.
func reducesTo(implied, term semantics.UnitTerm) bool {
	return implied.Commensurable(term) && sameScale(implied.Scale, term.Scale)
}

// namespaceOf is the qualified name of the namespace declaring sym, or "" for a root.
func namespaceOf(sym *symbols.Symbol) string {
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return ""
	}
	return symbols.FQNOf(sym.OwnerScope.Owner())
}

// baseUnitNamespaces lists, in factor order and once each, the namespaces
// declaring the base units a reduction is over.
func baseUnitNamespaces(term semantics.UnitTerm) []string {
	var out []string
	for _, f := range term.Factors {
		if f.Unit == nil {
			continue
		}
		if ns := namespaceOf(f.Unit); ns != "" && !slices.Contains(out, ns) {
			out = append(out, ns)
		}
	}
	return out
}

// unnamedUnitProduct is the unit of a quantity sent under no text: its base units
// at scale one, else the reduction as one opaque unit that names no dimension-one unit.
func unnamedUnitProduct(term semantics.UnitTerm) semantics.UnitProduct {
	if !sameScale(term.Scale, semantics.UnitScale(1)) {
		product := semantics.OpaqueUnitProduct(term.String(), term)
		product.Powers[0].DimensionOne = false
		return product
	}
	product := semantics.UnitProduct{}
	for _, f := range term.Factors {
		name := lexer.QualifiedNameText(symbols.FQNOf(f.Unit))
		product = product.Times(semantics.NamedUnitProduct(f.Unit, name, false).Pow(f.Exponent))
	}
	return product
}

// scaleTolerance is the relative difference two orders of composing one scale's
// factors can round to: a fraction of an ulp per multiplication, over at most dozens.
const scaleTolerance = 64 * 0x1p-52

// sameScale reports whether two scale ratios agree to within the rounding of
// composing them; a ratio further off is another scale, not noise.
func sameScale(a, b semantics.Scale) bool {
	return math.Abs(semantics.ConvertMagnitude(1, a, b)-1) <= scaleTolerance
}

// protoToUnitTerm rebuilds a unit's reduction, normalized so a term sent in any
// factor order is commensurable with the same unit derived in the model. A name
// the model does not declare uniquely as a measurement unit is an error, not a
// factor over whatever symbol it happened to resolve to.
func protoToUnitTerm(pt *pb.UnitTerm, idx *symbols.Index, sem *semantics.Model) (semantics.UnitTerm, error) {
	if pt == nil {
		// A magnitude sent under no unit at all: dimension one.
		return semantics.UnitTerm{Scale: semantics.UnitScale(1)}, nil
	}
	scale := semantics.Scale{Num: pt.GetScaleNum(), Den: pt.GetScaleDen()}
	if scale.IsZero() || !finite(scale.Num) || !finite(scale.Den) {
		return semantics.UnitTerm{}, fmt.Errorf("%w: %g/%g", ErrUnitScaleUnusable, scale.Num, scale.Den)
	}
	term := semantics.UnitTerm{Scale: scale}
	for _, f := range pt.GetFactors() {
		// An empty name is a lookup of the document root, so it is rejected here
		// rather than resolved to a symbol that measures nothing.
		if f.GetUnitId() == "" {
			return semantics.UnitTerm{}, fmt.Errorf("%w: unit factor names no unit", ErrUnknownBaseUnit)
		}
		matches := idx.LookupQualified(f.GetUnitId())
		if len(matches) != 1 {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s", ErrUnknownBaseUnit, f.GetUnitId())
		}
		unit, ok := sem.MeasurementUnitOf(matches[0])
		if !ok {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s", ErrNotAMeasurementUnit, f.GetUnitId())
		}
		term.Factors = append(term.Factors, semantics.UnitFactor{
			Unit:     unit,
			Exponent: f.GetExponent(),
		})
	}
	// Checked after normalization: repeated factors sum their exponents, and
	// finite powers can overflow in that sum.
	term = term.Normalized()
	for _, f := range term.Factors {
		if !finite(f.Exponent) {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s**%g", ErrUnitExponentUnusable, symbols.FQNOf(f.Unit), f.Exponent)
		}
	}
	return term, nil
}

// finite reports whether a wire double is a number a unit term can carry.
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// enumLiteralFromProto resolves a literal against the model, since a literal is
// the declaration it names: one the model does not declare has no identity here.
func enumLiteralFromProto(lit *pb.EnumLiteral, idx *symbols.Index) (runtime.Value, error) {
	if lit == nil || lit.GetLiteralId() == "" {
		return runtime.Value{}, fmt.Errorf("enumeration literal: literal_id names no declaration")
	}
	if idx == nil {
		return runtime.Value{}, fmt.Errorf("enumeration literal %s: no model to resolve it against", lit.GetLiteralId())
	}
	for _, sym := range idx.LookupQualified(lit.GetLiteralId()) {
		if semantics.EnumerationOwning(sym) != nil {
			return runtime.NewEnumLiteral(sym), nil
		}
	}
	return runtime.Value{}, fmt.Errorf("%s is not an enumeration literal of this model", lit.GetLiteralId())
}

// protoToScalar converts the arms of Value that name no symbol and hold no
// nested value.
func protoToScalar(pv *pb.Value) runtime.Value {
	switch k := pv.GetKind().(type) {
	case *pb.Value_IntValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: k.IntValue}}
	case *pb.Value_RealValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: k.RealValue}}
	case *pb.Value_BoolValue:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: k.BoolValue}}
	case *pb.Value_StringValue:
		return runtime.NewStringValue(k.StringValue)
	case *pb.Value_InstanceId:
		return runtime.Value{Kind: runtime.ValInstance, Instance: k.InstanceId}
	case *pb.Value_Null:
		return runtime.Value{Kind: runtime.ValNull}
	case *pb.Value_Complex:
		return runtime.NewComplex(ProtoToComplex(k.Complex))
	default:
		return runtime.Value{Kind: runtime.ValNull}
	}
}

const (
	// maxGraphDepth bounds how deep Instantiate expands nested objects.
	maxGraphDepth = 8
	// maxGraphInstances caps how many instances one response serializes.
	maxGraphInstances = 1000
)

// GraphBounds is how far InstanceGraphToProtoWithin expands an object graph:
// nested objects to Depth, and Instances objects in all.
type GraphBounds struct {
	Depth     int
	Instances int
}

// DefaultGraphBounds are the bounds the service serializes an instance graph under.
func DefaultGraphBounds() GraphBounds {
	return GraphBounds{Depth: maxGraphDepth, Instances: maxGraphInstances}
}

// InstanceGraph is an object graph serialized by InstanceGraphToProtoWithin.
type InstanceGraph struct {
	Root *pb.Instance
	All  []*pb.Instance // the root first
	// Truncated reports that the instance bound cut the graph short.
	Truncated bool
	// Errors are the typed failures behind every FeatureValue.Error in All.
	Errors []error
}

// InstanceGraphToProto converts inst and every instance reachable from it. The
// root is returned first; runtime instances live only for the duration of a
// request, so the whole reachable graph is serialized while the context is alive.
//
// Expansion stops at a child whose type is already on the path, at maxGraphDepth
// and at maxGraphInstances: reading a composite feature value materializes the object it
// holds, so a self-referential part would otherwise instantiate forever. An
// unexpanded child stays a bare instance id.
func InstanceGraphToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) (*pb.Instance, []*pb.Instance) {
	g := InstanceGraphToProtoWithin(rt, inst, idx, DefaultGraphBounds())
	return g.Root, g.All
}

// InstanceGraphToProtoWithin is InstanceGraphToProto under the given bounds; the
// type cycle guard applies whatever the bounds.
func InstanceGraphToProtoWithin(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index, bounds GraphBounds) InstanceGraph {
	var g InstanceGraph
	seen := make(map[int64]bool)
	onPath := make(map[*symbols.Symbol]bool)

	var walk func(*runtime.Instance, int) *pb.Instance
	walk = func(cur *runtime.Instance, depth int) *pb.Instance {
		if seen[cur.ID] {
			return nil
		}
		if len(g.All) >= bounds.Instances {
			g.Truncated = true
			return nil
		}
		seen[cur.ID] = true
		onPath[cur.Type] = true
		defer delete(onPath, cur.Type)

		// InstanceToProto reads every feature value through GetFeatureValue, which is what
		// lazily materializes the children the ids below resolve to.
		pbInst := instanceToProto(rt, cur, idx, func(err error) { g.Errors = append(g.Errors, err) })
		g.All = append(g.All, pbInst)

		if depth >= bounds.Depth {
			return pbInst
		}

		// In name order, so the graph is serialized in the same order every run.
		for _, name := range slices.Sorted(maps.Keys(pbInst.FeatureValues)) {
			for _, id := range instanceRefs(pbInst.FeatureValues[name]) {
				child, ok := rt.Instance(id)
				if !ok || onPath[child.Type] {
					continue
				}
				walk(child, depth+1)
			}
		}
		return pbInst
	}

	g.Root = walk(inst, 0)
	return g
}

// instanceRefs collects the instance IDs a feature value references, scalar or not.
func instanceRefs(fv *pb.FeatureValue) []int64 {
	var ids []int64
	var collect func(*pb.Value)
	collect = func(v *pb.Value) {
		if k, ok := v.GetKind().(*pb.Value_InstanceId); ok {
			ids = append(ids, k.InstanceId)
		}
		for _, nested := range nestedValues(v) {
			collect(nested)
		}
	}
	if fv.Value != nil {
		collect(fv.Value)
	}
	for _, v := range fv.Values {
		collect(v)
	}
	return ids
}

// collectionElements returns what a multi-valued feature holds; a multi-valued
// feature's contents can be either a sequence or a set.
func collectionElements(val runtime.Value) []runtime.Value {
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence() != nil {
			return val.Sequence().Elements()
		}
	case runtime.ValSet:
		if val.Set() != nil {
			return val.Set().Elements()
		}
	}
	return nil
}

// InstanceToProto converts runtime.Instance to protobuf Instance. Feature values
// are read through Instance.GetFeatureValue, so a derived default is evaluated against
// the instance rather than reported as unmaterialized.
// Features are read in name order, because reading one materializes the object it
// holds, so map order would decide the ids those objects are given.
func InstanceToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) *pb.Instance {
	return instanceToProto(rt, inst, idx, func(error) { /* unmaterialized features are silent */ })
}

// instanceToProto is InstanceToProto, handing each feature value it could not
// read to failed as the runtime's typed error before reporting its text.
func instanceToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index, failed func(error)) *pb.Instance {
	pbValues := make(map[string]*pb.FeatureValue)

	for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
		fv, err := inst.GetFeatureValue(rt, name)
		if err != nil {
			failed(err)
			pbValues[name] = &pb.FeatureValue{
				FeatureName: name,
				Error:       err.Error(),
			}
			continue
		}

		pbValue := &pb.FeatureValue{
			FeatureName:  name,
			Materialized: fv.Materialized,
		}

		// Check multiplicity to determine single- vs multi-valued
		if fv.Feature.Scalar() {
			// Single-valued. An unmaterialized one holds no value; marshalling it
			// anyway would report the empty value as an unsupported null. A
			// materialized one holding nothing is unset, as every surface reads it.
			switch {
			case !fv.Materialized:
			case fv.Value.Kind == runtime.ValInvalid:
				pbValue.Value = &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
			default:
				pbValue.Value = ValueToProtoIn(rt, fv.Value, idx)
			}
		} else {
			for _, elem := range collectionElements(fv.Values) {
				pbValue.Values = append(pbValue.Values, ValueToProtoIn(rt, elem, idx))
			}
		}

		pbValues[name] = pbValue
	}

	return &pb.Instance{
		Id:            inst.ID,
		TypeSymbolId:  idx.GetFQN(inst.Type),
		FeatureValues: pbValues,
	}
}
