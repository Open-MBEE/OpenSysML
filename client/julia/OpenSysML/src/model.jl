"""A path-backed or inline document passed to `parse_sources`."""
struct SourceDocument
    path::Union{Nothing,String}
    content::Union{Nothing,String}
    name::String
    language::Union{Nothing,String}
    function SourceDocument(path, content, name, language)
        (path === nothing) != (content === nothing) ||
            throw(ArgumentError("a source document is either a file path or inline content"))
        if path !== nothing
            isempty(path) && throw(ArgumentError("a file document needs a path"))
            isempty(name) || throw(ArgumentError("a file document is named by its path"))
            language === nothing || throw(ArgumentError("language applies to inline content"))
        else
            isempty(name) && throw(ArgumentError("inline content needs a name"))
            language === nothing || language in ("sysml", "kerml") ||
                throw(ArgumentError("language must be 'sysml' or 'kerml'"))
        end
        new(path, content, name, language)
    end
end

SourceDocument(path::AbstractString) = SourceDocument(String(path), nothing, "", nothing)
SourceDocument(name::AbstractString, content::AbstractString; language=nothing) =
    SourceDocument(nothing, String(content), String(name),
                   language === nothing ? nothing : String(language))
SourceDocument(; path=nothing, content=nothing, name="", language=nothing) =
    SourceDocument(path === nothing ? nothing : String(path),
                   content === nothing ? nothing : String(content), String(name),
                   language === nothing ? nothing : String(language))
document_name(doc::SourceDocument) = doc.path === nothing ? doc.name : doc.path

function source_documents(documents)
    (documents isa AbstractString || documents isa SourceDocument ||
     documents isa Pair || documents isa Tuple) &&
        throw(ArgumentError("parse_sources takes a sequence of documents; write one document as [document]"))
    result = SourceDocument[]
    for (position, document) in enumerate(documents)
        if document isa SourceDocument
            push!(result, document)
        elseif document isa AbstractString
            push!(result, SourceDocument(document))
        elseif document isa Pair && first(document) isa AbstractString && last(document) isa AbstractString
            push!(result, SourceDocument(first(document), last(document)))
        elseif document isa Tuple && length(document) == 2 &&
               all(part -> part isa AbstractString, document)
            push!(result, SourceDocument(document[1], document[2]))
        else
            throw(ArgumentError("document $(position) is not a path, (name, content) pair, or SourceDocument"))
        end
    end
    isempty(result) && throw(ArgumentError("parse_sources needs at least one document"))
    names = document_name.(result)
    length(unique(names)) == length(names) ||
        throw(ArgumentError("two documents are named $(repr(first(name for name in names if count(==(name), names) > 1)))"))
    result
end

struct Model
    connection::Connection
    hash::String
    diagnostics::Vector{Diagnostic}
    documents::Vector{String}
    roots::Vector{Any}
    source_path::Union{Nothing,String}
end
Model(conn::Connection, hash::AbstractString, diags::Vector{Diagnostic}) =
    Model(conn, String(hash), diags, String[], Any[], nothing)
Model(conn::Connection, hash, diagnostics, documents, roots, source_path=nothing) =
    Model(conn, String(hash), Diagnostic[d for d in diagnostics], String[String(d) for d in documents],
          Any[r for r in roots], source_path === nothing ? nothing : String(source_path))

mutable struct Instance
    id::Int64
    type_symbol_id::String
    feature_values::Dict{String,Any}
    raw_features::Dict{String,Any}
    graph::Dict{Int64,Instance}
end
function Instance(id::Integer, type_symbol_id, feature_values::AbstractDict)
    inst = Instance(Int64(id), String(type_symbol_id), Dict{String,Any}(feature_values),
                    Dict{String,Any}(), Dict{Int64,Instance}())
    inst.graph[inst.id] = inst
    inst
end

function Base.getproperty(inst::Instance, name::Symbol)
    name in (:id, :type_symbol_id, :raw_features, :graph) && return getfield(inst, name)
    name in (:feature_values, :features) && return features(inst)
    haskey(getfield(inst, :feature_values), String(name)) ||
        throw(ArgumentError("instance has no attribute or feature $(repr(String(name)))"))
    value = getfield(inst, :feature_values)[String(name)]
    value isa FeatureValueError && throw(value)
    value
end

function _resolve_instance_value(value, graph)
    value isa InstanceRef && return get(graph, value.id, value)
    value isa VectorValue && return VectorValue(Union{Int64,Float64}[_resolve_instance_value(v, graph) for v in value.components])
    value isa ArrayValue && return ArrayValue(value.dimensions, Any[_resolve_instance_value(v, graph) for v in value.elements])
    value isa AbstractVector && return Any[_resolve_instance_value(v, graph) for v in value]
    value isa Set && return Set(_resolve_instance_value(v, graph) for v in value)
    value
