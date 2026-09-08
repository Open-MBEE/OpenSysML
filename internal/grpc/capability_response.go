package grpc

import (
	"fmt"
	"strings"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (s *Service) symbolToProto(sym *symbols.Symbol, sc *SymbolContext) *pb.SymbolInfo {
	info := SymbolToProtoIn(sym, sc)
	if !s.capabilities.has(CapabilityTypeFacts) {
		info.TypeInfo = nil
		info.Multiplicity = nil
		info.Specializations = nil
	}
	if !s.capabilities.has(CapabilitySymbolAttributes) {
		info.Attributes = nil
		info.WithheldLibraryAttributes = 0
		return info
	}
	for _, attribute := range info.Attributes {
		s.filterValueCapabilities(attribute.Value)
	}
	return info
}

func (s *Service) valueToProto(rt *runtime.Context, value runtime.Value, idx *symbols.Index) *pb.Value {
	out := ValueToProtoIn(rt, value, idx)
	s.filterValueCapabilities(out)
	return out
}

func (s *Service) instanceGraphToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) (*pb.Instance, []*pb.Instance) {
	root, all := InstanceGraphToProto(rt, inst, idx)
	for _, instance := range all {
		s.filterInstanceCapabilities(instance)
	}
	return root, all
}

func (s *Service) filterInstanceCapabilities(instance *pb.Instance) {
	if instance == nil {
		return
	}
	if !s.capabilities.has(CapabilityFeatureValues) {
		instance.FeatureValues = nil
		return
	}
	for _, feature := range instance.FeatureValues {
		s.filterValueCapabilities(feature.Value)
		for _, value := range feature.Values {
			s.filterValueCapabilities(value)
		}
	}
}

func (s *Service) filterDiagnosticCapabilities(diags []*pb.Diagnostic) []*pb.Diagnostic {
	if !s.capabilities.has(CapabilityDiagnosticCodes) {
		for _, diag := range diags {
			diag.Code = ""
		}
	}
	return diags
}

func (s *Service) filterValueCapabilities(value *pb.Value) {
	if value == nil {
		return
	}
	switch kind := value.GetKind().(type) {
	case *pb.Value_Sequence:
		for _, element := range kind.Sequence.GetElements() {
			s.filterValueCapabilities(element)
		}
	case *pb.Value_EnumLiteral:
		if !s.capabilities.has(CapabilityEnumValues) {
			value.Kind = &pb.Value_Null{Null: "unsupported: enumeration literal"}
		}
	case *pb.Value_Unset:
		if !s.capabilities.has(CapabilityUnsetValue) {
			value.Kind = &pb.Value_Null{Null: "unsupported: unset value"}
		}
	case *pb.Value_Complex:
		if !s.capabilities.has(CapabilityComplexValues) {
			value.Kind = &pb.Value_Null{Null: "unsupported: complex number " + runtime.FormatComplex(ProtoToComplex(kind.Complex))}
		}
	case *pb.Value_Array, *pb.Value_Vector, *pb.Value_VectorQuantity:
		if !s.capabilities.has(CapabilityStructuredValues) {
			shown := displayValue(value)
			value.Kind = &pb.Value_Null{Null: "unsupported: " + shown.Kind.String() + " " + runtime.FormatValue(shown)}
			return
		}
		for _, nested := range nestedValues(value) {
			s.filterValueCapabilities(nested)
		}
	case *pb.Value_MeasurementRef:
		if !s.capabilities.has(CapabilityMeasurementRefs) {
			shown := displayValue(value)
			value.Kind = &pb.Value_Null{Null: "unsupported: " + shown.Kind.String() + " " + runtime.FormatValue(shown)}
		}
	case *pb.Value_Function:
		if !s.capabilities.has(CapabilityFunctionValues) {
			value.Kind = &pb.Value_Null{Null: "unsupported: " + runtime.ValFunction.String() + " " + kind.Function.GetCalcId()}
		}
	case *pb.Value_Infinity:
		if !s.capabilities.has(CapabilityInfinityValue) {
			value.Kind = &pb.Value_Null{Null: "unsupported: unbounded value *"}
		}
	case *pb.Value_Set:
		if !s.capabilities.has(CapabilitySetValues) {
			shown := displayValue(value)
			value.Kind = &pb.Value_Null{Null: "unsupported: " + shown.Kind.String() + " " + runtime.FormatValue(shown)}
			return
		}
		for _, nested := range nestedValues(value) {
			s.filterValueCapabilities(nested)
		}
	case *pb.Value_TensorQuantity:
		if !s.capabilities.has(CapabilityTensorValues) {
			shown := displayValue(value)
			value.Kind = &pb.Value_Null{Null: "unsupported: " + shown.Kind.String() + " " + runtime.FormatValue(shown)}
		}
	}
}

