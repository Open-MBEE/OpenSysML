struct Model
    connection::Connection
    hash::String
    diagnostics::Vector{Diagnostic}
end

struct Instance
    id::Int64
    type_symbol_id::String
    feature_values::Dict{String,Any}
end

function _check_error(answer, method)
    msg = get(answer, "error", "")
    isempty(msg) || throw(DiagnosticError(msg, Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])]))
    return answer
end

function parse_file(conn::Connection, path::AbstractString; language::AbstractString="", strict::Bool=false)
    request = Dict{String,Any}("filePath" => String(path))
    isempty(language) || (request["language"] = String(language))
    strict && (request["strictConformance"] = true)
    answer = _check_error(call(conn, "ParseFile", request), "ParseFile")
    Model(conn, answer["modelHash"], Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])])
end

function parse_source(conn::Connection, content::AbstractString; name::AbstractString="inline.sysml", language::AbstractString="")
    return parse_sources(conn, [(name, content)]; language=language)
end

function parse_sources(conn::Connection, documents; language::AbstractString="", strict::Bool=false)
    docs = Any[]
    for (name, content) in documents
        doc = Dict{String,Any}("name" => String(name), "content" => String(content))
        isempty(language) || (doc["language"] = String(language))
        push!(docs, doc)
    end
    request = Dict{String,Any}("documents" => docs)
    strict && (request["strictConformance"] = true)
    answer = _check_error(call(conn, "ParseSources", request), "ParseSources")
    Model(conn, answer["modelHash"], Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])])
end

diagnostics(model::Model) = model.diagnostics

function symbol(model::Model, id::AbstractString)
    answer = _check_error(call(model.connection, "GetSymbol",
                               Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(id))), "GetSymbol")
    return get(answer, "symbol", answer)
end

function evaluate(model::Model, expression::AbstractString; context=nothing, subject=nothing)
    request = Dict{String,Any}("modelHash" => model.hash, "expression" => String(expression))
    context === nothing || (request["contextSymbolId"] = String(context))
    subject === nothing || (request["subjectSymbolId"] = String(subject))
    answer = _check_error(call(model.connection, "Evaluate", request), "Evaluate")
    return decode_value(get(answer, "result", nothing))
end

function _decode_instance(inst)
    features = Dict{String,Any}()
    for (name, fv) in get(inst, "featureValues", Dict{String,Any}())
        if haskey(fv, "value")
            features[name] = decode_value(fv["value"])
        elseif haskey(fv, "values")
            features[name] = Any[decode_value(v) for v in fv["values"]]
        elseif haskey(fv, "error")
            features[name] = fv["error"]
        end
    end
    Instance(parse(Int64, inst["id"]), get(inst, "typeSymbolId", ""), features)
end

function instantiate(model::Model, type_id::AbstractString)
    answer = _check_error(call(model.connection, "Instantiate",
                               Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(type_id))), "Instantiate")
    inst = get(answer, "instance", nothing)
    inst === nothing && throw(DiagnosticError("Instantiate carried no instance", Diagnostic[]))
    return _decode_instance(inst)
end

function _encoded_inputs(inputs)
    return Dict{String,Any}(String(k) => encode_value(v) for (k, v) in pairs(inputs))
end

function execute_action(model::Model, action_id::AbstractString; inputs=Dict(), schedule::AbstractString="")
    request = Dict{String,Any}("modelHash" => model.hash, "actionSymbolId" => String(action_id),
                               "inputs" => _encoded_inputs(inputs))
    isempty(schedule) || (request["schedule"] = String(schedule))
    answer = _check_error(call(model.connection, "ExecuteAction", request), "ExecuteAction")
    return decode_values(answer)
end

function execute_state(model::Model, state_id::AbstractString; events=Any[], schedule::AbstractString="")
    request = Dict{String,Any}("modelHash" => model.hash, "stateMachineSymbolId" => String(state_id),
                               "events" => Any[String(e) for e in events])
    isempty(schedule) || (request["schedule"] = String(schedule))
    answer = _check_error(call(model.connection, "ExecuteState", request), "ExecuteState")
    return decode_values(answer)
end

function query(model::Model, query_text::AbstractString)
    answer = _check_error(call(model.connection, "Query",
                               Dict{String,Any}("modelHash" => model.hash, "oslcQuery" => String(query_text))), "Query")
    return decode_values(answer)
end