end

function Base.get(inst::Instance, name::AbstractString, default=nothing)
    raw = getfield(inst, :feature_values)
    haskey(raw, name) || return default
    value = raw[String(name)]
    value isa FeatureValueError && throw(value)
    _resolve_instance_value(value, getfield(inst, :graph))
end
function Base.getindex(inst::Instance, name::AbstractString)
    haskey(getfield(inst, :feature_values), name) || throw(KeyError(name))
    get(inst, name)
end
Base.haskey(inst::Instance, name::AbstractString) = haskey(getfield(inst, :feature_values), String(name))
Base.in(name::AbstractString, inst::Instance) = haskey(inst, name)
"""Return raw service metadata for an instance feature."""
get_feature(inst::Instance, name::AbstractString) = get(inst.raw_features, String(name), nothing)
"""Return decoded feature values, resolving references to instances in the graph."""
function features(inst::Instance)
    Dict{String,Any}(name => (value isa FeatureValueError ? value :
        _resolve_instance_value(value, getfield(inst, :graph)))
        for (name, value) in getfield(inst, :feature_values))
end

function _check_error(answer, method)
    msg = String(get(answer, "error", ""))
    isempty(msg) && return answer
    diags = Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])]
    if method in ("ParseFile", "ParseSources")
        throw(ModelError(msg, diags))
    end
    return _raise_answer_error(answer; diagnostics=diags)
end

function _model_from_answer(conn, answer; documents=String[], source_path=nothing, strict=false)
    raw_roots = get(answer, "roots", nothing)
    roots = raw_roots === nothing ? (haskey(answer, "root") ? Any[answer["root"]] : Any[]) : Any[r for r in raw_roots]
    model = Model(conn, answer["modelHash"],
                  Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])],
                  documents, roots, source_path)
    strict && raise_for_errors(model)
    return model
end

"""Parse a source file through the service."""
function parse_file(conn::Connection, path::AbstractString; language::AbstractString="",
                    strict::Bool=false, strict_conformance::Bool=false)
    request = Dict{String,Any}("filePath" => String(path))
    needed = String[]
    if !isempty(language)
        language in ("sysml", "kerml") ||
            throw(ArgumentError("language must be 'sysml' or 'kerml'"))
        request["language"] = String(language)
    end
    if strict_conformance
        require_capability(conn, CAPABILITY_STRICT_CONFORMANCE)
        request["strictConformance"] = true
        push!(needed, CAPABILITY_STRICT_CONFORMANCE)
    end
    answer = _translate(; not_found=ModelFileNotFoundError, connection=conn,
                        capabilities=Tuple(needed)) do
        call(conn, "ParseFile", request)
    end
    _check_error(answer, "ParseFile")
    _model_from_answer(conn, answer; documents=[String(path)], source_path=String(path), strict=strict)
end

"""Parse one inline SysML or KerML source document."""
function parse_source(conn::Connection, content::AbstractString; name::AbstractString="inline.sysml",
                      language::AbstractString="", strict::Bool=false,
                      strict_conformance::Bool=false)
    !isempty(language) && !(language in ("sysml", "kerml")) &&
        throw(ArgumentError("language must be 'sysml' or 'kerml'"))
    needed = String[]
    if !isempty(language)
        require_capability(conn, CAPABILITY_INLINE_LANGUAGE)
        push!(needed, CAPABILITY_INLINE_LANGUAGE)
    end
    if strict_conformance
        require_capability(conn, CAPABILITY_STRICT_CONFORMANCE)
        push!(needed, CAPABILITY_STRICT_CONFORMANCE)
    end
    request = Dict{String,Any}("content" => String(content))
    isempty(language) || (request["language"] = String(language))
    strict_conformance && (request["strictConformance"] = true)
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "ParseFile", request)
    end
    _check_error(answer, "ParseFile")
    _model_from_answer(conn, answer; documents=[String(name)], strict=strict)
end

