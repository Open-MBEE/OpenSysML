"""
    ElementIdentity(id, qualified_name, metaclass)

An element the model's element form knows: one of the model, or a standard-library
element it references. `id` is the form's `@id`, accepted back as the element to read;
`qualified_name` is empty for an unnamed element.
"""
struct ElementIdentity
    id::String
    qualified_name::String
    metaclass::String
end
ElementIdentity(raw::AbstractDict) = ElementIdentity(String(get(raw, "id", "")),
                                                     String(get(raw, "qualifiedName", "")),
                                                     String(get(raw, "metaclass", "")))
Base.show(io::IO, identity::ElementIdentity) =
    print(io, isempty(identity.qualified_name) ? identity.id : identity.qualified_name)

"""
    ElementProperty

One metamodel property of an element, by its specification name: the metaclass
declaring it (`defining_class`, empty when none does), whether it is a collection
(`many`) and derived, its `values` in order — an [`ElementIdentity`](@ref), a `String`,
a `Bool`, an `Int64`, a `Float64`, or the decimal text of a number beyond those — and
`unsupported`, the reason it has no answer when it cannot be answered (empty when it was).
"""
struct ElementProperty
    name::String
    defining_class::String
    many::Bool
    derived::Bool
    values::Vector{Any}
    unsupported::String
end

"""The element references among a property's values, in order."""
property_elements(property::ElementProperty) =
    ElementIdentity[v for v in property.values if v isa ElementIdentity]

"""The one value of a single-valued property, `nothing` when it has none."""
property_value(property::ElementProperty) =
    isempty(property.values) ? nothing : property.values[1]

"""Whether the property was answered, with or without values."""
is_answered(property::ElementProperty) = isempty(property.unsupported)

"""
    ElementProperties

What one `GetElementProperties` read answered: the `element` and its `properties`, in
the order asked for (the element form's order when none was named). Indexes by
property name: `read["inheritedFeature"]`.
"""
struct ElementProperties
    element::ElementIdentity
    properties::Vector{ElementProperty}
    by_name::Dict{String,ElementProperty}
end
ElementProperties(element::ElementIdentity, properties::Vector{ElementProperty}) =
    ElementProperties(element, properties, Dict{String,ElementProperty}(p.name => p for p in properties))
Base.getindex(read::ElementProperties, name::AbstractString) = read.by_name[String(name)]
Base.haskey(read::ElementProperties, name::AbstractString) = haskey(read.by_name, String(name))
Base.get(read::ElementProperties, name::AbstractString, default) = get(read.by_name, String(name), default)
Base.iterate(read::ElementProperties, state...) = iterate(read.properties, state...)
Base.length(read::ElementProperties) = length(read.properties)
Base.eltype(::Type{ElementProperties}) = ElementProperty
Base.show(io::IO, read::ElementProperties) =
    print(io, "ElementProperties(", read.element, ", ", [p.name for p in read.properties], ")")

function _property_value(raw::AbstractDict)
    haskey(raw, "element") && return ElementIdentity(raw["element"])
    haskey(raw, "stringValue") && return String(raw["stringValue"])
    haskey(raw, "boolValue") && return Bool(raw["boolValue"])
    haskey(raw, "intValue") && return parse(Int64, string(raw["intValue"]))
    haskey(raw, "realValue") && return Float64(raw["realValue"])
    haskey(raw, "number") && return String(raw["number"])
    throw(ArgumentError("an ElementPropertyValue with no value: $(raw)"))
end

function _element_property(raw::AbstractDict)
    ElementProperty(String(get(raw, "name", "")), String(get(raw, "definingClass", "")),
                    Bool(get(raw, "many", false)), Bool(get(raw, "derived", false)),
                    Any[_property_value(v) for v in get(raw, "values", Any[])],
                    String(get(raw, "unsupported", "")))
end

"""
    element_properties(model, element, properties...)

Read an element's metamodel properties by their KerML/SysML specification names
(`inheritedFeature`, `connectorEnd`, `isComposite`, …), as the model's api-json export
spells them. `element` is a qualified name, the positional id of an unnamed member
(`Owner::@2`) or the id an earlier answer carried; with no `properties` every property
the element form carries is read. Requires the `element_properties` capability.
"""
function element_properties(model::Model, element::AbstractString, properties::AbstractString...)
    require_capability(model.connection, CAPABILITY_ELEMENT_PROPERTIES)
    request = Dict{String,Any}("modelHash" => model.hash, "elementId" => String(element))
    isempty(properties) || (request["properties"] = String[String(p) for p in properties])
    answer = _translate(; capabilities=(CAPABILITY_ELEMENT_PROPERTIES,), connection=model.connection) do
        call(model.connection, "GetElementProperties", request)
    end
    _check_error(answer, "GetElementProperties")
    return ElementProperties(ElementIdentity(get(answer, "element", Dict{String,Any}())),
                             ElementProperty[_element_property(p) for p in get(answer, "properties", Any[])])
end
