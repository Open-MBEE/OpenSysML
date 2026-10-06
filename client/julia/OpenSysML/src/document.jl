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

"""The states, context, simulation time, and optional trace from one machine run."""
struct StateRun
    states_visited::Vector{String}
    final_context::Dict{String,Any}
    final_time::Float64
    trace::Vector{DocumentEvent}
    trace_dropped::Int
end

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

struct RenderSpan
    file::String
    start_line::Int
    start_col::Int
    end_line::Int
    end_col::Int
end

struct RenderPort
    id::String
    name::String
    type::String
    direction::String
end

struct RenderGeometry
    x::Float64
    y::Float64
    width::Float64
    height::Float64
    has_size::Bool
    collapsed::Bool
end

struct RenderStyle
    fill::String
    line::String
    text::String
    font::String
    font_size::Float64
    bold::Bool
    italic::Bool
end

struct RenderPoint
    x::Float64
    y::Float64
end

struct RenderNode
    id::String
    kind::String
    name::String
    name_synthesized::Bool
    type::String
    detail::String
    text::String
    stand_in::Bool
    parent::String
    ports::Vector{RenderPort}
    origin::Union{Nothing,RenderSpan}
    geometry::Union{Nothing,RenderGeometry}
    style::Union{Nothing,RenderStyle}
end

struct RenderEdge
    from::String
    to::String
    from_port::String
    to_port::String
    label::String
    name::String
    kind::String
    origin::Union{Nothing,RenderSpan}
    route::Vector{RenderPoint}
    style::Union{Nothing,RenderStyle}
end

struct RenderCanvas
    unit::String
    width::Float64
    height::Float64
    has_size::Bool
end

struct RenderRow
    cells::Vector{String}
    origin::Union{Nothing,RenderSpan}
end

struct RenderNote
    text::String
    anchor::String
    edge_from::String
    edge_to::String
    x::Float64
    y::Float64
    width::Float64
    height::Float64
    has_size::Bool
    origin::Union{Nothing,RenderSpan}
end

struct RenderedView
    view::String
    kind::String
    stated::String
    nodes::Vector{RenderNode}
    edges::Vector{RenderEdge}
    columns::Vector{String}
    rows::Vector{RenderRow}
    canvas::Union{Nothing,RenderCanvas}
    notes::Vector{RenderNote}
    notices::Vector{String}
end

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
    haskey(raw, "bigIntValue") && return parse_big_integer(string(raw["bigIntValue"]))
    haskey(raw, "rationalValue") && return parse_rational(raw["rationalValue"])
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
                Dict{String,Any}(integer_arm(value, "intValue", "bigIntValue"))
            elseif value isa Rational
                Dict{String,Any}(rational_arm(value, "rationalValue"))
            elseif value isa AbstractFloat
                Dict{String,Any}("realValue" => Float64(value))
            elseif value isa Quantity
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

# Whether a wire binding sends an exact Rational, which needs rational_values.
_binding_holds_rational(binding) = any(binding["values"]) do value
    haskey(value, "rationalValue") ||
        (haskey(value, "quantity") && haskey(value["quantity"], "rationalMagnitude"))
end

# Whether a wire binding sends an Integer beyond int64, which needs big_int_values.
_binding_holds_big_int(binding) = any(binding["values"]) do value
    haskey(value, "bigIntValue") ||
        (haskey(value, "quantity") && haskey(value["quantity"], "bigIntMagnitude"))
end

"""Run a document query with optional parameter bindings."""
function run_document_query(model::Model, query_id::AbstractString; bindings=Dict())
    conn = model.connection
    require_capability(conn, CAPABILITY_DOCUMENT_QUERY)
    wire = build_document_bindings(bindings)
    any(_binding_holds_big_int, wire) && require_capability(conn, CAPABILITY_BIG_INT_VALUES)
    has_capability(conn, CAPABILITY_RATIONAL_VALUES) || rationals_as_reals!(wire)
    any(_binding_holds_rational, wire) && require_capability(conn, CAPABILITY_RATIONAL_VALUES)
    request = Dict{String,Any}("modelHash" => model.hash, "queryId" => String(query_id),
        "bindings" => wire)
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

_render_string(raw, field) = String(get(raw, field, ""))
_render_float(raw, field) = Float64(get(raw, field, 0))
_render_bool(raw, field) = Bool(get(raw, field, false))