function _parse_sources(conn::Connection, docs::Vector{SourceDocument};
                        strict::Bool=false, strict_conformance::Bool=false)
    require_capability(conn, CAPABILITY_PARSE_SOURCES)
    any(doc -> doc.language !== nothing, docs) &&
        require_capability(conn, CAPABILITY_INLINE_LANGUAGE)
    if strict_conformance
        require_capability(conn, CAPABILITY_STRICT_CONFORMANCE)
    end
    request_docs = Any[]
    for doc in docs
        entry = Dict{String,Any}()
        if doc.path !== nothing
            entry["filePath"] = doc.path
        else
            entry["name"] = doc.name
            entry["content"] = doc.content
            doc.language === nothing || (entry["language"] = doc.language)
        end
        push!(request_docs, entry)
    end
    request = Dict{String,Any}("documents" => request_docs)
    strict_conformance && (request["strictConformance"] = true)
    needed = any(doc -> doc.language !== nothing, docs) ?
             (CAPABILITY_PARSE_SOURCES, CAPABILITY_INLINE_LANGUAGE) :
             (CAPABILITY_PARSE_SOURCES,)
    strict_conformance && (needed = (needed..., CAPABILITY_STRICT_CONFORMANCE))
    answer = _translate(; not_found=ModelFileNotFoundError, capabilities=needed,
                        connection=conn) do
        call(conn, "ParseSources", request)
    end
    _check_error(answer, "ParseSources")
    return _model_from_answer(conn, answer; documents=document_name.(docs), strict=strict)
end

"""Parse multiple file-backed or inline source documents together."""
function parse_sources(conn::Connection, documents; language::AbstractString="",
                       strict::Bool=false, strict_conformance::Bool=false)
    docs = source_documents(documents)
    if !isempty(language)
        docs = SourceDocument[doc.path === nothing && doc.language === nothing ?
                              SourceDocument(doc.name, doc.content; language=language) : doc
                              for doc in docs]
    end
    return _parse_sources(conn, docs; strict=strict, strict_conformance=strict_conformance)
end

"""Return the diagnostics reported while parsing `model`."""
diagnostics(model::Model) = model.diagnostics
"""Return only error-severity diagnostics from `model`."""
errors(model::Model) = Diagnostic[d for d in model.diagnostics if lowercase(d.severity) == "error"]
"""Return whether `model` has no error-severity diagnostics."""
isok(model::Model) = isempty(errors(model))
"""Return the first root symbol, or `nothing` when the model has none."""
root(model::Model) = getproperty(model, :root)
"""Return the root symbols declared by `model`."""
roots(model::Model) = getproperty(model, :roots)
"""Return the source document names associated with `model`."""
documents(model::Model) = model.documents

"""Raise `ModelError` if the model has error diagnostics; otherwise return it."""
function raise_for_errors(model::Model)
    es = errors(model)
    isempty(es) && return model
    location = model.source_path === nothing ? "the model" : model.source_path
    summaries = ["$(d.message)" for d in es[1:min(end, 3)]]
    length(es) > 3 && push!(summaries, "... and $(length(es) - 3) more")
    throw(ModelError("$(location) has $(length(es)) error(s): $(join(summaries, "; "))", es; model=model))
end

"""Return the raw symbol dictionary for a qualified identifier."""
function symbol(model::Model, id::AbstractString)
    answer = call(model.connection, "GetSymbol",
                  Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(id)))
    if !isempty(String(get(answer, "error", "")))
        return nothing
    end
    return get(answer, "symbol", answer)
end

"""Evaluate an expression, optionally in a symbol context or subject instance."""
function evaluate(model::Model, expression::AbstractString; context=nothing, subject=nothing)
    subject === nothing || require_capability(model.connection, CAPABILITY_EVALUATE_SUBJECT)
    request = Dict{String,Any}("modelHash" => model.hash, "expression" => String(expression))
    context === nothing || (request["contextSymbolId"] = String(context))
    subject === nothing || (request["subjectSymbolId"] = String(subject))
    answer = _check_error(call(model.connection, "Evaluate", request), "Evaluate")
    value = _decode_or_unsupported(get(answer, "result", nothing))
    value isa UnsupportedValueError && throw(value)
    return value
end

function _decode_instance(inst)
    features = Dict{String,Any}()
    for (name, fv) in get(inst, "featureValues", Dict{String,Any}())
        if !isempty(String(get(fv, "error", "")))
            features[name] = FeatureValueError(name, String(fv["error"]))
        elseif haskey(fv, "value")
            features[name] = _feature_value(String(name), fv["value"])
        elseif haskey(fv, "values")
            values = Any[_feature_value(String(name), v) for v in fv["values"]]
            unsupported = findfirst(value -> value isa FeatureValueError, values)
            features[name] = unsupported === nothing ? values : values[unsupported]
        elseif get(fv, "materialized", false)
            features[name] = Any[]
        else
            features[name] = FeatureValueError(String(name), "feature value is not materialized")
        end
    end
    result = Instance(parse(Int64, string(inst["id"])), get(inst, "typeSymbolId", ""), features)
    result.raw_features = Dict{String,Any}(String(k) => v for (k, v) in
                                           get(inst, "featureValues", Dict{String,Any}()))
    result
