_failure_reason(answer) = uppercase(String(get(answer, "failureReason", "")))
_diagnostics(answer) = Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])]
_is_wrong_kind(answer) = occursin("WRONG_KIND", _failure_reason(answer))

function _raise_answer_error(answer; diagnostics=_diagnostics(answer), result=nothing)
    message = String(get(answer, "error", ""))
    isempty(message) && return nothing
    if _is_wrong_kind(answer)
        throw(WrongKindError(message, diagnostics))
    elseif result !== nothing
        throw(AnalysisRunError(message, diagnostics, result))
    end
    throw(ExecutionFailure(message, diagnostics))
end

function _engine_preflight(conn::Connection, engine)
    if engine !== nothing && !isempty(String(engine)) && String(engine) != "auto"
        require_capability(conn, CAPABILITY_ENGINES)
    end
    engine == "explore" && require_capability(conn, CAPABILITY_SCHEDULE_EXPLORE)
    nothing
end

function _question_preflight(conn::Connection, question)
    question !== nothing && !isempty(String(question)) && String(question) != "evaluate" &&
        require_capability(conn, CAPABILITY_VERIFICATION_QUESTIONS)
    nothing
end

function _schedule_preflight(conn::Connection, schedule)
    schedule === nothing || isempty(String(schedule)) || require_capability(conn, CAPABILITY_SCHEDULE)
    schedule !== nothing && (String(schedule) == "explore" || startswith(String(schedule), "explore:")) &&
        require_capability(conn, CAPABILITY_SCHEDULE_EXPLORE)
    nothing
end

_is_exploring(schedule) = schedule !== nothing &&
    (String(schedule) == "explore" || startswith(String(schedule), "explore:"))

function _encoded(conn::Connection, value)
    for capability in sort!(collect(value_capabilities(value)))
        require_capability(conn, capability)
    end
    encode_value(value)
end

function _encoded_arguments(conn::Connection, arguments)
    Any[_encoded(conn, argument) for argument in arguments]
end

function _reported_instances(conn::Connection, answer)
    raw = get(answer, "instances", Any[])
    isempty(raw) || require_capability(conn, CAPABILITY_FEATURE_VALUES)
    _decode_instances(raw)
end

function _single_verdict(conn::Connection, answer)
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    raw = get(answer, "verdict", nothing)
    raw isa AbstractDict || throw(ExecutionFailure("verification carried no verdict", diags))
    _is_wrong_kind(raw) && throw(WrongKindError(String(get(raw, "error", "")), diags))
    instances = _reported_instances(conn, answer)
    verifications = VerificationVerdict[VerificationVerdict(v) for v in
        get(answer, "verificationVerdicts", Any[])]
    _verdict(raw, instances, diags, _verifications_for(raw, verifications))
end

function _verifications_for(raw, verifications)
    requirement = String(get(raw, "requirementId", ""))
    isempty(requirement) ? VerificationVerdict[] :
        VerificationVerdict[v for v in verifications if v.requirement_id == requirement]
end

function _verdict(raw, instances, diagnostics, verifications=VerificationVerdict[])
    Verdict(raw; instances=instances, diagnostics=diagnostics, verifications=verifications)
end

function _verification_call(model::Model, method::String, request;
                            engine=nothing, question=nothing)
    require_capability(model.connection, CAPABILITY_VERIFICATION)
    _engine_preflight(model.connection, engine)
    _question_preflight(model.connection, question)
    needed = String[CAPABILITY_VERIFICATION]
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    engine == "explore" && push!(needed, CAPABILITY_SCHEDULE_EXPLORE)
    question !== nothing && !isempty(String(question)) && String(question) != "evaluate" &&
        push!(needed, CAPABILITY_VERIFICATION_QUESTIONS)
    answer = _translate(; capabilities=Tuple(needed), connection=model.connection) do
        call(model.connection, method, request)
    end
    return _single_verdict(model.connection, answer)
end

"""Ask whether a constraint holds, optionally for a subject or engine."""
function verify_constraint(model::Model, symbol_id::AbstractString;
                           subject=nothing, engine=nothing, question=nothing)
    _verification_call(model, "VerifyConstraint",
        Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(symbol_id),
            "subjectSymbolId" => subject === nothing ? "" : String(subject),
            "engine" => engine === nothing || engine == "auto" ? "" : String(engine),
            "question" => question === nothing || question == "evaluate" ? "" : String(question));
        engine=engine, question=question)
