_int(raw, key) = parse(Int, string(get(raw, key, 0)))

"""One observed feature over the completed runs: its count, and for numbers the statistics."""
struct RunDistribution
    name::String
    unit::String
    count::Int
    numeric::Bool
    min::Any
    max::Any
    mean::Any
    deviation::Float64
    percentiles::Vector{Pair{Float64,Any}}
    histogram::Vector{Tuple{Any,Any,Int}}
    values::Vector{Pair{Any,Int}}
end
function RunDistribution(raw::AbstractDict, graph)
    decode(key) = haskey(raw, key) ? _decode_or_unsupported(raw[key], graph) : nothing
    RunDistribution(_render_string(raw, "name"), _render_string(raw, "unit"),
        _int(raw, "count"), Bool(get(raw, "numeric", false)),
        decode("min"), decode("max"), decode("mean"), Float64(get(raw, "deviation", 0.0)),
        Pair{Float64,Any}[Float64(p["percentile"]) => _decode_or_unsupported(p["value"], graph)
                          for p in get(raw, "percentiles", Any[])],
        Tuple{Any,Any,Int}[(_decode_or_unsupported(b["low"], graph),
                            _decode_or_unsupported(b["high"], graph), _int(b, "count"))
                           for b in get(raw, "histogram", Any[])],
        Pair{Any,Int}[_decode_or_unsupported(v["value"], graph) => _int(v, "count")
                      for v in get(raw, "values", Any[])])
end

"""
What `RunRuns` answered: one [`SweepRow`](@ref) per run with the observed features as
its outputs, each observable's [`RunDistribution`](@ref), and for a Monte Carlo case
its conclusion over the sample as `outputs` and `verdicts`.
"""
struct RunsResult
    rows::Vector{SweepRow}
    observables::Vector{String}
    distributions::Vector{RunDistribution}
    outputs::Dict{String,Any}
    verdicts::Vector{Verdict}
    unconcluded::String
    seed::Union{UInt64,Nothing}
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    standing::Standing
end
Base.length(result::RunsResult) = length(result.rows)
Base.iterate(result::RunsResult, state...) = iterate(result.rows, state...)
Base.getindex(result::RunsResult, index::Int) = result.rows[index]
"""Whether every run completed."""
completed(result::RunsResult) = !isempty(result.rows) && !any(failed, result.rows)
"""One observable's distribution by name, or `nothing`."""
function distribution(result::RunsResult, name::AbstractString)
    i = findfirst(d -> d.name == name, result.distributions)
    i === nothing ? nothing : result.distributions[i]
end

"""
    run_runs(model; action=nothing, analysis=nothing, runs=1, seed=nothing, draws=nothing,
             clock_step=0.0, observe=(), inputs=Dict(), performer=nothing, subject=nothing,
             arguments=Any[], named_arguments=Dict(), engine=nothing)

Run an action (`action`, with `inputs`, on a `performer`) or a Monte Carlo analysis
case (`analysis`, on a `subject` with `arguments`) `runs` times and table every run,
as `sysml -runs` does; `observe` names the action's features to table. The runs
draw at random, so pass `seed`, or fix the draws with `draws` as `min`, `max` or
`average`.
"""
function run_runs(model::Model; action=nothing, analysis=nothing, runs::Integer=1, seed=nothing,
                  draws=nothing, clock_step::Real=0.0, observe=(), inputs=Dict(),
                  performer=nothing, subject=nothing, arguments=Any[], named_arguments=Dict(),
                  engine=nothing)
    conn = model.connection
    needed = String[CAPABILITY_RUNS]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    foreach(c -> require_capability(conn, c), needed)
    request = Dict{String,Any}("modelHash" => model.hash,
        "actionSymbolId" => action === nothing ? "" : String(action),
        "analysisSymbolId" => analysis === nothing ? "" : String(analysis),
        "inputs" => _encoded_inputs(model, inputs),
        "performerSymbolId" => performer === nothing ? "" : String(performer),
        "subjectSymbolId" => subject === nothing ? "" : String(subject),
        "arguments" => _encoded_arguments(conn, arguments),
        "namedArguments" => Dict{String,Any}(String(k) => _encoded(conn, v) for (k, v) in pairs(named_arguments)),
        "runs" => Int(runs), "draws" => draws === nothing ? "" : String(draws),
        "clockStep" => Float64(clock_step), "observe" => String[string(o) for o in observe],
        "engine" => engine === nothing ? "" : String(engine))
    seed !== nothing && (request["seed"] = string(UInt64(seed)))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "RunRuns", request)
    end
    return _runs_result(conn, answer)
end

function _runs_result(conn::Connection, answer)
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    instances = _reported_instances(conn, answer)
    graph = Dict{Int64,Instance}(inst.id => inst for inst in instances)
    RunsResult(_sweep_rows(answer, graph, instances, diags),
        String[string(o) for o in get(answer, "observables", Any[])],
        RunDistribution[RunDistribution(d, graph) for d in get(answer, "distributions", Any[])],
        _output_map(get(answer, "outputs", Any[]), graph),
        Verdict[_verdict(v, instances, diags) for v in get(answer, "verdicts", Any[])],
        _render_string(answer, "unconcluded"),
        haskey(answer, "seed") ? parse(UInt64, string(answer["seed"])) : nothing,
        instances, diags, Standing(answer))
end

"""One thing the self-check reports: a violation, an unevaluated check, an error or a skipped rule."""
struct SelfCheckFinding
    status::String
    subject::String
    location::String
    constraint::String
    element::String
    message::String
    line::String
