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

struct DocumentVerdict
    assertion::ElementRef
    kind::String
    text::String
    path::String
    status::String
    condition::String
    reason::String
    verification::Vector{String}
end
Base.show(io::IO, verdict::DocumentVerdict) =
    print(io, "$(verdict.text)$(isempty(verdict.path) ? "" : " on $(verdict.path)"): $(verdict.status)")

struct DocumentState
    object::ObjectRef
    machine::String
    name::String
    path::String
    state::Union{Nothing,ElementRef}
    region::String
    enclosing::Vector{String}
end
Base.show(io::IO, state::DocumentState) =
    print(io, "$(state.object).$(state.machine) in $(state.path)")

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

struct DocumentRow
    element::ElementRef
    cells::Vector{Vector{Any}}
    verdict::Union{Nothing,DocumentVerdict}
    object::Union{Nothing,ObjectRef}
    state::Union{Nothing,DocumentState}
    event::Union{Nothing,DocumentEvent}
end
DocumentRow(element, cells; verdict=nothing, object=nothing, state=nothing, event=nothing) =
    DocumentRow(element, Vector{Any}[Any[cell...] for cell in cells], verdict, object, state, event)
Base.getindex(row::DocumentRow, index::Int) = row.cells[index]

struct DocumentQueryResult
    columns::Vector{String}
    rows::Vector{DocumentRow}
end
Base.iterate(result::DocumentQueryResult, state...) = iterate(result.rows, state...)
Base.length(result::DocumentQueryResult) = length(result.rows)

function _document_ref(raw)
    ElementRef(String(get(raw, "elementId", get(raw, "id", ""))),
               String(get(raw, "elementType", get(raw, "type", ""))))
end
function _document_object(raw)
    element_raw = get(raw, "element", nothing)
    ObjectRef(parse(Int64, string(get(raw, "instanceId", 0))), String(get(raw, "path", "")),
        element_raw isa AbstractDict ? _document_ref(element_raw) : nothing)
end

function _document_value(raw)
    haskey(raw, "elementId") && return ElementRef(String(raw["elementId"]), String(get(raw, "elementType", "")))
    haskey(raw, "stringValue") && return String(raw["stringValue"])
    haskey(raw, "intValue") && return parse(Int64, string(raw["intValue"]))
    haskey(raw, "realValue") && return asreal(raw["realValue"])
    haskey(raw, "boolValue") && return Bool(raw["boolValue"])
    haskey(raw, "infinity") && return Infinity()
    haskey(raw, "quantity") && return decode_quantity(raw["quantity"])
    if haskey(raw, "object")
        return _document_object(raw["object"])
    elseif haskey(raw, "verdict")
        v = raw["verdict"]
        a = get(v, "assertion", Dict{String,Any}())
        assertion = ElementRef(String(get(a, "elementId", "")),
                               String(get(a, "elementType", "")))
        return DocumentVerdict(assertion, String(get(v, "kind", "")),
            String(get(v, "text", "")), String(get(v, "path", "")),
            String(get(v, "verdict", "")), String(get(v, "condition", "")),
            String(get(v, "reason", "")), String[String(x) for x in get(v, "verification", Any[])])
    elseif haskey(raw, "state")
        v = raw["state"]
        obj = _document_object(get(v, "object", Dict{String,Any}()))
        state_raw = get(v, "state", nothing)
        return DocumentState(obj, String(get(v, "machine", "")), String(get(v, "name", "")),
            String(get(v, "statePath", "")), state_raw isa AbstractDict ? _document_value(state_raw) : nothing,
            String(get(v, "region", "")), String[String(x) for x in get(v, "enclosing", Any[])])
    elseif haskey(raw, "event")
        v = raw["event"]
        obj_raw = get(v, "object", nothing)
        target_raw = get(v, "target", nothing)
        time_raw = get(v, "time", Dict{String,Any}())
        time = haskey(time_raw, "quantity") ? decode_quantity(time_raw["quantity"]) :
               haskey(time_raw, "realValue") ? asreal(time_raw["realValue"]) :
               haskey(time_raw, "intValue") ? parse(Int64, string(time_raw["intValue"])) : 0.0
        return DocumentEvent(String(get(v, "kind", "")), time, String(get(v, "text", "")),
            obj_raw isa AbstractDict ? _document_object(obj_raw) : nothing,
            String(get(v, "machine", "")), String(get(v, "state", "")),
            String(get(v, "from", "")), String(get(v, "to", "")),
            target_raw isa AbstractDict ? _document_object(target_raw) : nothing,
            String(get(v, "event", "")), String[String(x) for x in get(v, "payload", Any[])],
            String[String(x) for x in get(v, "alternatives", Any[])], String(get(v, "taken", "")))
    end
    throw(UnsupportedValueError("unknown document value arm"))
end