end

"""Ask whether a requirement is satisfied, optionally for a subject or engine."""
function verify_requirement(model::Model, symbol_id::AbstractString;
                            subject=nothing, engine=nothing, question=nothing)
    _verification_call(model, "VerifyRequirement",
        Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(symbol_id),
            "subjectSymbolId" => subject === nothing ? "" : String(subject),
            "engine" => engine === nothing || engine == "auto" ? "" : String(engine),
            "question" => question === nothing || question == "evaluate" ? "" : String(question));
        engine=engine, question=question)
end

"""Return verdicts for the model's satisfaction assertions."""
function verify_satisfaction(model::Model; symbol=nothing, symbol_id=nothing,
                             engine=nothing, question=nothing)
    symbol !== nothing && symbol_id !== nothing &&
        throw(ArgumentError("pass symbol or symbol_id, not both"))
    target = symbol === nothing ? symbol_id : symbol
    conn = model.connection
    require_capability(conn, CAPABILITY_VERIFICATION)
    _engine_preflight(conn, engine)
    _question_preflight(conn, question)
    needed = String[CAPABILITY_VERIFICATION]
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    engine == "explore" && push!(needed, CAPABILITY_SCHEDULE_EXPLORE)
    question !== nothing && !isempty(String(question)) && String(question) != "evaluate" &&
        push!(needed, CAPABILITY_VERIFICATION_QUESTIONS)
    request = Dict{String,Any}("modelHash" => model.hash,
        "symbolId" => target === nothing ? "" : String(target),
        "engine" => engine === nothing || engine == "auto" ? "" : String(engine),
        "question" => question === nothing || question == "evaluate" ? "" : String(question))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "VerifySatisfaction", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    raw_verdicts = get(answer, "verdicts", Any[])
    for raw in raw_verdicts
        _is_wrong_kind(raw) && throw(WrongKindError(String(get(raw, "error", "")), diags))
    end
    instances = _reported_instances(conn, answer)
    verifications = VerificationVerdict[VerificationVerdict(v) for v in
        get(answer, "verificationVerdicts", Any[])]
    Verdict[_verdict(v, instances, diags, _verifications_for(v, verifications))
            for v in raw_verdicts]
end

"""Return whether every selected satisfaction assertion holds."""
satisfied(model::Model; symbol=nothing, symbol_id=nothing) =
    all(v.holds for v in verify_satisfaction(model; symbol=symbol, symbol_id=symbol_id))

"""Validate the assertions associated with an instance of a part."""
function validate_instance(model::Model, symbol_id::AbstractString; engine=nothing)
    conn = model.connection
    require_capability(conn, CAPABILITY_VERIFICATION)
    _engine_preflight(conn, engine)
    needed = String[CAPABILITY_VERIFICATION]
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    request = Dict{String,Any}("modelHash" => model.hash,
        "symbolId" => String(symbol_id),
        "engine" => engine === nothing || engine == "auto" ? "" : String(engine))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "ValidateInstance", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    instances = _reported_instances(conn, answer)
    verifications = VerificationVerdict[VerificationVerdict(v) for v in
        get(answer, "verificationVerdicts", Any[])]
    verdicts = Verdict[_verdict(v, instances, diags, _verifications_for(v, verifications))
                       for v in get(answer, "verdicts", Any[])]
    summary_raw = get(answer, "summary", nothing)
    summary = summary_raw isa AbstractDict ?
        _verdict(summary_raw, instances, diags, verifications) : nothing
    Validation(verdicts, summary; instances=instances, diagnostics=diags,
                verifications=verifications, bounded=get(answer, "bounded", false))
end

function _decode_or_unsupported(raw, graph=Dict{Int64,Instance}())
    value = decode_value_or_unsupported(raw)
    value isa UnsupportedValueError && return value
    _resolve_instance_value(value, graph)
end

function _output_map(entries, graph=Dict{Int64,Instance}())
    result = Dict{String,Any}()
    for entry in entries
        result[String(get(entry, "name", ""))] =
            _decode_or_unsupported(get(entry, "value", nothing), graph)
    end
    result
end

