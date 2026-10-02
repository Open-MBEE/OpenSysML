"""A value explicitly unset in the model."""
struct Unset end
"""An infinite numeric value carried by the service."""
struct Infinity end
struct InstanceRef
    id::Int64
end
InstanceRef(id::Integer) = InstanceRef(Int64(id))
"""A primitive unit identifier and its exponent."""
struct UnitFactor
    unit_id::String
    exponent::Float64
end
UnitFactor(unit_id::AbstractString, exponent::Real=1.0) =
    UnitFactor(String(unit_id), Float64(exponent))
Base.:(==)(left::UnitFactor, right::UnitFactor) =
    left.unit_id == right.unit_id && left.exponent == right.exponent
"""A unit and its reduction to primitive unit factors."""
struct Unit
    text::String
    scale_num::Float64
    scale_den::Float64
    factors::Vector{UnitFactor}
    reduction_given::Bool
end
"""Construct a unit with its optional reduction."""
Unit(text::AbstractString=""; scale_num=1.0, scale_den=1.0, factors=UnitFactor[],
     reduction_given=false) =
    Unit(String(text), Float64(scale_num), Float64(scale_den), UnitFactor[f for f in factors],
         Bool(reduction_given))
Base.:(==)(left::Unit, right::Unit) =
    left.text == right.text && left.scale_num == right.scale_num &&
    left.scale_den == right.scale_den && left.factors == right.factors
function Unit(text::AbstractString, wire::AbstractDict)
    factors = UnitFactor[UnitFactor(String(get(f, "unitId", "")),
                                    asreal(get(f, "exponent", 1.0)))
                         for f in get(wire, "factors", Any[])]
    Unit(text; scale_num=asreal(get(wire, "scaleNum", 1.0)),
         scale_den=asreal(get(wire, "scaleDen", 1.0)), factors=factors,
         reduction_given=true)
end
function Base.getindex(unit::Unit, key::AbstractString)
    key == "scaleNum" && return unit.scale_num
    key == "scaleDen" && return unit.scale_den
    key == "factors" && return [Dict("unitId" => f.unit_id, "exponent" => f.exponent) for f in unit.factors]
    throw(KeyError(key))
end

"""Return the summed primitive exponents of a unit."""
function exponents(unit::Unit)
    totals = Dict{String,Float64}()
    for factor in unit.factors
        totals[factor.unit_id] = get(totals, factor.unit_id, 0.0) + factor.exponent
    end
    Dict(k => v for (k, v) in totals if v != 0)
end
"""Return whether two units reduce to the same primitive dimensions."""
commensurable(left::Unit, right::Unit) = exponents(left) == exponents(right)
"""Return the scale and primitive factors that reduce a unit."""
function reduction(unit::Unit)
    parts = String[]
    if (unit.scale_num, unit.scale_den) != (1.0, 1.0)
        push!(parts, unit.scale_den == 1.0 ? string(unit.scale_num) :
              "$(unit.scale_num)/$(unit.scale_den)")
    end
    for factor in unit.factors
        push!(parts, factor.exponent == 1.0 ? factor.unit_id :
              "$(factor.unit_id)^$(factor.exponent)")
    end
    isempty(parts) ? "1" : join(parts, "·")
end
reduced(unit::Unit) = isempty(unit.text) || unit.reduction_given || !isempty(unit.factors) ||
                      (unit.scale_num, unit.scale_den) != (1.0, 1.0)
zero_scale(unit::Unit) = unit.scale_num == 0 || unit.scale_den == 0
same_reduction(left::Unit, right::Unit) =
    commensurable(left, right) && !zero_scale(left) && !zero_scale(right) &&
    left.scale_num * right.scale_den == right.scale_num * left.scale_den
dimensionless(unit::Unit) = isempty(exponents(unit))
Base.show(io::IO, unit::Unit) = print(io, isempty(unit.text) ? reduction(unit) : unit.text)
Base.string(unit::Unit) = isempty(unit.text) ? reduction(unit) : unit.text

"""A numeric magnitude associated with a named unit and optional reduction."""
struct Quantity
    magnitude::Union{Int64,BigInt,Rational{BigInt},Float64}
    unit::String
    unit_term::Union{Nothing,Unit}
end
Quantity(magnitude::Bool, unit::AbstractString, term=nothing) =
    Quantity(Float64(magnitude), unit, term)
