struct QueryElement
    id::String
    type::String
    properties::Dict{String,String}
end

Base.get(element::QueryElement, name, default=nothing) = get(element.properties, name, default)
Base.show(io::IO, element::QueryElement) = print(io, "$(element.id) ($(element.type))")
as_dict(element::QueryElement) = Dict{String,Any}("@id" => element.id, "@type" => element.type,
                                                  element.properties...)

function _reject_unknown(payload, known, what)
    unknown = sort!(String[String(k) for k in keys(payload) if !(String(k) in known)])
    isempty(unknown) && return nothing
    label = what == "Query" ? "a query" : "a $(what)"
    remedy = what == "Query" ? "; the standard's query is scope, select and where" : ""
    throw(QueryError("$(label) has no $(join(unknown, ", "))$(remedy)"))
end

function _sequence(field, value)
    value === nothing && return Any[]
    value isa AbstractString && return Any[value]
    value isa AbstractDict && return Any[value]
    value isa AbstractVector || value isa Tuple ||
        throw(QueryError("$(field) is a list, not $(typeof(value))"))
    Any[value...]
end

function _scope_id(entry)
    entry isa AbstractString && return String(entry)
    entry isa AbstractDict && get(entry, "@id", nothing) isa AbstractString &&
        return String(entry["@id"])
    throw(QueryError("a scope entry is an element's qualified name or a {'@id': ...} reference, not $(repr(entry))"))
end

function _query_value(value)
    value isa Bool && return value ? "true" : "false"
    value isa Union{AbstractString,Integer,AbstractFloat} && return string(value)
    throw(QueryError("cannot compare against $(repr(value))"))
end

function _query_values(value)
    value === nothing && return String[]
    values = value isa AbstractVector || value isa Tuple ? value : (value,)
    return String[_query_value(v) for v in values]
end

function _constraint(payload)
    payload isa AbstractDict ||
        throw(QueryError("a constraint is an object, not $(typeof(payload))"))
    declared = get(payload, "@type", nothing)
    if declared === nothing
        declared = haskey(payload, "constraint") ? "CompositeConstraint" : "PrimitiveConstraint"
    end
    if declared == "PrimitiveConstraint"
        _reject_unknown(payload, ("@type", "@id", "inverse", "property", "operator", "value"),
                        "PrimitiveConstraint")
        operator = get(payload, "operator", nothing)
        operators = Dict("=" => "PRIMITIVE_OPERATOR_EQUAL",
                         ">" => "PRIMITIVE_OPERATOR_GREATER",
                         "<" => "PRIMITIVE_OPERATOR_LESS")
        haskey(operators, operator) ||
            throw(QueryError("unknown primitive operator $(repr(operator)); expected one of <, =, >"))
        property = get(payload, "property", nothing)
        property isa AbstractString && !isempty(property) ||
            throw(QueryError("a primitive constraint names one property, not $(repr(property))"))
        return Dict{String,Any}("primitive" => Dict(
            "inverse" => Bool(get(payload, "inverse", false)),
            "property" => String(property), "operator" => operators[operator],
            "value" => _query_values(get(payload, "value", nothing))))
    elseif declared == "CompositeConstraint"
        _reject_unknown(payload, ("@type", "@id", "constraint", "operator"),
                        "CompositeConstraint")
        operator = get(payload, "operator", nothing)
        operators = Dict("and" => "COMPOSITE_OPERATOR_AND", "or" => "COMPOSITE_OPERATOR_OR")
        haskey(operators, operator) ||
            throw(QueryError("unknown composite operator $(repr(operator)); expected one of and, or"))
        nested = get(payload, "constraint", nothing)
        (nested isa AbstractVector || nested isa Tuple) && !isempty(nested) ||
            throw(QueryError("a composite constraint combines a non-empty list of constraints, not $(repr(nested))"))
        return Dict{String,Any}("composite" => Dict(
            "operator" => operators[operator],
            "constraint" => Any[_constraint(entry) for entry in nested]))
    end
    throw(QueryError("unknown constraint type $(repr(declared)); the standard's constraints are PrimitiveConstraint and CompositeConstraint"))
end

function _build_query(payload; scope=nothing, select=nothing, var"where"=nothing)
    condition = var"where"
    if payload !== nothing
        (scope === nothing && select === nothing && condition === nothing) ||
            throw(QueryError("pass a query payload or scope/select/where keywords, not both"))
        payload isa AbstractDict || throw(QueryError("a query is an object, not $(typeof(payload))"))
        declared = get(payload, "@type", "Query")
        declared == "Query" || throw(QueryError("expected a 'Query' payload, got $(repr(declared))"))
        _reject_unknown(payload, ("@type", "@id", "owningProject", "scope", "select", "where"), "Query")
        scope = get(payload, "scope", nothing)
        select = get(payload, "select", nothing)
        condition = get(payload, "where", nothing)
    end
    scopes = String[_scope_id(entry) for entry in _sequence("scope", scope)]
    selected = String[]
    for entry in _sequence("select", select)
        entry isa AbstractString || throw(QueryError("a selected property is a name, not $(repr(entry))"))
        push!(selected, String(entry))
    end
    query = Dict{String,Any}("scope" => scopes, "select" => selected)
    condition === nothing || (query["where"] = _constraint(condition))
    query
end

"""Build a standard Query payload from a payload dictionary or query fields."""
build_query(; payload=nothing, scope=nothing, select=nothing, var"where"=nothing) =
    _build_query(payload; scope=scope, select=select, var"where"=var"where")
build_query(payload; scope=nothing, select=nothing, var"where"=nothing) =
    _build_query(payload; scope=scope, select=select, var"where"=var"where")

"""Run a typed standard query and return its elements."""
function query(model::Model; payload=nothing, scope=nothing, select=nothing, var"where"=nothing)
    require_capability(model.connection, CAPABILITY_QUERY)
    request_query = build_query(; payload=payload, scope=scope, select=select, var"where"=var"where")
    answer = _translate(; capabilities=(CAPABILITY_QUERY,), connection=model.connection) do
        call(model.connection, "Query", Dict{String,Any}("modelHash" => model.hash,
                                                          "query" => request_query))
    end
    _check_error(answer, "Query")
    return QueryElement[QueryElement(String(e["id"]), String(e["type"]),
                                     Dict{String,String}(String(k) => String(v)
                                                         for (k, v) in get(e, "properties", Dict())))
                        for e in get(answer, "elements", Any[])]
end
