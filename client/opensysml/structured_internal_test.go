package opensysml

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

// A structured value is refused before it leaves the client when the service
// lacks structured_values, since such a service would read it as null: whether
// the service predates the capability or the GetServerInfo RPC itself.
func TestStructuredInputIsNotSentWithoutStructuredValues(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	vector := Vector{Real(3), Real(4)}
	for name, old := range map[string]*oldCaller{
		"predates structured_values": {t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues}},
		"predates GetServerInfo":     {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			for label, input := range map[string]Value{
				"vector":          vector,
				"array":           Array{Dimensions: []int64{1}, Elements: []Value{Int(1)}},
				"vector quantity": VectorQuantity{{Magnitude: Real(1), Unit: "m"}},
				"nested":          Sequence{Int(1), Sequence{vector}},
				"in an array":     Array{Dimensions: []int64{1}, Elements: []Value{vector}},
			} {
				_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
				wantUnimplemented(t, "ExecuteAction "+label, err)
				_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
				wantUnimplemented(t, "EvaluateCalc "+label, err)
			}
		})
	}
}

// An array carrying a Complex needs complex_values, as a sequence does.
func TestComplexInArrayNeedsComplexValues(t *testing.T) {
	old := &oldCaller{t: t, capabilities: []string{CapabilityStructuredValues}}
	c := &client{caller: old}
	input := Array{Dimensions: []int64{1}, Elements: []Value{Complex(complex(1, 2))}}
	_, err := c.ExecuteAction(context.Background(), &Model{Hash: "h"}, "A", map[string]Value{"z": input})
	wantUnimplemented(t, "ExecuteAction array of complex", err)
}

// A malformed structured value in an answer reads as an unsupported null
// naming the fault, never as an Array, Vector or VectorQuantity that
// contradicts its own contract.
func TestMalformedStructuredAnswersAreNullsNamingTheFault(t *testing.T) {
	one := &pb.Value{Kind: &pb.Value_IntValue{IntValue: 1}}
	metre := &pb.Quantity{Unit: "m", Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1}}
	noMagnitude := &pb.Quantity{Unit: "m"}
	array := func(dims []int64, elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Array{Array: &pb.Array{Dimensions: dims, Elements: elements}}}
	}
	vectorQuantity := func(components ...*pb.Quantity) *pb.Value {
		return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: &pb.VectorQuantity{Components: components}}}
	}
	text := &pb.Value{Kind: &pb.Value_StringValue{StringValue: "x"}}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"zero dimension":           {array([]int64{0}), "not positive"},
		"negative dimension":       {array([]int64{2, -3}, one), "not positive"},
		"too few elements":         {array([]int64{2, 3}, one, one, one, one, one), "do not fill"},
		"too many elements":        {array([]int64{2}, one, one, one), "do not fill"},
		"rank 0 without one":       {array(nil), "do not fill"},
		"rank 0 with two":          {array(nil, one, one), "do not fill"},
		"overflowing shape":        {array([]int64{1 << 40, 1 << 40}, one), "exceeds the Integer range"},
		"non-numeric vector":       {&pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{one, text}}}}, "non-numeric"},
		"empty vector quantity":    {vectorQuantity(), "without components"},
		"magnitude-less component": {vectorQuantity(metre, noMagnitude), "without a magnitude"},
		"magnitude-less quantity":  {&pb.Value{Kind: &pb.Value_Quantity{Quantity: noMagnitude}}, "without a magnitude"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	well := array([]int64{2, 1}, one, one)
	if got, ok := valueFromProto(well).(Array); !ok || len(got.Elements) != 2 {
		t.Fatalf("well-formed array read as %#v", valueFromProto(well))
	}
	nested := valueFromProto(array([]int64{1}, &pb.Value{Kind: &pb.Value_Quantity{Quantity: noMagnitude}}))
	if got, ok := nested.(Array); !ok || len(got.Elements) != 1 || got.Elements[0] != Null("unsupported: quantity without a magnitude") {
		t.Fatalf("array of a magnitude-less quantity read as %#v", nested)
	}
}

