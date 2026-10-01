struct Unset end
struct Infinity end
struct InstanceRef
    id::Int64
end
struct Quantity
    magnitude::Union{Int64,Float64}
    unit::String
    unit_term
end
struct EnumLiteral
    literal_id::String
    enumeration_id::String
    name::String
    value::Any
end
EnumLiteral(literal_id, enumeration_id, name) = EnumLiteral(literal_id, enumeration_id, name, nothing)
struct FunctionRef
    calc_id::String
    self::Union{InstanceRef,Nothing}
end
struct Metaobject
    element_id::String
    metaclass_id::String
end
struct Undetermined
    reason::String
    lower::String
    upper::String
end

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
    "intValue", "realValue", "boolValue", "stringValue", "instanceId", "sequence",
    "null", "unset", "quantity", "enumLiteral", "complex", "array", "vector",
    "vectorQuantity", "measurementRef", "infinity", "function", "set",
    "tensorQuantity", "metaobject", "undetermined",
])

is_value_object(v) =
    v isa AbstractDict && length(v) == 1 && first(keys(v)) in VALUE_ARMS

function decode_quantity(q)
    magnitude = haskey(q, "intMagnitude") ? parse(Int64, q["intMagnitude"]) :
                haskey(q, "realMagnitude") ? asreal(q["realMagnitude"]) :
                error("quantity carries neither intMagnitude nor realMagnitude")
    Quantity(magnitude, get(q, "unit", ""), get(q, "unitTerm", nothing))
end

function decode_elements(body, key)
    return [decode_value(e) for e in get(body, key, Any[])]
end

function decode_value(v)
    v === nothing && return missing
    haskey(v, "intValue") && return parse(Int64, v["intValue"])
    haskey(v, "realValue") && return asreal(v["realValue"])
    haskey(v, "boolValue") && return v["boolValue"]::Bool
    haskey(v, "stringValue") && return v["stringValue"]::String
    haskey(v, "instanceId") && return InstanceRef(parse(Int64, v["instanceId"]))
    haskey(v, "sequence") && return decode_elements(v["sequence"], "elements")
    haskey(v, "null") && return isempty(v["null"]) ? nothing :
        error("unsupported value: $(v["null"])")
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
        any(<=(0), dims) && error("array dimension is not positive: $dims")
        elements = decode_elements(a, "elements")
        prod(dims; init=1) == length(elements) ||
            error("array has $(length(elements)) elements for dimensions $dims")
        return (dimensions=dims, elements=elements)
    end
    haskey(v, "vector") && begin
        components = map(get(v["vector"], "components", Any[])) do c
            haskey(c, "intValue") && return parse(Int64, c["intValue"])
            haskey(c, "realValue") && return asreal(c["realValue"])
            error("vector component is not an intValue or realValue: $(first(keys(c)))")
        end
        return (components=components,)
    end
    haskey(v, "vectorQuantity") && begin
        components = get(v["vectorQuantity"], "components", Any[])
        isempty(components) && error("vectorQuantity has no components")
        return (components=[decode_quantity(c) for c in components],)
    end
    haskey(v, "measurementRef") && begin
        m = v["measurementRef"]
        unit = get(m, "unit", "")
        unit_id = get(m, "unitId", nothing)
        (isempty(unit) && unit_id === nothing) &&
            error("measurementRef carries neither unit nor unitId")
        (!isempty(unit) || unit_id !== nothing) && !haskey(m, "unitTerm") &&
            error("measurementRef carries a unit without its unitTerm")
        return (unit=unit, unit_id=unit_id, unit_term=get(m, "unitTerm", nothing))
    end
    haskey(v, "infinity") && begin
        v["infinity"] === true || error("infinity arm does not carry true")
        return Infinity()
    end
    haskey(v, "function") && begin
        f = v["function"]
        calc_id = get(f, "calcId", "")
        isempty(calc_id) && error("function carries no calcId")
        self_id = get(f, "selfId", "0")
        self = self_id == "0" || self_id == 0 ? nothing : InstanceRef(parse(Int64, string(self_id)))
        return FunctionRef(calc_id, self)
    end
    haskey(v, "set") && begin
        elements = decode_elements(v["set"], "elements")
        length(unique(elements)) == length(elements) ||
            error("set lists a member more than once")
        return Set(elements)
    end
    haskey(v, "tensorQuantity") && begin
        t = v["tensorQuantity"]
        dims = Int64[parse(Int64, d) for d in get(t, "dimensions", Any[])]
        any(<=(0), dims) && error("tensorQuantity dimension is not positive: $dims")
        components = [decode_quantity(c) for c in get(t, "components", Any[])]
        prod(dims; init=1) == length(components) ||
            error("tensorQuantity has $(length(components)) components for dimensions $dims")
        return (dimensions=dims, components=components)
    end
    haskey(v, "metaobject") && begin
        m = v["metaobject"]
        element_id = get(m, "elementId", "")
        isempty(element_id) && error("metaobject carries no elementId")
        return Metaobject(element_id, get(m, "metaclassId", ""))
    end
    haskey(v, "undetermined") && begin
        u = v["undetermined"]
        count = get(u, "count", Dict{String,Any}())
        return Undetermined(get(u, "reason", ""), get(count, "lower", ""), get(count, "upper", ""))
    end
    error("unknown Value arm: $(first(keys(v)))")
