struct ElementRef
    id::String
    type::String
end
ElementRef(id::AbstractString, type::AbstractString="") = ElementRef(String(id), String(type))
Base.show(io::IO, element::ElementRef) =
    print(io, isempty(element.type) ? element.id : "$(element.id) ($(element.type))")

struct ObjectRef
    id::Int64
    path::String
    element::Union{Nothing,ElementRef}
end
ObjectRef(id::Integer=0, path::AbstractString="", element=nothing) =
    ObjectRef(Int64(id), String(path), element)
Base.show(io::IO, object::ObjectRef) = print(io, isempty(object.path) ? "#$(object.id)" : object.path)

struct DocumentEvent
    kind::String
    time::Any
    text::String
    object::Union{Nothing,ObjectRef}
    machine::String
    state::String
    from_state::String
    to_state::String
    target::Union{Nothing,ObjectRef}
    event::String
    payload::Vector{String}
    alternatives::Vector{String}
    taken::String
end
Base.show(io::IO, event::DocumentEvent) = print(io, "$(event.time): $(event.text)")