// A measurement reference is refused before it leaves the client when the
// service lacks measurement_refs, however deeply nested.
func TestMeasurementRefInputIsNotSentWithoutMeasurementRefs(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	metre := MeasurementRef{Unit: "m", Term: &UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []UnitFactor{{UnitID: "SI::metre", Exponent: 1}}}}
	old := &oldCaller{t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues, CapabilityStructuredValues}}
	c := &client{caller: old}
	for label, input := range map[string]Value{
		"reference":   metre,
		"nested":      Sequence{Int(1), Sequence{metre}},
		"in an array": Array{Dimensions: []int64{1}, Elements: []Value{metre}},
	} {
		_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
		wantUnimplemented(t, "ExecuteAction "+label, err)
		_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
		wantUnimplemented(t, "EvaluateCalc "+label, err)
	}
}

// A set or a tensor quantity is refused before it leaves the client when the
// service lacks set_values or tensor_values, however deeply nested.
func TestSetAndTensorInputsAreNotSentWithoutTheirCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	metre := Quantity{Magnitude: Real(1), Unit: "m"}
	old := &oldCaller{t: t, capabilities: []string{CapabilityFeatureValues, CapabilityComplexValues, CapabilityStructuredValues, CapabilityMeasurementRefs}}
	c := &client{caller: old}
	for label, input := range map[string]Value{
		"set":                Set{Int(1)},
		"set nested":         Sequence{Int(1), Sequence{Set{}}},
		"set in an array":    Array{Dimensions: []int64{1}, Elements: []Value{Set{Int(1)}}},
		"tensor":             TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}},
		"tensor nested":      Sequence{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}},
		"tensor in a set":    Set{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}},
		"tensor in an array": Array{Dimensions: []int64{1}, Elements: []Value{TensorQuantity{Dimensions: []int64{1}, Components: []Quantity{metre}}}},
	} {
		_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"x": input})
		wantUnimplemented(t, "ExecuteAction "+label, err)
		_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
		wantUnimplemented(t, "EvaluateCalc "+label, err)
	}
}