Quantity(magnitude::Integer, unit::AbstractString, term=nothing) =
    Quantity(Int64(magnitude), String(unit), term === nothing ? nothing :
             term isa Unit ? term : term isa AbstractDict ? Unit(unit, term) : throw(ArgumentError("invalid unit term")))
Quantity(magnitude::Rational, unit::AbstractString, term=nothing) =
    Quantity(_exact_rational(magnitude), String(unit), term === nothing ? nothing :
             term isa Unit ? term : term isa AbstractDict ? Unit(unit, term) : throw(ArgumentError("invalid unit term")))
Quantity(magnitude::AbstractFloat, unit::AbstractString, term=nothing) =
    Quantity(Float64(magnitude), String(unit), term === nothing ? nothing :
             term isa Unit ? term : term isa AbstractDict ? Unit(unit, term) : throw(ArgumentError("invalid unit term")))
struct EnumLiteral
    literal_id::String
    enumeration_id::String
    name::String
    value::Any
end
EnumLiteral(literal_id, enumeration_id, name) = EnumLiteral(literal_id, enumeration_id, name, nothing)
Base.:(==)(left::EnumLiteral, right::EnumLiteral) = left.literal_id == right.literal_id
struct FunctionRef
    calc_id::String
    self::Union{InstanceRef,Nothing}
end
Base.:(==)(left::FunctionRef, right::FunctionRef) =
    left.calc_id == right.calc_id && left.self == right.self
struct Metaobject
    element_id::String
    metaclass_id::String
end
Base.:(==)(left::Metaobject, right::Metaobject) = left.element_id == right.element_id
struct Undetermined
    reason::String
    lower::String
    upper::String
end

"""A unit value with its optional model declaration identity."""
struct MeasurementRef
    unit::String
    unit_id::Union{Nothing,String}
    unit_term::Union{Nothing,Unit}
end
MeasurementRef(unit::AbstractString, unit_id=nothing, term=nothing) =
    MeasurementRef(String(unit), unit_id === nothing ? nothing : String(unit_id),
                   term === nothing ? nothing : term isa Unit ? term :
                   term isa AbstractDict ? Unit(unit, term) : throw(ArgumentError("invalid unit term")))
function Base.:(==)(left::MeasurementRef, right::MeasurementRef)
    a = left.unit_term === nothing ? Unit(left.unit) : left.unit_term
    b = right.unit_term === nothing ? Unit(right.unit) : right.unit_term
    if !reduced(a) || !reduced(b)
        return a == b && left.unit_id == right.unit_id
    end
    same_reduction(a, b) || return false
    dimensionless(a) && (left.unit_id !== nothing || right.unit_id !== nothing) &&
        return left.unit_id == right.unit_id
    true
end
"""A multidimensional array with row-major elements."""
struct ArrayValue
    dimensions::Vector{Int64}
    elements::Vector{Any}
    ArrayValue(dimensions::Vector{Int64}, elements::Vector{Any}, ::Val{:validated}) =
        new(dimensions, elements)
end
function ArrayValue(dimensions, elements)
    dims = Int64[]
    for extent in dimensions
        (extent isa Bool || !(extent isa Integer) || extent <= 0) &&
            throw(ArgumentError("array dimension $(repr(extent)) is not a positive integer"))
        push!(dims, Int64(extent))
    end
    values = Any[element for element in elements]
    expected = prod(dims; init=1)
    expected == length(values) ||
        throw(ArgumentError("array of dimensions $(dims) holds $(length(values)) element(s), want $(expected)"))
    ArrayValue(dims, values, Val(:validated))
end
"""A numeric vector value."""
struct VectorValue
    components::Vector{Union{Int64,Float64}}
    VectorValue(components::Vector{Union{Int64,Float64}}, ::Val{:validated}) =
        new(components)
end
function VectorValue(components)
    values = Union{Int64,Float64}[]
    for component in components
        (component isa Bool || !(component isa Union{Integer,AbstractFloat})) &&
            throw(ArgumentError("vector component $(repr(component)) is not a number"))
        push!(values, component isa Integer ? Int64(component) : Float64(component))
    end
    VectorValue(values, Val(:validated))
end
"""A vector whose components carry quantities and units."""
struct VectorQuantity
    components::Vector{Quantity}
    VectorQuantity(components::Vector{Quantity}, ::Val{:validated}) = new(components)