function _case_evaluations(entries, graph=Dict{Int64,Instance}())
    CaseEvaluation[CaseEvaluation(String(get(e, "functionId", "")),
        Any[_decode_or_unsupported(v, graph) for v in get(e, "arguments", Any[])],
        haskey(e, "result") && isempty(String(get(e, "error", ""))) ?
            _decode_or_unsupported(e["result"], graph) : nothing,
        String(get(e, "error", "")), Bool(get(e, "selected", false)),
        Bool(get(e, "tied", false))) for e in entries]
end

"""Evaluate a calculation with positional arguments."""
function calc(model::Model, symbol_id::AbstractString; arguments=Any[], engine=nothing)
    conn = model.connection
    require_capability(conn, CAPABILITY_VERIFICATION)
    _engine_preflight(conn, engine)
    encoded = _encoded_arguments(conn, arguments)
    needed = String[CAPABILITY_VERIFICATION, CAPABILITY_COMPLEX_VALUES,
        CAPABILITY_STRUCTURED_VALUES, CAPABILITY_MEASUREMENT_REFS,
        CAPABILITY_FUNCTION_VALUES, CAPABILITY_SET_VALUES, CAPABILITY_TENSOR_VALUES,
        CAPABILITY_METAOBJECT_VALUES]
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    request = Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(symbol_id),
        "arguments" => encoded, "engine" => engine === nothing || engine == "auto" ? "" : String(engine))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "EvaluateCalc", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    outputs = _output_map(get(answer, "outputs", Any[]))
    value = isempty(outputs) && haskey(answer, "result") ?
        _decode_or_unsupported(answer["result"]) : nothing
    value isa UnsupportedValueError && throw(value)
    CalcResult(value, outputs; diagnostics=diags, standing=Standing(answer))
end

function _analysis_request(model::Model, symbol_id; subject=nothing, arguments=Any[],
                           named_arguments=Dict(), schedule=nothing, engine=nothing,
                           allow_exploration=false)
    conn = model.connection
    _is_exploring(schedule) && !allow_exploration &&
        throw(ArgumentError("schedule $(repr(String(schedule))) answers with every outcome, not one run's result: use explore_analysis"))
    allow_exploration && !_is_exploring(schedule) &&
        throw(ArgumentError("schedule $(repr(String(schedule))) answers one run's result, not every outcome: spell it 'explore' or 'explore:runs=<n>,depth=<d>'"))
    engine == "explore" &&
        throw(ArgumentError("engine 'explore' answers with every outcome, not one run's result: use explore_analysis"))
    require_capability(conn, CAPABILITY_VERIFICATION)
    _schedule_preflight(conn, schedule)
    _engine_preflight(conn, engine)
    args = _encoded_arguments(conn, arguments)
    named = Dict{String,Any}()
    for (name, value) in pairs(named_arguments)
        named[String(name)] = _encoded(conn, value)
    end
    needed = String[CAPABILITY_VERIFICATION, CAPABILITY_COMPLEX_VALUES,
        CAPABILITY_STRUCTURED_VALUES, CAPABILITY_MEASUREMENT_REFS, CAPABILITY_SET_VALUES,
        CAPABILITY_TENSOR_VALUES, CAPABILITY_METAOBJECT_VALUES]
    _is_exploring(schedule) && append!(needed, (CAPABILITY_SCHEDULE_EXPLORE,))
    schedule !== nothing && !isempty(String(schedule)) && push!(needed, CAPABILITY_SCHEDULE)
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    request = Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(symbol_id),
        "subjectSymbolId" => subject === nothing ? "" : String(subject),
        "arguments" => args, "namedArguments" => named,
        "schedule" => schedule === nothing ? "" : String(schedule),
        "engine" => engine === nothing || engine == "auto" ? "" : String(engine))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "RunAnalysis", request)
    end
    answer
end

function _analysis_result(conn::Connection, answer)
    diags = _diagnostics(answer)
    instances = _reported_instances(conn, answer)
    graph = Dict{Int64,Instance}(inst.id => inst for inst in instances)
    verifications = VerificationVerdict[VerificationVerdict(v) for v in
        get(answer, "verificationVerdicts", Any[])]
    verdicts = Verdict[_verdict(v, instances, diags, _verifications_for(v, verifications))
        for v in get(answer, "verdicts", Any[])]
    AnalysisResult(_output_map(get(answer, "outputs", Any[]), graph), verdicts;
        instances=instances, diagnostics=diags,
        verifications=verifications,
        evaluations=_case_evaluations(get(answer, "evaluations", Any[]), graph),
        standing=Standing(answer))
