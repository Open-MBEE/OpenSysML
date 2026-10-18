"""One feature value a data import set."""
struct ImportedValue
    element::String
    feature::String
    value::String
    redefines::String
    location::String
    description::String
end

"""What one data import assigned: the values set, in order, and their counts."""
struct ImportedFile
    name::String
    values::Int
    elements::Int
    assignments::Vector{ImportedValue}
end

"""
What `import_data` answers: the values each import set and the edited model, whose hash
the service holds as a new model (empty on a dry run).
"""
struct ImportResult
    files::Vector{ImportedFile}
    documents::Vector{EditedDocument}
    content::String
    model_hash::String
    diagnostics::Vector{Diagnostic}
    changed::Bool
end

"""
Whether the import changed the model: the model the service answers is not the one imported
into. False on a dry run, and for an import that set nothing or only values the model already
had, which the service answers under the original hash.
"""
applied(result::ImportResult) = result.changed

"""
One tabular or JSON data file to import: its `name` (whose extension selects the format
unless `format` does), its `content`, and the `map` (the `-import-map` file's content)
naming which column sets which feature.
"""
struct DataImport
    name::String
    content::String
    format::String
    map::String
end

DataImport(name::AbstractString, content::AbstractString; format="", map="") =
    DataImport(String(name), String(content), String(format), String(map))