end

function encode_quantity(q::Quantity)
    body = Dict{String,Any}()
    if q.magnitude isa Int64
        body["intMagnitude"] = string(q.magnitude)
    else
        body["realMagnitude"] = q.magnitude
    end
    isempty(q.unit) || (body["unit"] = q.unit)
    q.unit_term === nothing || (body["unitTerm"] = q.unit_term)
    return body
end

function encode_value(x::Bool)
    Dict{String,Any}("boolValue" => x)
end
encode_value(x::Integer) = Dict{String,Any}("intValue" => string(Int64(x)))
encode_value(x::AbstractFloat) = Dict{String,Any}("realValue" => Float64(x))
encode_value(x::AbstractString) = Dict{String,Any}("stringValue" => String(x))
encode_value(::Nothing) = Dict{String,Any}("null" => "")
encode_value(x::Complex) =
    Dict{String,Any}("complex" => Dict{String,Any}("real" => Float64(real(x)), "imaginary" => Float64(imag(x))))
encode_value(x::InstanceRef) = Dict{String,Any}("instanceId" => string(x.id))
encode_value(q::Quantity) = Dict{String,Any}("quantity" => encode_quantity(q))
encode_value(l::EnumLiteral) = begin
    body = Dict{String,Any}("literalId" => l.literal_id, "enumerationId" => l.enumeration_id, "name" => l.name)
    l.value === nothing || (body["value"] = encode_value(l.value))
    Dict{String,Any}("enumLiteral" => body)
end
function encode_value(f::FunctionRef)
    f.self === nothing ||
        error("a function read off an object cannot be sent: selfId names no instance in another call")
    Dict{String,Any}("function" => Dict{String,Any}("calcId" => f.calc_id))
end
encode_value(u::Undetermined) =
    error("an undetermined value cannot be sent: $(u.reason)")
encode_value(u::Unset) = error("an unset value cannot be sent")
encode_value(i::Infinity) = Dict{String,Any}("infinity" => true)
encode_value(x::AbstractVector) =
    Dict{String,Any}("sequence" => Dict{String,Any}("elements" => Any[encode_value(e) for e in x]))
encode_value(x::AbstractSet) =
    Dict{String,Any}("set" => Dict{String,Any}("elements" => Any[encode_value(e) for e in x]))

# Decodes every Value object nested inside a response tree, leaving the rest of
# the record's shape intact.
function decode_values(x::AbstractDict)
    if is_value_object(x)
        return decode_value(x)
    end
    out = Dict{String,Any}()
    for (k, v) in x
        out[k] = decode_values(v)
    end
    out
end
decode_values(x::AbstractVector) = Any[decode_values(e) for e in x]
decode_values(x) = x