// A malformed tensor quantity in an answer reads as an unsupported null naming
// the fault; a set reads as its elements, whatever they are.
func TestMalformedTensorAnswersAreNullsNamingTheFault(t *testing.T) {
	one := &pb.Quantity{Magnitude: &pb.Quantity_IntMagnitude{IntMagnitude: 1}, Unit: "m"}
	tensor := func(dimensions []int64, components ...*pb.Quantity) *pb.Value {
		return &pb.Value{Kind: &pb.Value_TensorQuantity{TensorQuantity: &pb.TensorQuantity{Dimensions: dimensions, Components: components}}}
	}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"too few components":  {tensor([]int64{2, 2}, one, one, one), "do not fill"},
		"too many components": {tensor([]int64{2}, one, one, one), "do not fill"},
		"zero dimension":      {tensor([]int64{0}), "not positive"},
		"negative dimension":  {tensor([]int64{-1}, one), "not positive"},
		"no magnitude":        {tensor([]int64{1}, &pb.Quantity{Unit: "m"}), "without a magnitude"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	set := &pb.Value{Kind: &pb.Value_Set{Set: &pb.ValueSet{Elements: []*pb.Value{
		{Kind: &pb.Value_IntValue{IntValue: 1}},
		{Kind: &pb.Value_Set{Set: &pb.ValueSet{}}},
	}}}}
	if got, want := valueFromProto(set), (Set{Int(1), Set{}}); !reflect.DeepEqual(got, want) {
		t.Errorf("set read as %#v, want %#v", got, want)
	}
	sent, err := valueToProto(Set{Int(1), Set{}})
	if err != nil || !proto.Equal(sent, set) {
		t.Errorf("set sent as %v (%v), want %v", sent, err, set)
	}
}

// A set in an answer listing a member twice — by value as the model judges it,
// so an Integer and the whole Real of its value are one member, nested
// collections and quantities included — reads as an unsupported null naming
// the member; one whose members only look alike reads as its elements.
func TestRepeatedSetMembersAreNullsNamingTheFault(t *testing.T) {
	pbInt := func(n int64) *pb.Value { return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}} }
	pbReal := func(x float64) *pb.Value { return &pb.Value{Kind: &pb.Value_RealValue{RealValue: x}} }
	pbComplex := func(re, im float64) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Complex{Complex: &pb.Complex{Real: re, Imaginary: im}}}
	}
	pbSeq := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	pbSet := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Set{Set: &pb.ValueSet{Elements: elements}}}
	}
	pbQty := func(n int64, unit string) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: &pb.Quantity{Magnitude: &pb.Quantity_IntMagnitude{IntMagnitude: n}, Unit: unit}}}
	}
	pbRealQty := func(x float64, unit string) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: &pb.Quantity{Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: x}, Unit: unit}}}
	}
	for name, value := range map[string]*pb.Value{
		"integer twice":        pbSet(pbInt(1), pbInt(2), pbInt(1)),
		"integer and real":     pbSet(pbInt(1), pbReal(1)),
		"real and complex":     pbSet(pbReal(1.5), pbComplex(1.5, 0)),
		"integer and complex":  pbSet(pbInt(2), pbComplex(2, 0)),
		"sequence twice":       pbSet(pbSeq(pbInt(1), pbInt(2)), pbSeq(pbInt(1), pbInt(2))),
		"set twice, reordered": pbSet(pbSet(pbInt(1), pbInt(2)), pbSet(pbInt(2), pbInt(1))),
		"quantity twice":       pbSet(pbQty(1, "m"), pbQty(1, "m")),
		"quantity by real":     pbSet(pbQty(1, "m"), pbRealQty(1, "m")),
		"empty set twice":      pbSet(pbSet(), pbSet()),
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: set lists a member twice: ") {
				t.Fatalf("read as %#v, want an unsupported Null naming the repeated member", got)
			}
		})
	}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  Set
	}{
		"integer and near real": {pbSet(pbInt(1<<53+1), pbReal(1<<53)), Set{Int(1<<53 + 1), Real(1 << 53)}},
		"integer and fraction":  {pbSet(pbInt(1), pbReal(1.5)), Set{Int(1), Real(1.5)}},
		"real and imaginary":    {pbSet(pbReal(1), pbComplex(1, 1)), Set{Real(1), Complex(complex(1, 1))}},
		"integer and boolean":   {pbSet(pbInt(1), &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: true}}), Set{Int(1), Bool(true)}},
		"sequence and set":      {pbSet(pbSeq(pbInt(1)), pbSet(pbInt(1))), Set{Sequence{Int(1)}, Set{Int(1)}}},
		"sequences reordered":   {pbSet(pbSeq(pbInt(1), pbInt(2)), pbSeq(pbInt(2), pbInt(1))), Set{Sequence{Int(1), Int(2)}, Sequence{Int(2), Int(1)}}},
		"quantities in a unit":  {pbSet(pbQty(1, "m"), pbQty(1, "km")), Set{Quantity{Magnitude: Int(1), Unit: "m"}, Quantity{Magnitude: Int(1), Unit: "km"}}},
		"empty and singleton":   {pbSet(pbSet(), pbSet(pbSet())), Set{Set{}, Set{Set{}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := valueFromProto(tc.value); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("read as %#v, want %#v", got, tc.want)
			}
		})
	}
	if !Equal(Set{Int(1), Sequence{Int(2), Int(3)}}, Set{Sequence{Int(2), Int(3)}, Int(1)}) {
		t.Error("sets holding the same members in another order are not Equal")
	}
	if Equal(Set{Int(1)}, Sequence{Int(1)}) || Equal(Int(1), Bool(true)) || Equal(nil, Null("")) {
		t.Error("values of different kinds are Equal")
	}
	if !Equal(Null("a"), Null("b")) || !Equal(Unset{}, Unset{}) || Equal(Unset{}, Null("")) {
		t.Error("a Null is not one whatever its reason, or an Unset is not one")
	}
}