end
function VectorQuantity(components)
    values = Quantity[]
    for component in components
        component isa Quantity ||
            throw(ArgumentError("vector quantity component $(repr(component)) is not a Quantity"))
        push!(values, component)
    end
    isempty(values) && throw(ArgumentError("vector quantity has no components"))
    VectorQuantity(values, Val(:validated))
end
"""A tensor whose components carry quantities and units."""
struct TensorQuantity
    dimensions::Vector{Int64}
    components::Vector{Quantity}
    TensorQuantity(dimensions::Vector{Int64}, components::Vector{Quantity},
                   ::Val{:validated}) = new(dimensions, components)
end
function TensorQuantity(dimensions, components)
    dims = Int64[]
    for extent in dimensions
        (extent isa Bool || !(extent isa Integer) || extent <= 0) &&
            throw(ArgumentError("tensor dimension $(repr(extent)) is not a positive integer"))
        push!(dims, Int64(extent))
    end
    values = Quantity[]
    for component in components
        component isa Quantity ||
            throw(ArgumentError("tensor component $(repr(component)) is not a Quantity"))
        push!(values, component)
    end
    expected = prod(dims; init=1)
    expected == length(values) ||
        throw(ArgumentError("tensor of dimensions $(dims) holds $(length(values)) component(s), want $(expected)"))
    TensorQuantity(dims, values, Val(:validated))
end

Base.length(value::ArrayValue) = length(value.elements)
Base.iterate(value::ArrayValue, state...) = iterate(value.elements, state...)
Base.getindex(value::ArrayValue, index::Int) = value.elements[index]
Base.length(value::TensorQuantity) = length(value.components)
Base.iterate(value::TensorQuantity, state...) = iterate(value.components, state...)
Base.getindex(value::TensorQuantity, index::Int) = value.components[index]
"""Return the number of dimensions in an array or tensor."""
Base.ndims(value::Union{ArrayValue,TensorQuantity}) = length(value.dimensions)
function Base.getindex(value::VectorValue, index::Int)
    value.components[index]
end
Base.length(value::Union{VectorValue,VectorQuantity}) = length(value.components)
Base.iterate(value::Union{VectorValue,VectorQuantity}, state...) = iterate(value.components, state...)
"""Return the shared unit of a vector quantity, or `nothing` when units differ."""
function unit(value::VectorQuantity)
    first_unit = _unit(first(value.components))
    all(component -> _unit(component) == first_unit, value.components) ? first_unit : nothing
end
"""Return vector-quantity magnitudes as a numeric vector."""
magnitudes(value::VectorQuantity) = VectorValue([component.magnitude for component in value.components])
function _unit(q::Quantity)
    q.unit_term === nothing ? Unit(q.unit) : q.unit_term
end
function _quantity_units_commensurable(left::Unit, right::Unit)
    (!reduced(left) || !reduced(right)) ? left == right : commensurable(left, right)
end
"""Express the magnitude of `q` in a commensurable unit."""
function in_unit(q::Quantity, unit::Unit)
    source = _unit(q)
    _quantity_units_commensurable(source, unit) ||
        throw(IncommensurableUnitsError("express", source, unit))
    (!reduced(source) || !reduced(unit)) && return q.magnitude
    source.scale_den * unit.scale_num == 0 &&
        throw(DivideError())
    return q.magnitude * (source.scale_num * unit.scale_den) /
           (source.scale_den * unit.scale_num)
end
in_unit(q::Quantity, unit::Quantity) = in_unit(q, _unit(unit))
"""Return `q` converted to `unit`."""
function to_unit(q::Quantity, unit::Unit)
    Quantity(in_unit(q, unit), unit.text, unit)
end
to_unit(q::Quantity, unit::Quantity) = to_unit(q, _unit(unit))
function _sum_quantities(left::Quantity, right::Quantity, sign::Int)
    target = _unit(left)
    other = _unit(right)
    _quantity_units_commensurable(target, other) ||
        throw(IncommensurableUnitsError(sign > 0 ? "add" : "subtract", target, other))
    Quantity(left.magnitude + sign * in_unit(right, target), left.unit, left.unit_term)