"""
Set the feature values tabular or JSON data assigns, one row per element, as
`sysml -import` does. `imports` are `DataImport`s (or `name => content` pairs), imported in
order; `dry_run` reports the assignments without changing the model; `accept_documents`
accepts a multi-document model's edited documents. An import the model refuses raises an
`OpenSysMLError` and leaves the model unchanged.
"""
function import_data(model::Model, imports; dry_run::Bool=false, accept_documents::Bool=false)
    conn = model.connection
    require_capability(conn, CAPABILITY_IMPORT_DATA)
    encoded = Any[]
    for item in imports
        entry = item isa DataImport ? item : DataImport(first(item), last(item))
        push!(encoded, Dict{String,Any}("name" => entry.name, "content" => entry.content,
            "importFormat" => entry.format, "importMap" => entry.map))
    end
    request = Dict{String,Any}("modelHash" => model.hash, "imports" => encoded,
        "importDryRun" => dry_run, "acceptDocuments" => accept_documents)
    answer = _translate(; capabilities=(CAPABILITY_IMPORT_DATA,), connection=conn) do
        call(conn, "ImportData", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    files = ImportedFile[]
    for file in get(answer, "files", Any[])
        assignments = ImportedValue[ImportedValue(_render_string(a, "element"),
            _render_string(a, "feature"), _render_string(a, "value"),
            _render_string(a, "redefines"), _render_string(a, "where"),
            _render_string(a, "description")) for a in get(file, "assignments", Any[])]
        push!(files, ImportedFile(_render_string(file, "name"),
            parse(Int, string(get(file, "values", 0))),
            parse(Int, string(get(file, "elements", 0))), assignments))
    end
    documents = EditedDocument[EditedDocument(_render_string(d, "name"), _render_string(d, "content"))
                               for d in get(answer, "documents", Any[])]
    model_hash = _render_string(answer, "modelHash")
    ImportResult(files, documents, _render_string(answer, "content"), model_hash, diags,
        !isempty(model_hash) && model_hash != model.hash)
end

"""The statistics of one observable's values, from a tool's results or from runs."""
struct ComparedDistribution
    count::Int
    min::Float64
    mean::Float64
    p50::Float64
    p90::Float64
    max::Float64
    deviation::Float64
end

"""The signed relative differences between the runs' statistics and the tool's, as text."""
struct StatisticDifference
    min::String
    mean::String
    p50::String
    p90::String
    max::String
end

"""One statistic an analysis case declares of an observable, beside what the tool stored."""
struct CaseStatistic
    statistic::String
    of::String
    tool::String
    run::String
    difference::String
end

"""The statistics an analysis case declares of an observable."""
struct CaseStatistics
    analysis::String
    rows::Vector{CaseStatistic}
    notes::Vector{String}
end

"""One observable's distribution in a tool's results beside the runs' distribution of it."""
struct ObservableComparison
    stored::String
    feature::String
    unit::String
    tool::Union{ComparedDistribution,Nothing}
    pooled::Bool
    unstored::Bool
    run::Union{ComparedDistribution,Nothing}
    produced::Int
    compared::Bool
    difference::Union{StatisticDifference,Nothing}
    declared::Union{CaseStatistics,Nothing}
    notes::Vector{String}
end

"""One configuration of a tool's results index, run again and compared."""
struct ConfigurationComparison
    id::String
    name::String
    behavior::String
    target::String
    result_location::String
    status::String
    unresolved::String
    stored_runs::Int
    stored_snapshots::Int
    runs::Int
    completed::Int
    draws::String
    seed::Union{UInt64,Nothing}
    clock_step::Float64
    rows::Vector{SweepRow}
    instances::Vector{Instance}
    observables::Vector{ObservableComparison}
    errors::Vector{String}
    notes::Vector{String}
    lines::Vector{String}
end

"""
What `compare_results` answers: each configuration of a SysML v1 tool's results index run
again, with the tool's distributions beside the runs'.
"""
struct Comparison
    source::String
    configurations::Vector{ConfigurationComparison}
    standing::Standing
    diagnostics::Vector{Diagnostic}
end

_strings(raw) = String[String(v) for v in raw]

function _distribution(raw)
    raw === nothing && return nothing
    ComparedDistribution(parse(Int, string(get(raw, "count", 0))),
        Float64(get(raw, "min", 0.0)), Float64(get(raw, "mean", 0.0)),
        Float64(get(raw, "p50", 0.0)), Float64(get(raw, "p90", 0.0)),
        Float64(get(raw, "max", 0.0)), Float64(get(raw, "deviation", 0.0)))
end

function _observable(raw)
    difference = get(raw, "difference", nothing)
    declared = get(raw, "declared", nothing)
    ObservableComparison(_render_string(raw, "stored"), _render_string(raw, "feature"),
        _render_string(raw, "unit"), _distribution(get(raw, "tool", nothing)),
        Bool(get(raw, "pooled", false)), Bool(get(raw, "unstored", false)),
        _distribution(get(raw, "run", nothing)), parse(Int, string(get(raw, "produced", 0))),
        Bool(get(raw, "compared", false)),
        difference === nothing ? nothing :
            StatisticDifference(_render_string(difference, "min"), _render_string(difference, "mean"),
                _render_string(difference, "p50"), _render_string(difference, "p90"),
                _render_string(difference, "max")),
        declared === nothing ? nothing :
            CaseStatistics(_render_string(declared, "analysis"),
                CaseStatistic[CaseStatistic(_render_string(r, "statistic"), _render_string(r, "of"),
                    _render_string(r, "tool"), _render_string(r, "run"),
                    _render_string(r, "difference")) for r in get(declared, "rows", Any[])],
                _strings(get(declared, "notes", Any[]))),
        _strings(get(raw, "notes", Any[])))
end

"""
Run the configurations a SysML v1 tool's results index (`results`, the sidecar's JSON)
names and table the tool's distributions beside the runs', as `sysml -compare-results`
does. Options: `actions` selects configurations by name or id; `runs` per configuration
(0 for as many as the tool stored); `seed` and `draws` control random draws; `clock_step`
the clock; `observe` maps stored observable names onto the features that produce them;
`engine` picks the engine.
"""
function compare_results(model::Model, results::AbstractString; actions=(), runs::Integer=0,
                         seed=nothing, draws="", clock_step=nothing, observe=(), engine=nothing)
    conn = model.connection
    require_capability(conn, CAPABILITY_COMPARE_RESULTS)
    _engine_preflight(conn, engine)
    0 <= runs <= typemax(Int64) || throw(ArgumentError("runs must be a non-negative Int64"))
    seed === nothing || 0 <= seed <= typemax(UInt64) ||
        throw(ArgumentError("seed must be a non-negative UInt64"))
    needed = String[CAPABILITY_COMPARE_RESULTS]
    append!(needed, _engine_capabilities(engine))
    request = Dict{String,Any}("modelHash" => model.hash, "results" => String(results),
        "actions" => _strings(actions), "runs" => Int(runs), "draws" => String(draws),
        "observe" => Any[Dict{String,Any}("stored" => String(first(pair)),
            "feature" => String(last(pair))) for pair in observe],
        "engine" => engine === nothing || engine == "auto" ? "" : String(engine))
    seed === nothing || (request["seed"] = string(UInt64(seed)))
    clock_step === nothing || (request["clockStep"] = Float64(clock_step))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "CompareResults", request)
    end
    diags = _diagnostics(answer)
    _raise_answer_error(answer; diagnostics=diags)
    configurations = ConfigurationComparison[]
    for raw in get(answer, "configurations", Any[])
        instances = _reported_instances(conn, raw)
        graph = Dict{Int64,Instance}(inst.id => inst for inst in instances)
        rows = SweepRow[]
        for row in get(raw, "rows", Any[])
            push!(rows, SweepRow(_output_map(get(row, "inputs", Any[]), graph),
                _output_map(get(row, "outputs", Any[]), graph),
                Verdict[_verdict(v, instances, diags) for v in get(row, "verdicts", Any[])],
                parse(Float64, string(get(row, "elapsedMicros", 0))) / 1e6,
                String(get(row, "error", "")),
                _case_evaluations(get(row, "evaluations", Any[]), graph)))
        end
        seed_text = get(raw, "seed", nothing)
        push!(configurations, ConfigurationComparison(_render_string(raw, "id"),
            _render_string(raw, "name"), _render_string(raw, "behavior"),
            _render_string(raw, "target"), _render_string(raw, "resultLocation"),
            _render_string(raw, "status"), _render_string(raw, "unresolved"),
            parse(Int, string(get(raw, "storedRuns", 0))),
            parse(Int, string(get(raw, "storedSnapshots", 0))),
            parse(Int, string(get(raw, "runs", 0))), parse(Int, string(get(raw, "completed", 0))),
            _render_string(raw, "draws"),
            seed_text === nothing ? nothing : parse(UInt64, string(seed_text)),
            Float64(get(raw, "clockStep", 0.0)), rows, instances,
            ObservableComparison[_observable(o) for o in get(raw, "observables", Any[])],
            _strings(get(raw, "errors", Any[])), _strings(get(raw, "notes", Any[])),
            _strings(get(raw, "lines", Any[]))))
    end
    Comparison(_render_string(answer, "source"), configurations, Standing(answer), diags)
end