end
SelfCheckFinding(raw::AbstractDict) = SelfCheckFinding(_render_string(raw, "status"),
    _render_string(raw, "subject"), _render_string(raw, "location"),
    _render_string(raw, "constraint"), _render_string(raw, "element"),
    _render_string(raw, "message"), _render_string(raw, "line"))

"""
What `SelfCheck` answered: every rule constraint of `SysMLValidation` applied to the
model, with the findings and counts `sysml -self-check` prints; `status` is `holds`
when nothing was violated and nothing errored.
"""
struct SelfCheckResult
    status::String
    findings::Vector{SelfCheckFinding}
    elements::Int
    checks::Int
    violations::Int
    unevaluated::Int
    evaluation_errors::Int
    applications::Vector{Pair{String,Int}}
    summary::String
    diagnostics::Vector{Diagnostic}
end
SelfCheckResult(raw::AbstractDict) = SelfCheckResult(_render_string(raw, "status"),
    SelfCheckFinding[SelfCheckFinding(f) for f in get(raw, "findings", Any[])],
    _int(raw, "elements"), _int(raw, "checks"), _int(raw, "violations"),
    _int(raw, "unevaluated"), _int(raw, "evaluationErrors"),
    Pair{String,Int}[_render_string(a, "constraint") => _int(a, "count")
                     for a in get(raw, "applications", Any[])],
    _render_string(raw, "summary"), _diagnostics(raw))
"""Whether the check found no violation and no evaluation error."""
holds(result::SelfCheckResult) = result.status == "holds"

"""
    self_check(model; packages=())

Apply every rule constraint of the `SysMLValidation` library, and of `packages`, to
the model as `sysml -self-check` does.
"""
function self_check(model::Model; packages=())
    conn = model.connection
    require_capability(conn, CAPABILITY_SELF_CHECK)
    request = Dict{String,Any}("modelHash" => model.hash,
        "selfCheckPackage" => String[string(p) for p in packages])
    answer = _translate(; capabilities=(CAPABILITY_SELF_CHECK,), connection=conn) do
        call(conn, "SelfCheck", request)
    end
    _check_error(answer, "SelfCheck")
    return SelfCheckResult(answer)
end

"""
What `RecordRun` did: the run, sweep or sample it made, the records it generated and
the recorded model's hash, which the service holds as a new model.
"""
struct Recording
    run::Union{AnalysisResult,Nothing}
    sweep::Union{SweepTable,Nothing}
    sample::Union{RunsResult,Nothing}
    package::String
    definition::String
    records::Vector{String}
    source::String
    content::String
    documents::Vector{EditedDocument}
    skipped::Int
    reasons::Vector{String}
    diagnostics::Vector{Diagnostic}
    model_hash::String
end

"""
    record_run(model, analysis_id; subject=nothing, arguments=Any[], named_arguments=Dict(),
               into=nothing, ranges=nothing, runs=0, seed=nothing, engine=nothing)

Run an analysis case once, over the sweep `ranges`, or as a Monte Carlo sample of
`runs`, and record the result into the model as `sysml -record-run` does; `into`
names the package the records go to (`Records` by default). The recorded model is
returned as `model_hash`.
"""
function record_run(model::Model, analysis_id::AbstractString; subject=nothing, arguments=Any[],
                    named_arguments=Dict(), into=nothing, ranges=nothing, runs::Integer=0,
                    seed=nothing, engine=nothing)
    conn = model.connection
    require_capability(conn, CAPABILITY_RECORD_RUN)
    encoded_ranges = Any[]
    if ranges !== nothing
        for (name, endpoints) in _ordered_sweep_ranges(ranges)
            entry = Dict{String,Any}("parameter" => String(name),
                "start" => _encoded(conn, endpoints[1]), "end" => _encoded(conn, endpoints[2]))
            length(endpoints) == 3 && (entry["step"] = _encoded(conn, endpoints[3]))
            push!(encoded_ranges, entry)
        end
    end
    request = Dict{String,Any}("modelHash" => model.hash, "analysisSymbolId" => String(analysis_id),
        "subjectSymbolId" => subject === nothing ? "" : String(subject),
        "arguments" => _encoded_arguments(conn, arguments),
        "namedArguments" => Dict{String,Any}(String(k) => _encoded(conn, v) for (k, v) in pairs(named_arguments)),
        "engine" => engine === nothing ? "" : String(engine),
        "recordInto" => into === nothing ? "" : String(into), "ranges" => encoded_ranges,
        "runs" => Int(runs), "acceptDocuments" => true)
    seed !== nothing && (request["seed"] = string(UInt64(seed)))
    answer = _translate(; capabilities=(CAPABILITY_RECORD_RUN,), connection=conn) do
        call(conn, "RecordRun", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    return Recording(
        haskey(answer, "run") ? _analysis_result(conn, answer["run"]) : nothing,
        haskey(answer, "sweep") ? _sweep_table(conn, answer["sweep"]) : nothing,
        haskey(answer, "sample") ? _runs_result(conn, answer["sample"]) : nothing,
        _render_string(answer, "package"), _render_string(answer, "definition"),
        String[string(r) for r in get(answer, "records", Any[])],
        _render_string(answer, "source"), _render_string(answer, "content"),
        EditedDocument[EditedDocument(_render_string(d, "name"), _render_string(d, "content"))
                       for d in get(answer, "documents", Any[])],
        _int(answer, "skipped"), String[string(r) for r in get(answer, "reasons", Any[])],
        diags, _render_string(answer, "modelHash"))
end
