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
    remedy = what == "Query" ? "; the standard's query is scope, select, where and orderBy" : ""
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

# The compared values in the shape the operator compares against: the ordered
# comparisons one value, `in` a list of them, `instanceOf` metaclass names.
function _operand(operator, property, value)
    islist = value isa AbstractVector || value isa Tuple
    if operator in (">", "<", ">=", "<=")
        islist && length(value) != 1 &&
            throw(QueryError("$(operator) on $(repr(property)) compares against one value, not a list of $(length(value))"))
        value === nothing && throw(QueryError("$(operator) on $(repr(property)) has no value to compare against"))
        return _query_values(value)
    elseif operator == "in"
        islist || throw(QueryError("in on $(repr(property)) compares against a list of values, not $(repr(value))"))
        isempty(value) && throw(QueryError("in on $(repr(property)) has no values to be a member of"))
        return String[_query_value(v) for v in value]
    elseif operator == "instanceOf"
        names = islist ? value : (value,)
        for metaclass in names
            metaclass isa AbstractString && !isempty(metaclass) ||
                throw(QueryError("instanceOf on $(repr(property)) compares against a metaclass name, not $(repr(metaclass))"))
        end
        return String[String(m) for m in names]
    end
    return _query_values(value)
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
                         "<" => "PRIMITIVE_OPERATOR_LESS",
                         ">=" => "PRIMITIVE_OPERATOR_GREATER_EQUAL",
                         "<=" => "PRIMITIVE_OPERATOR_LESS_EQUAL",
                         "in" => "PRIMITIVE_OPERATOR_IN",
                         "instanceOf" => "PRIMITIVE_OPERATOR_INSTANCE_OF")
        haskey(operators, operator) ||
            throw(QueryError("unknown primitive operator $(repr(operator)); expected one of <, <=, =, >, >=, in, instanceOf"))
        property = get(payload, "property", nothing)
        property isa AbstractString && !isempty(property) ||
            throw(QueryError("a primitive constraint names one property, not $(repr(property))"))
        return Dict{String,Any}("primitive" => Dict(
            "inverse" => Bool(get(payload, "inverse", false)),
            "property" => String(property), "operator" => operators[operator],
            "value" => _operand(operator, property, get(payload, "value", nothing))))
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
        length(nested) == 1 &&
            throw(QueryError("a composite constraint combines at least two constraints, got one; write the constraint on its own"))
        return Dict{String,Any}("composite" => Dict(
            "operator" => operators[operator],
            "constraint" => Any[_constraint(entry) for entry in nested]))
    end
    throw(QueryError("unknown constraint type $(repr(declared)); the standard's constraints are PrimitiveConstraint and CompositeConstraint"))
end

function _property_names(field, value)
    names = String[]
    for entry in _sequence(field, value)
        entry isa AbstractString || throw(QueryError("a selected property is a name, not $(repr(entry))"))
        push!(names, String(entry))
    end
    names
end

function _build_query(payload; scope=nothing, select=nothing, var"where"=nothing, order_by=nothing)
    condition = var"where"
    if payload !== nothing
        (scope === nothing && select === nothing && condition === nothing && order_by === nothing) ||
            throw(QueryError("pass a query payload or scope/select/where/order_by keywords, not both"))
        payload isa AbstractDict || throw(QueryError("a query is an object, not $(typeof(payload))"))
        declared = get(payload, "@type", "Query")
        declared == "Query" || throw(QueryError("expected a 'Query' payload, got $(repr(declared))"))
        _reject_unknown(payload, ("@type", "@id", "owningProject", "scope", "select", "where", "orderBy"), "Query")
        scope = get(payload, "scope", nothing)
        select = get(payload, "select", nothing)
        condition = get(payload, "where", nothing)
        order_by = get(payload, "orderBy", nothing)
    end
    scopes = String[_scope_id(entry) for entry in _sequence("scope", scope)]
    query = Dict{String,Any}("scope" => scopes, "select" => _property_names("select", select),
                             "orderBy" => _property_names("orderBy", order_by))
    condition === nothing || (query["where"] = _constraint(condition))
    query
end

"""Build a standard Query payload from a payload dictionary or query fields; `order_by`
names the properties the elements are sorted by, in order of precedence, each ascending."""
build_query(; payload=nothing, scope=nothing, select=nothing, var"where"=nothing, order_by=nothing) =
    _build_query(payload; scope=scope, select=select, var"where"=var"where", order_by=order_by)
build_query(payload; scope=nothing, select=nothing, var"where"=nothing, order_by=nothing) =
    _build_query(payload; scope=scope, select=select, var"where"=var"where", order_by=order_by)

"""Run a typed standard query and return its elements."""
function query(model::Model; payload=nothing, scope=nothing, select=nothing, var"where"=nothing, order_by=nothing)
    request_query = build_query(; payload=payload, scope=scope, select=select, var"where"=var"where", order_by=order_by)
    needed = isempty(get(request_query, "orderBy", ())) ? (CAPABILITY_QUERY,) :
             (CAPABILITY_QUERY, CAPABILITY_QUERY_ORDER_BY)
    foreach(capability -> require_capability(model.connection, capability), needed)
    answer = _translate(; capabilities=needed, connection=model.connection) do
        call(model.connection, "Query", Dict{String,Any}("modelHash" => model.hash,
                                                          "query" => request_query))
    end
    _check_error(answer, "Query")
    return QueryElement[QueryElement(String(e["id"]), String(e["type"]),
                                     Dict{String,String}(String(k) => String(v)
                                                         for (k, v) in get(e, "properties", Dict())))
                        for e in get(answer, "elements", Any[])]
end