end
Base.:+(left::Quantity, right::Quantity) = _sum_quantities(left, right, 1)
Base.:-(left::Quantity, right::Quantity) = _sum_quantities(left, right, -1)
Base.:-(q::Quantity) = Quantity(-q.magnitude, q.unit, q.unit_term)
Base.abs(q::Quantity) = Quantity(abs(q.magnitude), q.unit, q.unit_term)
Base.:*(q::Quantity, n::Real) = Quantity(q.magnitude * n, q.unit, q.unit_term)
Base.:*(n::Real, q::Quantity) = q * n
Base.:/(q::Quantity, n::Real) = Quantity(q.magnitude / n, q.unit, q.unit_term)
function _exact_base_magnitude(q::Quantity, unit::Unit)
    magnitude = q.magnitude
    magnitude isa Union{Integer,Rational} && !(magnitude isa Bool) ||
        return nothing
    isfinite(unit.scale_num) && isinteger(unit.scale_num) ||
        return nothing
    isfinite(unit.scale_den) && isinteger(unit.scale_den) ||
        return nothing
    (unit.scale_num == 0 || unit.scale_den == 0) && return nothing
    Rational{BigInt}(magnitude) * BigInt(round(unit.scale_num)) // BigInt(round(unit.scale_den))
end
function Base.:(==)(left::Quantity, right::Quantity)
    a, b = _unit(left), _unit(right)
    if !reduced(a) || !reduced(b)
        return left.unit == right.unit && left.magnitude == right.magnitude
    end
    commensurable(a, b) || return false
    (zero_scale(a) || zero_scale(b)) && return false
    left_exact = _exact_base_magnitude(left, a)
    right_exact = _exact_base_magnitude(right, b)
    left_exact !== nothing && right_exact !== nothing &&
        return left_exact == right_exact
    return left.magnitude * a.scale_num / a.scale_den ==
           right.magnitude * b.scale_num / b.scale_den
end
function Base.isless(left::Quantity, right::Quantity)
    a, b = _unit(left), _unit(right)
    _quantity_units_commensurable(a, b) ||
        throw(IncommensurableUnitsError("order", a, b))
    if !reduced(a) || !reduced(b)
        return left.magnitude < right.magnitude
    end
    left_exact = _exact_base_magnitude(left, a)
    right_exact = _exact_base_magnitude(right, b)
    left_exact !== nothing && right_exact !== nothing &&
        return left_exact < right_exact
    return left.magnitude * a.scale_num / a.scale_den <
           right.magnitude * b.scale_num / b.scale_den
end
function Base.hash(value::Quantity, seed::UInt)
    unit = _unit(value)
    if !reduced(unit) || zero_scale(unit)
        return hash((value.magnitude, value.unit), seed)
    end
    exact = _exact_base_magnitude(value, unit)
    base = exact === nothing ? value.magnitude * unit.scale_num / unit.scale_den : exact
    dimensions = Tuple(sort!(collect(exponents(unit)); by=first))
    hash((base, dimensions), seed)
end
Base.show(io::IO, q::Quantity) = print(io, "$(q.magnitude) [$(q.unit)]")

function asreal(x)
    if x isa AbstractString
        x == "NaN" && return NaN
        x == "Infinity" && return Inf
        x == "-Infinity" && return -Inf
        return parse(Float64, x)
    end
    Float64(x)
end

const VALUE_ARMS = Set([
    "intValue", "bigIntValue", "rationalValue", "realValue", "boolValue", "stringValue", "instanceId", "sequence",
    "null", "unset", "quantity", "enumLiteral", "complex", "array", "vector",
    "vectorQuantity", "measurementRef", "infinity", "function", "set",
    "tensorQuantity", "metaobject", "undetermined",
])

function is_value_object(value)
    value isa AbstractDict && length(value) == 1 || return false
    arm = first(keys(value))
    arm in VALUE_ARMS || return false
    arm in ("intValue", "realValue", "boolValue", "stringValue", "instanceId") &&
        value[arm] isa AbstractDict && return false
    return true
end

# The Integer beyond Int64 a bigIntValue spells: canonical decimal digits, an
# optional `-` and no leading zero. One within Int64 travels as intValue.
function parse_big_integer(digits::AbstractString)
    occursin(r"^-?[1-9][0-9]*$", digits) || error("not the decimal digits of an integer: $(repr(digits))")
    n = parse(BigInt, digits)
    typemin(Int64) <= n <= typemax(Int64) && error("$digits is within Int64, which intValue carries")
    return n
end