end

"""Run an analysis case once and return outputs and verdicts."""
function run_analysis(model::Model, symbol_id::AbstractString; subject=nothing,
                      arguments=Any[], named_arguments=Dict(), schedule=nothing, engine=nothing)
    answer = _analysis_request(model, symbol_id; subject=subject, arguments=arguments,
        named_arguments=named_arguments, schedule=schedule, engine=engine)
    message = String(get(answer, "error", ""))
    partial = any(!isempty(get(answer, key, Any[])) for key in
                  ("outputs", "verdicts", "evaluations", "instances"))
    !isempty(message) && !partial && _raise_answer_error(answer)
    result = _analysis_result(model.connection, answer)
    isempty(message) || throw(AnalysisRunError(message, result.diagnostics, result))
    result
end

"""Run a parameter sweep and return its ordered result rows."""
function run_sweep(model::Model, symbol_id::AbstractString, ranges::AbstractDict;
                   subject=nothing, arguments=Any[], named_arguments=Dict(),
                   samples::Integer=0, seed::Integer=0, engine=nothing)
    conn = model.connection
    require_capability(conn, CAPABILITY_VERIFICATION)
    _engine_preflight(conn, engine)
    args = _encoded_arguments(conn, arguments)
    named = Dict{String,Any}(String(k) => _encoded(conn, v) for (k, v) in pairs(named_arguments))
    encoded_ranges = Any[]
    for (name, endpoints) in pairs(ranges)
        endpoints isa Tuple || endpoints isa AbstractVector ||
            throw(ArgumentError("range $(name) is a two- or three-value sequence"))
        length(endpoints) in (2, 3) ||
            throw(ArgumentError("range $(name) takes (from, to) or (from, to, step)"))
        entry = Dict{String,Any}("parameter" => String(name),
            "start" => _encoded(conn, endpoints[1]), "end" => _encoded(conn, endpoints[2]))
        length(endpoints) == 3 && (entry["step"] = _encoded(conn, endpoints[3]))
        push!(encoded_ranges, entry)
    end
    0 <= samples <= typemax(Int64) ||
        throw(ArgumentError("samples must be a non-negative Int64"))
    0 <= seed <= typemax(UInt64) ||
        throw(ArgumentError("seed must be a non-negative UInt64"))
    needed = String[CAPABILITY_VERIFICATION, CAPABILITY_COMPLEX_VALUES, CAPABILITY_STRUCTURED_VALUES]
    engine !== nothing && !isempty(String(engine)) && String(engine) != "auto" &&
        push!(needed, CAPABILITY_ENGINES)
    request = Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(symbol_id),
        "subjectSymbolId" => subject === nothing ? "" : String(subject), "arguments" => args,
        "namedArguments" => named, "ranges" => encoded_ranges, "samples" => Int(samples),
        "seed" => string(UInt64(seed)), "engine" => engine === nothing || engine == "auto" ? "" : String(engine))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "RunSweep", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    instances = _reported_instances(conn, answer)
    graph = Dict{Int64,Instance}(inst.id => inst for inst in instances)
    rows = SweepRow[]
    for row in get(answer, "rows", Any[])
        row_inputs = _output_map(get(row, "inputs", Any[]), graph)
        row_outputs = _output_map(get(row, "outputs", Any[]), graph)
        verdicts = Verdict[_verdict(v, instances, diags) for v in get(row, "verdicts", Any[])]
        push!(rows, SweepRow(row_inputs, row_outputs, verdicts,
            parse(Float64, string(get(row, "elapsedMicros", 0))) / 1e6,
            String(get(row, "error", "")),
            _case_evaluations(get(row, "evaluations", Any[]), graph)))
    end
    SweepTable(rows, String[String(v) for v in get(answer, "parameters", Any[])],
        Bool(get(answer, "sampled", false)), parse(UInt64, string(get(answer, "seed", 0))),
        instances, diags, Standing(answer))
end

function list_engines(model::Model)
    list_engines(model.connection)
end