end

function _feature_value(name, raw)
    decoded = _decode_or_unsupported(raw)
    decoded isa UnsupportedValueError &&
        return FeatureValueError(name, decoded.message)
    decoded
end

function _decode_instances(raw_instances)
    instances = Instance[_decode_instance(raw) for raw in raw_instances]
    graph = Dict{Int64,Instance}(inst.id => inst for inst in instances)
    for inst in instances
        empty!(inst.graph)
        merge!(inst.graph, graph)
    end
    return instances
end

"""Instantiate a model type and decode its feature values."""
function instantiate(model::Model, type_id::AbstractString)
    require_capability(model.connection, CAPABILITY_FEATURE_VALUES)
    answer = _check_error(call(model.connection, "Instantiate",
                               Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(type_id))),
                          "Instantiate")
    inst = get(answer, "instance", nothing)
    inst === nothing && throw(ExecutionFailure("Instantiate carried no instance", Diagnostic[]))
    raw_instances = get(answer, "instances", Any[inst])
    instances = _decode_instances(raw_instances)
    index = findfirst(i -> i.id == parse(Int64, string(inst["id"])), instances)
    return index === nothing ? first(instances) : instances[index]
end

function _encoded_inputs(model::Model, inputs)
    encoded = Dict{String,Any}()
    for (key, value) in pairs(inputs)
        for capability in sort!(collect(value_capabilities(value)))
            require_capability(model.connection, capability)
        end
        encoded[String(key)] = encode_value(value)
    end
    return encoded
end

function _check_schedule(schedule)
    (schedule == "explore" || startswith(schedule, "explore:")) &&
        throw(ArgumentError("schedule=\"explore\" uses explore_action or explore_state; call those exploration APIs instead"))
end

"""Execute an action with encoded inputs and optional scheduling or performer."""
function execute_action(model::Model, action_id::AbstractString; inputs=Dict(),
                        schedule::AbstractString="", performer=nothing)
    _check_schedule(schedule)
    encoded = _encoded_inputs(model, inputs)
    _schedule_preflight(model.connection, schedule)
    performer !== nothing && require_capability(model.connection, CAPABILITY_PERFORMER)
    needed = String[_schedule_capabilities(schedule)...]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    request = Dict{String,Any}("modelHash" => model.hash, "actionSymbolId" => String(action_id),
                               "inputs" => encoded)
    if !isempty(schedule)
        request["schedule"] = String(schedule)
    end
    if performer !== nothing
        request["performerSymbolId"] = String(performer)
    end
    answer = _translate(; capabilities=Tuple(unique(needed)), connection=model.connection) do
        call(model.connection, "ExecuteAction", request)
    end
    _check_error(answer, "ExecuteAction")
    return decode_values(answer)
end

"""Execute a state machine with optional events, scheduling, or performer."""
function execute_state(model::Model, state_id::AbstractString; events=Any[],
                       schedule::AbstractString="", performer=nothing)
    _check_schedule(schedule)
    _schedule_preflight(model.connection, schedule)
    performer !== nothing && require_capability(model.connection, CAPABILITY_PERFORMER)
    needed = String[_schedule_capabilities(schedule)...]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    request = Dict{String,Any}("modelHash" => model.hash, "stateMachineSymbolId" => String(state_id),
                               "events" => Any[String(e) for e in events])
    if !isempty(schedule)
        request["schedule"] = String(schedule)
    end
    if performer !== nothing
        request["performerSymbolId"] = String(performer)
    end
    answer = _translate(; capabilities=Tuple(unique(needed)), connection=model.connection) do
        call(model.connection, "ExecuteState", request)
    end
    _check_error(answer, "ExecuteState")
    return decode_values(answer)
end

"""Run a legacy OSLC query and return its decoded response dictionary."""
function query(model::Model, query_text::AbstractString)
    require_capability(model.connection, CAPABILITY_QUERY)
    require_capability(model.connection, CAPABILITY_OSLC_QUERY)
    answer = _translate(; capabilities=(CAPABILITY_QUERY, CAPABILITY_OSLC_QUERY),
                        connection=model.connection) do
        call(model.connection, "Query",
             Dict{String,Any}("modelHash" => model.hash, "oslcQuery" => String(query_text)))
    end
    _check_error(answer, "Query")
    return decode_values(answer)
end