function _document_row(raw)
    cells = Vector{Any}[Any[_document_value(v) for v in get(cell, "values", Any[])]
                        for cell in get(raw, "cells", Any[])]
    element_raw = get(raw, "element", Dict{String,Any}())
    element = ElementRef("", String(get(element_raw, "elementType", "")))
    verdict = nothing
    object = nothing
    state = nothing
    event = nothing
    if haskey(element_raw, "verdict")
        verdict = _document_value(element_raw)
        element = verdict.assertion
    elseif haskey(element_raw, "object")
        object = _document_value(element_raw)
        element = something(object.element, ElementRef(""))
    elseif haskey(element_raw, "state")
        state = _document_value(element_raw)
        object = state.object
        element = something(object.element, ElementRef(""))
    elseif haskey(element_raw, "event")
        event = _document_value(element_raw)
        object = event.object
        element = object === nothing ? ElementRef("") : something(object.element, ElementRef(""))
    elseif haskey(element_raw, "elementId")
        element = _document_value(element_raw)
    end
    DocumentRow(element, cells; verdict=verdict, object=object, state=state, event=event)
end

"""Encode element, object, and scalar bindings for a document query."""
function build_document_bindings(bindings=Dict())
    result = Any[]
    for (parameter, bound) in pairs(bindings)
        values = bound isa AbstractVector || bound isa Tuple ? bound : (bound,)
        encoded = Any[]
        for value in values
            wire = if value isa ElementRef
                Dict{String,Any}("elementId" => value.id)
            elseif value isa ObjectRef
                (value.id != 0 || !isempty(value.path)) ||
                    throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): an object is bound by id or by path; neither was given"))
                Dict{String,Any}("object" => Dict("instanceId" => string(value.id), "path" => value.path))
            elseif value isa Bool
                Dict{String,Any}("boolValue" => value)
            elseif value isa AbstractString
                Dict{String,Any}("stringValue" => String(value))
            elseif value isa Integer
                typemin(Int64) <= value <= typemax(Int64) ||
                    throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): an int must fit in a signed 64-bit integer"))
                Dict{String,Any}("intValue" => string(value))
            elseif value isa AbstractFloat
                Dict{String,Any}("realValue" => Float64(value))
            elseif value isa Quantity
                value.magnitude isa Integer && !(typemin(Int64) <= value.magnitude <= typemax(Int64)) &&
                    throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): an Integer magnitude must fit in a signed 64-bit integer"))
                !isempty(value.unit) && value.unit_term === nothing &&
                    throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): a named unit needs its unit term"))
                Dict{String,Any}("quantity" => encode_quantity(value))
            elseif value isa DocumentVerdict
                throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): a verdict is answered by queries, not bound to them"))
            elseif value isa DocumentState || value isa DocumentEvent
                what = value isa DocumentState ? "a state" : "an event"
                throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): $(what) row is answered by queries, not bound to them"))
            else
                throw(DocumentQueryError("binding $(repr(parameter)) cannot carry $(repr(value)): a binding is a str, int, float, bool, Quantity, ElementRef or ObjectRef"))
            end
            push!(encoded, wire)
        end
        push!(result, Dict{String,Any}("parameter" => String(parameter), "values" => encoded))
    end
    result
end

"""Run a document query with optional parameter bindings."""
function run_document_query(model::Model, query_id::AbstractString; bindings=Dict())
    conn = model.connection
    require_capability(conn, CAPABILITY_DOCUMENT_QUERY)
    request = Dict{String,Any}("modelHash" => model.hash, "queryId" => String(query_id),
        "bindings" => build_document_bindings(bindings))
    answer = _translate(; not_found=SymbolNotFoundError,
        capabilities=(CAPABILITY_DOCUMENT_QUERY,), connection=conn) do
        call(conn, "RunDocumentQuery", request)
    end
    DocumentQueryResult(String[String(get(c, "name", "")) for c in get(answer, "columns", Any[])],
        DocumentRow[_document_row(row) for row in get(answer, "rows", Any[])])
end

"""Render a document as Markdown or HTML."""
function render_document(model::Model, document_id::AbstractString; form="markdown")
    form in ("markdown", "html") ||
        throw(ArgumentError("form must be 'markdown' or 'html'"))
    conn = model.connection
    caps = form == "html" ?
        (CAPABILITY_RENDER_DOCUMENT, CAPABILITY_RENDER_DOCUMENT_HTML) :
        (CAPABILITY_RENDER_DOCUMENT,)
    foreach(c -> require_capability(conn, c), caps)
    answer = _translate(; not_found=SymbolNotFoundError, capabilities=caps,
        connection=conn) do
        call(conn, "RenderDocument", Dict{String,Any}("modelHash" => model.hash,
            "documentId" => String(document_id), "form" => form == "markdown" ? "" : form))
    end
    return form == "html" ? String(get(answer, "html", "")) : String(get(answer, "markdown", ""))
end