# The exact Rational a rationalValue spells: canonical decimal terms in lowest
# terms over a positive denominator, of a value no Float64 holds.
function parse_rational(terms)
    terms isa AbstractDict || error("not a rational: $(repr(terms))")
    numerator, denominator = string(get(terms, "numerator", "")), string(get(terms, "denominator", ""))
    occursin(r"^(0|-?[1-9][0-9]*)$", numerator) && occursin(r"^[1-9][0-9]*$", denominator) ||
        error("not the decimal terms of a rational: $(repr(numerator))/$(repr(denominator))")
    n, d = parse(BigInt, numerator), parse(BigInt, denominator)
    gcd(n, d) == 1 || error("$numerator/$denominator is not in lowest terms")
    q = n // d
    _binary64(q) && error("$numerator/$denominator is a Float64, which realValue carries")
    return q
end

# Whether a Float64 holds the rational exactly.
_binary64(q::Rational) = (x = Float64(q); isfinite(x) && Rational{BigInt}(x) == q)

# A Rational as KerML holds it: lowest terms over BigInt.
_exact_rational(q::Rational) = Rational{BigInt}(q)

# A Rational on the wire: realValue when a Float64 holds it exactly, rationalValue otherwise.
rational_arm(q::Rational, real::String, rational::String) =
    _binary64(q) ? (real => _json_real(Float64(q))) :
    (rational => Dict{String,Any}("numerator" => string(numerator(q)),
                                  "denominator" => string(denominator(q))))

# An Integer on the wire: intValue within Int64, bigIntValue beyond it.
integer_arm(x::Integer, small::String, big::String) =
    typemin(Int64) <= x <= typemax(Int64) ? (small => string(Int64(x))) : (big => string(BigInt(x)))

_unsupported_value(message) = throw(UnsupportedValueError(String(message)))

function decode_quantity(q)
    magnitude = haskey(q, "intMagnitude") ? parse(Int64, q["intMagnitude"]) :
                haskey(q, "bigIntMagnitude") ? parse_big_integer(q["bigIntMagnitude"]) :
                haskey(q, "rationalMagnitude") ? parse_rational(q["rationalMagnitude"]) :
                haskey(q, "realMagnitude") ? asreal(q["realMagnitude"]) :
                _unsupported_value("quantity carries neither intMagnitude nor realMagnitude")
    term = get(q, "unitTerm", nothing)
    Quantity(magnitude, get(q, "unit", ""), term === nothing ? nothing : Unit(get(q, "unit", ""), term))
end

function decode_elements(body, key)
    return [decode_value(e) for e in get(body, key, Any[])]
end