// Equal judges numbers as the service does: by value across Int, Real and a
// Complex on the real axis, exactly — an Int beyond a double's precision is
// not the Real it would round to — and inside quantities, vectors and sets.
func TestEqualJudgesNumbersByValue(t *testing.T) {
	for name, tc := range map[string]struct {
		a, b Value
		want bool
	}{
		"int and whole real":                 {Int(1), Real(1), true},
		"int and fraction":                   {Int(1), Real(1.5), false},
		"int and real beyond 2^53":           {Int(1<<53 + 1), Real(1 << 53), false},
		"int and real at 2^53":               {Int(1 << 53), Real(1 << 53), true},
		"max int and 2^63":                   {Int(math.MaxInt64), Real(-math.MinInt64), false},
		"min int and -2^63":                  {Int(math.MinInt64), Real(math.MinInt64), true},
		"int and infinity":                   {Int(0), Real(math.Inf(1)), false},
		"zero and negative zero":             {Int(0), Real(math.Copysign(0, -1)), true},
		"real and real-axis complex":         {Real(2.5), Complex(complex(2.5, 0)), true},
		"int and real-axis complex":          {Int(2), Complex(complex(2, 0)), true},
		"int and imaginary":                  {Int(2), Complex(complex(2, 1)), false},
		"complex twice":                      {Complex(complex(2, 1)), Complex(complex(2, 1)), true},
		"int and bool":                       {Int(1), Bool(true), false},
		"int and string":                     {Int(1), String("1"), false},
		"quantity by int and real":           {Quantity{Magnitude: Int(1), Unit: "m"}, Quantity{Magnitude: Real(1), Unit: "m"}, true},
		"quantity in another unit":           {Quantity{Magnitude: Int(1), Unit: "m"}, Quantity{Magnitude: Int(1), Unit: "km"}, false},
		"vector by int and real":             {Vector{Int(1), Real(2)}, Vector{Real(1), Int(2)}, true},
		"vector and sequence":                {Vector{Int(1)}, Sequence{Int(1)}, false},
		"set by int and real":                {Set{Int(1), Real(2.5)}, Set{Real(2.5), Real(1)}, true},
		"set and near real":                  {Set{Int(1<<53 + 1)}, Set{Real(1 << 53)}, false},
		"set listing one twice":              {Set{Int(1), Int(1)}, Set{Int(1), Int(2)}, false},
		"set listing one twice, sizes alike": {Set{Int(1), Int(1), Int(2)}, Set{Int(1), Int(2), Int(2)}, true},
		"set listing one twice, one":         {Set{Int(1), Real(1)}, Set{Int(1)}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Equal(tc.a, tc.b); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := Equal(tc.b, tc.a); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

// Equal judges quantities as the service does: commensurable ones by their
// magnitude over the base units, exactly while both are Ints over whole
// scales; ones carrying no reduction in their unit as written.
func TestEqualJudgesQuantitiesAcrossUnits(t *testing.T) {
	term := func(num, den float64, factors ...UnitFactor) *UnitTerm {
		return &UnitTerm{ScaleNum: num, ScaleDen: den, Factors: factors}
	}
	metre := UnitFactor{UnitID: "SI::metre", Exponent: 1}
	second := UnitFactor{UnitID: "SI::second", Exponent: 1}
	perSecond := UnitFactor{UnitID: "SI::second", Exponent: -1}
	m := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "m", Term: term(1, 1, metre)}
	}
	cm := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "cm", Term: term(1, 100, metre)}
	}
	cmDecimal := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "cm", Term: term(0.01, 1, metre)}
	}
	km := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "km", Term: term(1000, 1, metre)}
	}
	s := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "s", Term: term(1, 1, second)}
	}
	kmh := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "km/h", Term: term(1000, 3600, metre, perSecond)}
	}
	ms := func(magnitude Number) Quantity {
		return Quantity{Magnitude: magnitude, Unit: "m/s", Term: term(1, 1, perSecond, metre)}
	}
	named := func(magnitude Number, unit string) Quantity { return Quantity{Magnitude: magnitude, Unit: unit} }
	for name, tc := range map[string]struct {
		a, b Value
		want bool
	}{
		"metre and centimetres":                  {m(Int(1)), cm(Int(100)), true},
		"metre and centimetres, decimal":         {m(Int(1)), cmDecimal(Int(100)), true},
		"metre and centimetres, real":            {m(Real(1)), cm(Real(100)), true},
		"metre and centimetres, mixed":           {m(Int(1)), cm(Real(100)), true},
		"metre and one centimetre":               {m(Int(1)), cm(Int(1)), false},
		"metres and kilometre":                   {m(Int(1000)), km(Int(1)), true},
		"metres and kilometre, real":             {m(Real(1000)), km(Int(1)), true},
		"metres and kilometre, off by 1":         {m(Int(1001)), km(Int(1)), false},
		"metres and kilometres beyond 2^53":      {m(Int(1000 * (1<<53 + 1))), km(Int(1<<53 + 1)), true},
		"metres and kilometres beyond 2^53, off": {m(Int(1000*(1<<53+1) + 1)), km(Int(1<<53 + 1)), false},
		"metre and second":                       {m(Int(1)), s(Int(1)), false},
		"speeds":                                 {kmh(Real(5.4)), ms(Real(1.5)), true},
		"speeds, int":                            {kmh(Int(36)), ms(Int(10)), true},
		"speeds, unlike":                         {kmh(Int(36)), ms(Int(11)), false},
		"speed and length":                       {kmh(Int(1)), m(Int(1)), false},
		"unit named twice over":                  {m(Int(1)), Quantity{Magnitude: Int(1), Unit: "m", Term: term(1, 1, metre, perSecond, second)}, true},
		"named alike, no reduction":              {named(Int(1), "m"), named(Real(1), "m"), true},
		"named unlike, no reduction":             {named(Int(1), "m"), named(Int(100), "cm"), false},
		"reduction on one side":                  {named(Int(1), "m"), m(Int(1)), false},
		"zero scale":                             {Quantity{Magnitude: Int(0), Unit: "x", Term: term(0, 1, metre)}, m(Int(0)), false},
		"set of lengths in any unit":             {Set{m(Int(1)), km(Int(2))}, Set{m(Int(2000)), cm(Int(100))}, true},
		"set of lengths, one unlike":             {Set{m(Int(1)), km(Int(2))}, Set{m(Int(2000)), cm(Int(1))}, false},
		"vector quantities across units":         {VectorQuantity{m(Int(1)), km(Int(1))}, VectorQuantity{cm(Int(100)), m(Int(1000))}, true},
		"tensor quantities across units": {
			TensorQuantity{Dimensions: []int64{1, 1}, Components: []Quantity{m(Int(1))}},
			TensorQuantity{Dimensions: []int64{1, 1}, Components: []Quantity{cm(Int(100))}},
			true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Equal(tc.a, tc.b); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := Equal(tc.b, tc.a); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}

	// Membership and duplicate detection follow: a set holding 1 m holds 100
	// cm, and one listing both is refused before it is sent.
	lengths := Set{m(Int(1)), s(Int(1))}
	if !lengths.Contains(cm(Int(100))) || lengths.Contains(cm(Int(1))) {
		t.Errorf("Set{1 m, 1 s}.Contains: 100 cm %v, 1 cm %v", lengths.Contains(cm(Int(100))), lengths.Contains(cm(Int(1))))
	}
	var status *StatusError
	if _, err := valueToProto(Set{m(Int(1)), cm(Int(100))}); !errors.As(err, &status) || status.Code != CodeInvalidArgument {
		t.Errorf("valueToProto(Set{1 m, 100 cm}) = %v, want an invalid-argument StatusError", err)
	}
	if _, err := valueToProto(Set{m(Int(1)), cm(Int(1))}); err != nil {
		t.Errorf("valueToProto(Set{1 m, 1 cm}) = %v", err)
	}
}

