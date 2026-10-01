abstract type TypedObject end

const _TYPED_IDS = IdDict{Any,String}()
const _TYPED_BASES = Dict{String,Vector{String}}()
const _GENERATED_TYPE_IDS = Set{String}()

function _register_typed!(::Type{T}, id::AbstractString, bases=String[]) where {T<:TypedObject}
    type_id = String(id)
    _TYPED_IDS[T] = type_id
    _TYPED_BASES[type_id] = unique(String[String(base) for base in bases])
    push!(_GENERATED_TYPE_IDS, type_id)
    nothing
end

function _typed_subtype(actual::String, expected::String, visited=Set{String}())
    actual == expected && return true
    actual in visited && return false
    push!(visited, actual)
    any(base -> _typed_subtype(base, expected, visited), get(_TYPED_BASES, actual, String[]))
end

function from_instance(::Type{T}, instance::Instance) where {T<:TypedObject}
    expected = get(_TYPED_IDS, T, "")
    actual = instance.type_symbol_id
    if !isempty(actual) && actual in _GENERATED_TYPE_IDS &&
       !_typed_subtype(actual, expected)
        throw(InstanceTypeError(isempty(expected) ? string(T) : expected, actual))
    end
    T(instance)
end

unchecked(::Type{T}, instance::Instance) where {T<:TypedObject} = T(instance)

Base.:(==)(left::TypedObject, right::TypedObject) =
    typeof(left) === typeof(right) && left.instance.id == right.instance.id
Base.hash(value::TypedObject, seed::UInt) = hash((typeof(value), value.instance.id), seed)
Base.show(io::IO, value::TypedObject) =
    print(io, "$(nameof(typeof(value)))(instance=$(repr(value.instance)))")

_type_mismatch(feature_name, expected, value) =
    TypeMismatchError(String(feature_name), String(expected), value)

function as_bool(feature_name, value)
    value isa Bool || throw(_type_mismatch(feature_name, "bool", value))
    value
end

function as_int(feature_name, value)
    value isa Integer && !(value isa Bool) ||
        throw(_type_mismatch(feature_name, "int", value))
    value
end

function as_float(feature_name, value)
    value isa Real && !(value isa Bool) ||
        throw(_type_mismatch(feature_name, "float", value))
    Float64(value)
end

function as_complex(feature_name, value)
    (value isa Real || value isa Complex) && !(value isa Bool) ||
        throw(_type_mismatch(feature_name, "complex", value))
    ComplexF64(value)
end

function as_str(feature_name, value)
    value isa AbstractString || throw(_type_mismatch(feature_name, "str", value))
    String(value)
end

function as_quantity(feature_name, value)
    value isa Quantity || throw(_type_mismatch(feature_name, "Quantity", value))
    value
end

function as_enum_literal(feature_name, value)
    value isa EnumLiteral || throw(_type_mismatch(feature_name, "EnumLiteral", value))
    value
end

as_object(::AbstractString, value) = value

function as_typed(::Type{T}) where {T<:TypedObject}
    function decode(feature_name, value)
        value isa Instance || throw(_type_mismatch(feature_name, string(nameof(T)), value))
        from_instance(T, value)
    end
    decode
end

function feature_value(obj::TypedObject, feature_name::AbstractString, decode::Function)
    instance = obj.instance
    haskey(instance, feature_name) ||
        throw(_type_mismatch(feature_name, "a value", nothing))
    value = instance[feature_name]
    value === nothing && throw(_type_mismatch(feature_name, "a value", nothing))
    value isa AbstractVector &&
        throw(_type_mismatch(feature_name, "a single value", value))
    decode(String(feature_name), value)
end

function optional_feature_value(obj::TypedObject, feature_name::AbstractString, decode::Function)
    instance = obj.instance
    haskey(instance, feature_name) || return nothing
    value = instance[feature_name]
    (value === nothing || value isa Unset) && return nothing
    if value isa AbstractVector
        isempty(value) && return nothing
        length(value) <= 1 ||
            throw(_type_mismatch(feature_name, "at most one value", value))
        value = first(value)
    end
    decode(String(feature_name), value)
end

list_feature_value(obj::TypedObject, feature_name::AbstractString, decode::Function) =
    list_feature_value(obj, feature_name, decode, Any)

function list_feature_value(obj::TypedObject, feature_name::AbstractString,
                            decode::Function, ::Type{T}) where {T}
    instance = obj.instance
    haskey(instance, feature_name) || return T[]
    value = instance[feature_name]
    (value === nothing || value isa Unset) && return T[]
    values = value isa AbstractVector ? value : Any[value]
    T[decode(String(feature_name), element) for element in values]
end