function _render_span(raw)
    raw isa AbstractDict || return nothing
    RenderSpan(_render_string(raw, "file"), Int(get(raw, "startLine", 0)),
        Int(get(raw, "startCol", 0)), Int(get(raw, "endLine", 0)), Int(get(raw, "endCol", 0)))
end

function _render_geometry(raw)
    raw isa AbstractDict || return nothing
    RenderGeometry(_render_float(raw, "x"), _render_float(raw, "y"),
        _render_float(raw, "width"), _render_float(raw, "height"),
        _render_bool(raw, "hasSize"), _render_bool(raw, "collapsed"))
end

function _render_style(raw)
    raw isa AbstractDict || return nothing
    RenderStyle(_render_string(raw, "fill"), _render_string(raw, "line"),
        _render_string(raw, "text"), _render_string(raw, "font"),
        _render_float(raw, "fontSize"), _render_bool(raw, "bold"), _render_bool(raw, "italic"))
end

function rendered_view_result(raw)
    nodes = RenderNode[]
    for node in get(raw, "nodes", Any[])
        ports = RenderPort[RenderPort(_render_string(port, "id"), _render_string(port, "name"),
            _render_string(port, "type"), _render_string(port, "direction"))
            for port in get(node, "ports", Any[])]
        push!(nodes, RenderNode(_render_string(node, "id"), _render_string(node, "kind"),
            _render_string(node, "name"), _render_bool(node, "nameSynthesized"),
            _render_string(node, "type"), _render_string(node, "detail"),
            _render_string(node, "text"), _render_bool(node, "standIn"),
            _render_string(node, "parent"), ports, _render_span(get(node, "origin", nothing)),
            _render_geometry(get(node, "geometry", nothing)),
            _render_style(get(node, "style", nothing))))
    end
    edges = RenderEdge[RenderEdge(_render_string(edge, "from"), _render_string(edge, "to"),
        _render_string(edge, "fromPort"), _render_string(edge, "toPort"),
        _render_string(edge, "label"), _render_string(edge, "name"),
        _render_string(edge, "kind"), _render_span(get(edge, "origin", nothing)),
        RenderPoint[RenderPoint(_render_float(point, "x"), _render_float(point, "y"))
            for point in get(edge, "route", Any[])],
        _render_style(get(edge, "style", nothing))) for edge in get(raw, "edges", Any[])]
    rows = RenderRow[RenderRow(String[String(cell) for cell in get(row, "cells", Any[])],
        _render_span(get(row, "origin", nothing))) for row in get(raw, "rows", Any[])]
    notes = RenderNote[RenderNote(_render_string(note, "text"), _render_string(note, "anchor"),
        _render_string(note, "edgeFrom"), _render_string(note, "edgeTo"),
        _render_float(note, "x"), _render_float(note, "y"), _render_float(note, "width"),
        _render_float(note, "height"), _render_bool(note, "hasSize"),
        _render_span(get(note, "origin", nothing))) for note in get(raw, "notes", Any[])]
    canvas_raw = get(raw, "canvas", nothing)
    canvas = canvas_raw isa AbstractDict ?
        RenderCanvas(_render_string(canvas_raw, "unit"), _render_float(canvas_raw, "width"),
            _render_float(canvas_raw, "height"), _render_bool(canvas_raw, "hasSize")) : nothing
    RenderedView(_render_string(raw, "view"), _render_string(raw, "kind"),
        _render_string(raw, "stated"), nodes, edges,
        String[String(c) for c in get(raw, "columns", Any[])], rows, canvas, notes,
        String[String(n) for n in get(raw, "notices", Any[])])
end

"""Render a named view with minimal or full ports."""
function render_view(model::Model, view_name::AbstractString; ports="minimal")
    ports in ("minimal", "full") || throw(ArgumentError("ports must be 'minimal' or 'full'"))
    conn = model.connection
    require_capability(conn, CAPABILITY_RENDER_VIEW)
    answer = _translate(; not_found=(message, _) ->
        SymbolNotFoundError(view_name; service_message=message),
        capabilities=(CAPABILITY_RENDER_VIEW,), connection=conn) do
        call(conn, "RenderView", Dict{String,Any}("modelHash" => model.hash,
            "view" => String(view_name), "ports" => ports == "minimal" ? "" : ports))
    end
    rendered_view_result(answer)
end
