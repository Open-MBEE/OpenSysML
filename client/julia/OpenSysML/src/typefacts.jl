"""One derived type of a feature: its qualified name and symbol kind."""
struct TypeRef
    id::String
    kind::String
end
TypeRef(raw::AbstractDict) = TypeRef(String(get(raw, "id", "")), String(get(raw, "kind", "")))

"""
A feature's type. `declared`, `resolved_id` and `resolved_kind` are the typing as written;
`types` is the derived KerML `Feature::type` (the `type` property of `element_properties`),
which the typing of every feature it subsets, references or redefines contributes to.
"""
struct TypeFacts
    declared::String
    resolved_id::String
    resolved_kind::String
    primitive::String
    primitive_source::String
    quantity::Bool
    unit::String
    types::Vector{TypeRef}
end
TypeFacts(raw::AbstractDict) =
    TypeFacts(String(get(raw, "declared", "")), String(get(raw, "resolvedId", "")),
              String(get(raw, "resolvedKind", "")), String(get(raw, "primitive", "")),
              String(get(raw, "primitiveSource", "")), Bool(get(raw, "quantity", false)),
              String(get(raw, "unit", "")),
              TypeRef[TypeRef(ref) for ref in get(raw, "types", [])])

struct Multiplicity
    lower::String
    upper::String
end
Multiplicity(raw::AbstractDict) =
    Multiplicity(String(get(raw, "lower", "")), String(get(raw, "upper", "")))
"""Return whether a multiplicity allows multiple values, or `nothing` if unknown."""
function is_collection(m::Multiplicity)
    isempty(m.upper) && return nothing
    m.upper == "*" && return true
    parsed = tryparse(Int, m.upper)
    parsed === nothing ? nothing : parsed > 1
end
"""Return whether a multiplicity permits no value, or `nothing` if unknown."""
function is_optional(m::Multiplicity)
    isempty(m.lower) && return nothing
    parsed = tryparse(Int, m.lower)
    parsed === nothing ? nothing : parsed == 0
end

"""
One generalization edge written on a symbol; `kind` is `specializes`, `subsets`, `redefines`,
`typing` or `references` (reference subsetting, `::>` and `include`).
"""
struct Specialization
    kind::String
    declared::String
    target_id::String
    target_kind::String
end
Specialization(raw::AbstractDict) =
    Specialization(String(get(raw, "kind", "")), String(get(raw, "declared", "")),
                   String(get(raw, "targetId", "")), String(get(raw, "targetKind", "")))

struct AttributeFacts
    name::String
    type::String
    value::Any
    unit::String
end
function AttributeFacts(raw::AbstractDict)
    value = haskey(raw, "value") ? decode_value(raw["value"]) : nothing
    AttributeFacts(String(get(raw, "name", "")), String(get(raw, "type", "")),
                   value, String(get(raw, "unit", "")))
end

struct SymbolFacts
    id::String
    name::String
    kind::String
    type::Union{Nothing,TypeFacts}
    multiplicity::Union{Nothing,Multiplicity}
    specializations::Vector{Specialization}
    attributes::Vector{AttributeFacts}
    withheld_library_attributes::Int
end