"""Decode one JSON-transcoded SysML value arm."""
function decode_value(v)
    v === nothing && return missing
    haskey(v, "intValue") && return parse(Int64, v["intValue"])
    haskey(v, "bigIntValue") && return parse_big_integer(v["bigIntValue"])
    haskey(v, "rationalValue") && return parse_rational(v["rationalValue"])
    haskey(v, "realValue") && return asreal(v["realValue"])
    haskey(v, "boolValue") && return v["boolValue"]::Bool
    haskey(v, "stringValue") && return v["stringValue"]::String
    haskey(v, "instanceId") && return InstanceRef(parse(Int64, v["instanceId"]))
    haskey(v, "sequence") && return decode_elements(v["sequence"], "elements")
    haskey(v, "null") && return isempty(v["null"]) ? nothing :
        throw(UnsupportedValueError(String(v["null"])))
    haskey(v, "unset") && return Unset()
    haskey(v, "quantity") && return decode_quantity(v["quantity"])
    haskey(v, "enumLiteral") && begin
        l = v["enumLiteral"]
        scalar = haskey(l, "value") ? decode_value(l["value"]) : nothing
        return EnumLiteral(l["literalId"], l["enumerationId"], get(l, "name", ""), scalar)
    end
    haskey(v, "complex") && begin
        c = v["complex"]
        return complex(asreal(get(c, "real", 0.0)), asreal(get(c, "imaginary", 0.0)))
    end
    haskey(v, "array") && begin
        a = v["array"]
        dims = Int64[parse(Int64, d) for d in get(a, "dimensions", Any[])]
        any(<=(0), dims) && _unsupported_value("array dimension is not positive: $dims")
        elements = decode_elements(a, "elements")
        prod(dims; init=1) == length(elements) ||
            _unsupported_value("array has $(length(elements)) elements for dimensions $dims")
        return ArrayValue(dims, elements)
    end
    haskey(v, "vector") && begin
        components = map(get(v["vector"], "components", Any[])) do c
            haskey(c, "intValue") && return parse(Int64, c["intValue"])
            haskey(c, "bigIntValue") && return parse_big_integer(c["bigIntValue"])
            haskey(c, "rationalValue") && return parse_rational(c["rationalValue"])
            haskey(c, "realValue") && return asreal(c["realValue"])
            _unsupported_value("vector component is not an intValue or realValue: $(first(keys(c)))")
        end
        return VectorValue(Union{Int64,Float64}[c for c in components])
    end
    haskey(v, "vectorQuantity") && begin
        components = get(v["vectorQuantity"], "components", Any[])
        isempty(components) && _unsupported_value("vectorQuantity has no components")
        return VectorQuantity(Quantity[decode_quantity(c) for c in components])
    end
    haskey(v, "measurementRef") && begin
        m = v["measurementRef"]
        unit = get(m, "unit", "")
        unit_id = get(m, "unitId", nothing)
        (isempty(unit) && unit_id === nothing) &&
            _unsupported_value("measurementRef carries neither unit nor unitId")
        (!isempty(unit) || unit_id !== nothing) && !haskey(m, "unitTerm") &&
            _unsupported_value("measurementRef carries a unit without its unitTerm")
        return MeasurementRef(unit, unit_id, Unit(unit, get(m, "unitTerm", Dict{String,Any}())))
    end
    haskey(v, "infinity") && begin
        v["infinity"] === true || _unsupported_value("infinity arm does not carry true")
        return Infinity()
    end
    haskey(v, "function") && begin
        f = v["function"]
        calc_id = get(f, "calcId", "")
        isempty(calc_id) && _unsupported_value("function carries no calcId")
        self_id = get(f, "selfId", "0")
        self = self_id == "0" || self_id == 0 ? nothing : InstanceRef(parse(Int64, string(self_id)))
        return FunctionRef(calc_id, self)
    end
    haskey(v, "set") && begin
        elements = decode_elements(v["set"], "elements")
        for i in eachindex(elements), j in firstindex(elements):i-1
            same_value(elements[i], elements[j]) && _unsupported_value("set lists a member more than once")
        end
        return Set(elements)
    end
    haskey(v, "tensorQuantity") && begin
        t = v["tensorQuantity"]
        dims = Int64[parse(Int64, d) for d in get(t, "dimensions", Any[])]
        any(<=(0), dims) && _unsupported_value("tensorQuantity dimension is not positive: $dims")
        components = [decode_quantity(c) for c in get(t, "components", Any[])]
        prod(dims; init=1) == length(components) ||
            _unsupported_value("tensorQuantity has $(length(components)) components for dimensions $dims")
        return TensorQuantity(dims, components)
    end
    haskey(v, "metaobject") && begin
        m = v["metaobject"]
        element_id = get(m, "elementId", "")
        isempty(element_id) && _unsupported_value("metaobject carries no elementId")
        return Metaobject(element_id, get(m, "metaclassId", ""))
    end
    haskey(v, "undetermined") && begin
        u = v["undetermined"]
        count = get(u, "count", Dict{String,Any}())
        return Undetermined(get(u, "reason", ""), get(count, "lower", ""), get(count, "upper", ""))
    end
    arm = isempty(v) ? "<none>" : first(keys(v))
    _unsupported_value("unknown Value arm: $(arm)")
end

function _json_real(value::Real)
    number = Float64(value)
    isnan(number) ? "NaN" : isinf(number) ? (number > 0 ? "Infinity" : "-Infinity") :
    number
end

function encode_quantity(q::Quantity)
    body = Dict{String,Any}()
    if q.magnitude isa Integer
        push!(body, integer_arm(q.magnitude, "intMagnitude", "bigIntMagnitude"))
    elseif q.magnitude isa Rational
        push!(body, rational_arm(q.magnitude, "realMagnitude", "rationalMagnitude"))
    else
        body["realMagnitude"] = _json_real(Float64(q.magnitude))
    end
    isempty(q.unit) || (body["unit"] = q.unit)
    if q.unit_term !== nothing
        body["unitTerm"] = Dict{String,Any}(
            "scaleNum" => _json_real(q.unit_term.scale_num),
            "scaleDen" => _json_real(q.unit_term.scale_den),
            "factors" => Any[Dict("unitId" => f.unit_id, "exponent" => _json_real(f.exponent))
                             for f in q.unit_term.factors])
    end
    return body
end

"""Encode a Julia value as the corresponding SysML value arm."""
function encode_value(x::Bool)
    Dict{String,Any}("boolValue" => x)