// displayValue reads a wire value back for rendering only, needing no model:
// a quantity keeps the unit text it was sent under, a literal its name.
func displayValue(pv *pb.Value) runtime.Value {
	switch k := pv.GetKind().(type) {
	case *pb.Value_Sequence:
		seq := runtime.NewSequence()
		for _, elem := range k.Sequence.GetElements() {
			seq.Append(displayValue(elem))
		}
		return runtime.NewSequenceValue(seq)
	case *pb.Value_Quantity:
		return displayQuantity(k.Quantity)
	case *pb.Value_EnumLiteral:
		return runtime.NewStringValue(k.EnumLiteral.GetName())
	case *pb.Value_Array:
		elements := make([]runtime.Value, 0, len(k.Array.GetElements()))
		for _, elem := range k.Array.GetElements() {
			elements = append(elements, displayValue(elem))
		}
		return runtime.NewArrayValue(k.Array.GetDimensions(), elements)
	case *pb.Value_Vector:
		components := make([]semantics.Value, 0, len(k.Vector.GetComponents()))
		for _, comp := range k.Vector.GetComponents() {
			components = append(components, displayValue(comp).Const)
		}
		return runtime.NewVectorValue(components)
	case *pb.Value_VectorQuantity:
		num := make([]semantics.Value, 0, len(k.VectorQuantity.GetComponents()))
		units := make([]runtime.Unit, 0, len(k.VectorQuantity.GetComponents()))
		for _, comp := range k.VectorQuantity.GetComponents() {
			q := displayQuantity(comp).Quantity()
			num = append(num, q.Num)
			units = append(units, q.Unit)
		}
		return runtime.NewVectorQuantityValue(num, units)
	case *pb.Value_MeasurementRef:
		text := k.MeasurementRef.GetUnit()
		if text == "" {
			text = describeUnitTerm(k.MeasurementRef.GetUnitTerm())
		}
		return runtime.NewMeasurementRefValue(runtime.Unit{Text: text, Product: semantics.NamedUnitProduct(nil, text, false)})
	case *pb.Value_Set:
		set := runtime.NewSet()
		for _, elem := range k.Set.GetElements() {
			set.Add(displayValue(elem))
		}
		return runtime.NewSetValue(set)
	case *pb.Value_TensorQuantity:
		num := make([]semantics.Value, 0, len(k.TensorQuantity.GetComponents()))
		units := make([]runtime.Unit, 0, len(k.TensorQuantity.GetComponents()))
		for _, comp := range k.TensorQuantity.GetComponents() {
			q := displayQuantity(comp).Quantity()
			num = append(num, q.Num)
			units = append(units, q.Unit)
		}
		return runtime.NewTensorQuantityValue(k.TensorQuantity.GetDimensions(), num, units)
	default:
		return protoToScalar(pv)
	}
}

// displayQuantity is a quantity for rendering only, its unit the text sent or,
// under no text, its reduction spelled from the wire.
func displayQuantity(pq *pb.Quantity) runtime.Value {
	var num semantics.Value
	switch m := pq.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		num = semantics.Value{Kind: semantics.ValInt, Int: m.IntMagnitude}
	case *pb.Quantity_RealMagnitude:
		num = semantics.Value{Kind: semantics.ValReal, Real: m.RealMagnitude}
	}
	text := pq.GetUnit()
	if text == "" {
		text = describeUnitTerm(pq.GetUnitTerm())
	}
	return runtime.NewQuantityValue(&runtime.Quantity{Num: num, Unit: runtime.Unit{Text: text}})
}

// describeUnitTerm renders a unit's reduction as "1000/3600·SI::m·SI::s^-1",
// leaving a scale of one and an exponent of one implicit.
func describeUnitTerm(term *pb.UnitTerm) string {
	if term == nil {
		return "absent"
	}
	var parts []string
	if term.GetScaleNum() != term.GetScaleDen() {
		parts = append(parts, fmt.Sprintf("%g/%g", term.GetScaleNum(), term.GetScaleDen()))
	}
	for _, factor := range term.GetFactors() {
		if factor.GetExponent() == 1 {
			parts = append(parts, factor.GetUnitId())
			continue
		}
		parts = append(parts, fmt.Sprintf("%s^%g", factor.GetUnitId(), factor.GetExponent()))
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, "·")
}