// A measurement reference is one reduction at one scale however it is spelt;
// a named unit of dimension one is only its own declaration.
func TestEqualJudgesMeasurementRefsByReduction(t *testing.T) {
	metre := UnitFactor{UnitID: "SI::metre", Exponent: 1}
	perMetre := UnitFactor{UnitID: "SI::metre", Exponent: -1}
	perSecond := UnitFactor{UnitID: "SI::second", Exponent: -1}
	ref := func(unit, id string, num, den float64, factors ...UnitFactor) MeasurementRef {
		return MeasurementRef{Unit: unit, UnitID: id, Term: &UnitTerm{ScaleNum: num, ScaleDen: den, Factors: factors}}
	}
	namedSpeed := ref("SI::'m/s'", "SI::'m/s'", 1, 1, metre, perSecond)
	composedSpeed := ref("m / s", "", 1, 1, perSecond, metre)
	km := ref("km", "SI::kilometre", 1000, 1, metre)
	rad := ref("rad", "SI::radian", 1, 1)
	sr := ref("sr", "SI::steradian", 1, 1)
	ratio := ref("m / m", "", 1, 1, metre, perMetre)
	for name, tc := range map[string]struct {
		a, b Value
		want bool
	}{
		"named and composed, factors reordered": {namedSpeed, composedSpeed, true},
		"aliases":                               {km, ref("km", "SI::km", 2000, 2, metre), true},
		"scale as ratio or decimal":             {ref("km/m", "", 1000, 1), ref("m/mm", "", 1, 0.001), true},
		"unlike scale":                          {km, ref("m", "SI::metre", 1, 1, metre), false},
		"unlike dimension":                      {ref("m", "SI::metre", 1, 1, metre), ref("s", "SI::second", 1, 1, UnitFactor{UnitID: "SI::second", Exponent: 1}), false},
		"zero scale":                            {ref("x", "", 0, 1, metre), ref("x", "", 0, 1, metre), false},
		"named dimension one":                   {rad, sr, false},
		"named dimension one, spelt twice":      {rad, ref("SI::rad", "SI::radian", 1, 1, metre, perMetre), true},
		"named and composed dimension one":      {rad, ratio, false},
		"composed dimension one":                {ratio, ref("", "", 1, 1), true},
		"no reduction, alike":                   {MeasurementRef{Unit: "m", UnitID: "SI::metre"}, MeasurementRef{Unit: "m", UnitID: "SI::metre"}, true},
		"no reduction on one side":              {MeasurementRef{Unit: "m", UnitID: "SI::metre"}, ref("m", "SI::metre", 1, 1, metre), false},
		"set of references":                     {Set{namedSpeed, rad}, Set{rad, composedSpeed}, true},
		"set of references, one unlike":         {Set{namedSpeed, rad}, Set{sr, composedSpeed}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Equal(tc.a, tc.b); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := Equal(tc.b, tc.a); got != tc.want {
				t.Errorf("Equal(%#v, %#v) = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
	if !(Set{namedSpeed}).Contains(composedSpeed) || (Set{rad}).Contains(sr) {
		t.Error("membership does not follow Equal")
	}
	var status *StatusError
	for name, set := range map[string]Set{"named and composed": {namedSpeed, composedSpeed}, "aliases": {km, ref("km", "SI::km", 2000, 2, metre)}} {
		if _, err := valueToProto(set); !errors.As(err, &status) || status.Code != CodeInvalidArgument {
			t.Errorf("valueToProto(%s) = %v, want an invalid-argument StatusError", name, err)
		}
	}
	if _, err := valueToProto(Set{rad, sr}); err != nil {
		t.Errorf("valueToProto(Set{rad, sr}) = %v", err)
	}
}

// An enumeration literal is its LiteralID; its name and enumeration describe it.
func TestEqualJudgesEnumLiteralsByID(t *testing.T) {
	red := EnumLiteral{LiteralID: "D::Color::red", EnumerationID: "D::Color", Name: "Color::red"}
	same := EnumLiteral{LiteralID: "D::Color::red", EnumerationID: "E::Palette", Name: "red"}
	green := EnumLiteral{LiteralID: "D::Color::green", EnumerationID: "D::Color", Name: "Color::red"}
	if !Equal(red, same) || Equal(red, green) || !(Set{red}).Contains(EnumLiteral{LiteralID: "D::Color::red"}) {
		t.Errorf("Equal(red, same) = %v, Equal(red, green) = %v", Equal(red, same), Equal(red, green))
	}
	if !Equal(Set{red, green}, Set{EnumLiteral{LiteralID: "D::Color::green"}, same}) {
		t.Error("sets of literals compare by LiteralID")
	}
	var status *StatusError
	if _, err := valueToProto(Set{red, same}); !errors.As(err, &status) || status.Code != CodeInvalidArgument {
		t.Errorf("valueToProto(Set{red, same}) = %v, want an invalid-argument StatusError", err)
	}
	if _, err := valueToProto(Set{red, green}); err != nil {
		t.Errorf("valueToProto(Set{red, green}) = %v", err)
	}
}

// A set a caller assembles with a member listed twice, by Equal, is refused
// before it is sent, as the service would refuse it; one whose members only
// look alike is sent.
func TestRepeatedSetMembersAreNotSent(t *testing.T) {
	for name, set := range map[string]Set{
		"integer twice":        {Int(1), Int(2), Int(1)},
		"integer and real":     {Int(1), Real(1)},
		"real and complex":     {Real(1.5), Complex(complex(1.5, 0))},
		"sequence twice":       {Sequence{Int(1), Int(2)}, Sequence{Int(1), Int(2)}},
		"set twice, reordered": {Set{Int(1), Int(2)}, Set{Int(2), Int(1)}},
		"nested":               {Int(3), Set{Int(1), Int(1)}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := valueToProto(set)
			var status *StatusError
			if !errors.As(err, &status) || status.Code != CodeInvalidArgument || !strings.HasPrefix(status.Message, "set lists a member twice: ") {
				t.Fatalf("sent with err %v, want an invalid-argument StatusError naming the repeated member", err)
			}
		})
	}
	for name, set := range map[string]Set{
		"integer and near real": {Int(1<<53 + 1), Real(1 << 53)},
		"integer and boolean":   {Int(1), Bool(true)},
		"sequence and set":      {Sequence{Int(1)}, Set{Int(1)}},
		"sequences reordered":   {Sequence{Int(1), Int(2)}, Sequence{Int(2), Int(1)}},
		"empty and singleton":   {Set{}, Set{Set{}}},
	} {
		t.Run(name, func(t *testing.T) {
			sent, err := valueToProto(set)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got := valueFromProto(sent); !reflect.DeepEqual(got, set) {
				t.Errorf("read back as %#v, want %#v", got, set)
			}
		})
	}
}

// A malformed measurement reference in an answer reads as an unsupported null
// naming the fault; a well-formed one reads as itself, reduction and identity
// intact.
func TestMalformedMeasurementRefAnswersAreNullsNamingTheFault(t *testing.T) {
	ref := func(pm *pb.MeasurementRef) *pb.Value {
		return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: pm}}
	}
	for name, tc := range map[string]struct {
		value *pb.Value
		want  string
	}{
		"empty":                {ref(&pb.MeasurementRef{}), "naming no unit"},
		"unreduced by text":    {ref(&pb.MeasurementRef{Unit: "m"}), "without its reduction"},
		"unreduced by unit_id": {ref(&pb.MeasurementRef{UnitId: "SI::metre"}), "without its reduction"},
	} {
		t.Run(name, func(t *testing.T) {
			got := valueFromProto(tc.value)
			null, ok := got.(Null)
			if !ok || !strings.HasPrefix(string(null), "unsupported: ") || !strings.Contains(string(null), tc.want) {
				t.Fatalf("read as %#v, want an unsupported Null containing %q", got, tc.want)
			}
		})
	}
	term := &pb.UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::metre", Exponent: 1}}}
	got := valueFromProto(ref(&pb.MeasurementRef{Unit: "km", UnitTerm: term, UnitId: "SI::kilometre"}))
	want := MeasurementRef{Unit: "km", Term: &UnitTerm{ScaleNum: 1000, ScaleDen: 1, Factors: []UnitFactor{{UnitID: "SI::metre", Exponent: 1}}}, UnitID: "SI::kilometre"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("well-formed reference read as %#v, want %#v", got, want)
	}
	sent, err := valueToProto(want)
	if err != nil {
		t.Fatalf("valueToProto: %v", err)
	}
	if !proto.Equal(sent, ref(&pb.MeasurementRef{Unit: "km", UnitTerm: term, UnitId: "SI::kilometre"})) {
		t.Fatalf("marshalled as %v", sent)
	}
}

func TestAValueArmThisClientPredatesIsANullNamingIt(t *testing.T) {
	// A newer service's arm parses as an unknown field: a Value with no kind.
	got := valueFromProto(&pb.Value{})
	null, ok := got.(Null)
	if !ok || !strings.Contains(string(null), "does not know") {
		t.Fatalf("unknown arm read as %#v, want a Null naming it", got)
	}
	if plain := valueFromProto(&pb.Value{Kind: &pb.Value_Null{}}); plain != Null("") {
		t.Fatalf("plain null read as %#v", plain)
	}
}