end
encode_value(x::Integer) = Dict{String,Any}(integer_arm(x, "intValue", "bigIntValue"))
encode_value(x::Rational) = Dict{String,Any}(rational_arm(x, "realValue", "rationalValue"))
encode_value(x::AbstractFloat) = Dict{String,Any}("realValue" => _json_real(x))
encode_value(x::AbstractString) = Dict{String,Any}("stringValue" => String(x))
encode_value(::Nothing) = Dict{String,Any}("null" => "")
encode_value(x::Complex) =
    Dict{String,Any}("complex" => Dict{String,Any}(
        "real" => _json_real(Float64(real(x))),
        "imaginary" => _json_real(Float64(imag(x)))))
encode_value(x::InstanceRef) = Dict{String,Any}("instanceId" => string(x.id))
function encode_value(ref::MeasurementRef)
    body = Dict{String,Any}("unit" => ref.unit)
    ref.unit_id === nothing || (body["unitId"] = ref.unit_id)
    if ref.unit_term !== nothing
        body["unitTerm"] = Dict{String,Any}(
            "scaleNum" => _json_real(ref.unit_term.scale_num),
            "scaleDen" => _json_real(ref.unit_term.scale_den),
            "factors" => Any[Dict("unitId" => f.unit_id, "exponent" => _json_real(f.exponent))
                             for f in ref.unit_term.factors])
    end
    Dict{String,Any}("measurementRef" => body)
end
encode_value(v::ArrayValue) =
    Dict{String,Any}("array" => Dict("dimensions" => string.(v.dimensions),
                                      "elements" => Any[encode_value(e) for e in v.elements]))
encode_value(v::VectorValue) =
    Dict{String,Any}("vector" => Dict("components" => Any[encode_value(e) for e in v.components]))
encode_value(v::VectorQuantity) =
    Dict{String,Any}("vectorQuantity" => Dict("components" => Any[encode_quantity(e) for e in v.components]))
encode_value(v::TensorQuantity) =
    Dict{String,Any}("tensorQuantity" => Dict("dimensions" => string.(v.dimensions),
                                               "components" => Any[encode_quantity(e) for e in v.components]))
encode_value(q::Quantity) = Dict{String,Any}("quantity" => encode_quantity(q))
encode_value(l::EnumLiteral) = begin
    body = Dict{String,Any}("literalId" => l.literal_id, "enumerationId" => l.enumeration_id, "name" => l.name)
    l.value === nothing || (body["value"] = encode_value(l.value))
    Dict{String,Any}("enumLiteral" => body)
end
function encode_value(f::FunctionRef)
    f.self === nothing ||
        throw(ArgumentError("a function read off an object cannot be sent: selfId names no instance in another call"))
    Dict{String,Any}("function" => Dict{String,Any}("calcId" => f.calc_id))
end
encode_value(u::Undetermined) =
    throw(ArgumentError("an undetermined value cannot be sent: $(u.reason)"))
encode_value(u::Unset) = throw(ArgumentError("an unset value cannot be sent"))
encode_value(i::Infinity) = Dict{String,Any}("infinity" => true)
encode_value(x::AbstractVector) =
    Dict{String,Any}("sequence" => Dict{String,Any}("elements" => Any[encode_value(e) for e in x]))
encode_value(x::AbstractSet) =
    Dict{String,Any}("set" => Dict{String,Any}("elements" => Any[encode_value(e) for e in x]))
encode_value(m::Metaobject) =
    Dict{String,Any}("metaobject" => Dict("elementId" => m.element_id, "metaclassId" => m.metaclass_id))

function decode_value_or_unsupported(raw)
    try
        decoded = decode_value(raw)
        if decoded isa Quantity && !isempty(decoded.unit) && decoded.unit_term === nothing
            return UnsupportedValueError(
                "quantity in [$(decoded.unit)] carries no reduction to base units")
        end
        decoded
    catch error
        error isa UnsupportedValueError && return error
        error isa Union{ErrorException,ArgumentError,OverflowError,InexactError,TypeError} ||
            rethrow()
        UnsupportedValueError(sprint(showerror, error))
    end
end

"""Compare values using model-value identity and empty-value rules."""
function _same_unordered(left, right)
    length(left) == length(right) || return false
    remaining = collect(right)
    for item in left
        index = findfirst(other -> same_value(item, other), remaining)
        index === nothing && return false
        deleteat!(remaining, index)
    end
    true
end
function same_value(left, right)
    _empty_value(left) && _empty_value(right) && return true
    if left isa Quantity && right isa Quantity
        return left == right
    elseif left isa EnumLiteral && right isa EnumLiteral
        return left.literal_id == right.literal_id
    elseif left isa Metaobject && right isa Metaobject
        return left == right
    elseif left isa MeasurementRef && right isa MeasurementRef
        return left == right
    elseif left isa FunctionRef && right isa FunctionRef
        return left == right
    elseif left isa ArrayValue && right isa ArrayValue
        return left.dimensions == right.dimensions &&
               same_value(left.elements, right.elements)
    elseif left isa VectorValue && right isa VectorValue
        return same_value(left.components, right.components)
    elseif left isa VectorQuantity && right isa VectorQuantity
        return same_value(left.components, right.components)
    elseif left isa TensorQuantity && right isa TensorQuantity
        return left.dimensions == right.dimensions &&
               same_value(left.components, right.components)
    elseif left isa AbstractDict && right isa AbstractDict
        return left == right
    elseif left isa AbstractVector && right isa AbstractVector
        return length(left) == length(right) && all(same_value(a, b) for (a, b) in zip(left, right))
    elseif left isa AbstractSet && right isa AbstractSet
        return _same_unordered(left, right)
    end
    return isequal(left, right)
end

_empty_value(value) = value === nothing ||
    (value isa Union{AbstractVector,AbstractSet} && isempty(value))

"""Return the array contents unfolded according to its dimensions."""
function nested(value::ArrayValue)
    function unfold(offset, dims)
        isempty(dims) && return value.elements[offset]
        stride = prod(dims[2:end]; init=1)
        Any[unfold(offset + (i - 1) * stride, dims[2:end]) for i in 1:dims[1]]
    end
    unfold(1, value.dimensions)
end

"""Return the service capabilities required to encode `value`."""
function value_capabilities(value)
    capabilities = Set{String}()
    function visit(item)
        if item isa Integer && !(item isa Bool)
            typemin(Int64) <= item <= typemax(Int64) || push!(capabilities, CAPABILITY_BIG_INT_VALUES)
        elseif item isa Rational
            _binary64(item) || push!(capabilities, CAPABILITY_RATIONAL_VALUES)
        elseif item isa Quantity
            visit(item.magnitude)
        elseif item isa Complex
            push!(capabilities, CAPABILITY_COMPLEX_VALUES)
        elseif item isa MeasurementRef
            push!(capabilities, CAPABILITY_MEASUREMENT_REFS)
        elseif item isa FunctionRef
            push!(capabilities, CAPABILITY_FUNCTION_VALUES)
        elseif item isa AbstractSet
            push!(capabilities, CAPABILITY_SET_VALUES)
            foreach(visit, item)
        elseif item isa TensorQuantity
            push!(capabilities, CAPABILITY_TENSOR_VALUES)
            push!(capabilities, CAPABILITY_STRUCTURED_VALUES)
            foreach(visit, item.components)
        elseif item isa Union{ArrayValue,VectorValue,VectorQuantity}
            push!(capabilities, CAPABILITY_STRUCTURED_VALUES)
            item isa ArrayValue && foreach(visit, item.elements)
            item isa VectorValue && foreach(visit, item.components)
            item isa VectorQuantity && foreach(visit, item.components)
        elseif item isa Metaobject
            push!(capabilities, CAPABILITY_METAOBJECT_VALUES)
        elseif item isa EnumLiteral
            item.value === nothing || visit(item.value)
        elseif item isa Infinity
            push!(capabilities, CAPABILITY_INFINITY_VALUE)
        elseif item isa Undetermined
            push!(capabilities, CAPABILITY_UNDETERMINED_VALUE)
        elseif item isa AbstractVector || item isa Tuple
            foreach(visit, item)
        elseif item isa AbstractDict
            foreach(visit, values(item))
        end
    end
    visit(value)
    return capabilities
end

# Decodes every Value object nested inside a response tree, leaving the rest of
# the record's shape intact.
function decode_values(x::AbstractDict)
    if is_value_object(x)
        return decode_value_or_unsupported(x)
    end
    out = Dict{String,Any}()
    for (k, v) in x
        out[k] = decode_values(v)
    end
    out
end
decode_values(x::AbstractVector) = Any[decode_values(e) for e in x]
decode_values(x) = x
