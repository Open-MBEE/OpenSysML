struct VerificationVerdict
    case_id::String
    kind::String
    detail::String
    subcase::Bool
    requirement_id::String
    instance_id::Int64
    instance_type_id::String
end
VerificationVerdict(raw::AbstractDict) =
    VerificationVerdict(String(get(raw, "caseId", "")), String(get(raw, "kind", "")),
        String(get(raw, "detail", "")), Bool(get(raw, "subcase", false)),
        String(get(raw, "requirementId", "")), Int64(get(raw, "instanceId", 0)),
        String(get(raw, "instanceTypeId", "")))
Base.Bool(v::VerificationVerdict) = v.kind == "pass"
Base.show(io::IO, v::VerificationVerdict) = print(io, "$(v.kind) verification $(v.case_id)")

struct WitnessAssignment
    feature::String
    value::Any
    unit::String
    exact::String
end
WitnessAssignment(raw::AbstractDict) =
    WitnessAssignment(String(get(raw, "feature", "")),
        haskey(raw, "value") ? decode_value(raw["value"]) : nothing,
        String(get(raw, "unit", "")), String(get(raw, "exact", "")))

"""
    VerdictNote

One assumption or omission behind a verdict, worded as every surface reports it:
`kind` is `"assumed_one"` (a feature read as holding one value or object while only
the default `0..*` multiplicity governs it) or `"uncovered"` (an object-holding
feature whose own assertions the question does not state); `subject` names the
feature as the report does; `text` is the note as a sentence.
"""
struct VerdictNote
    kind::String
    subject::String
    text::String
end
VerdictNote(raw::AbstractDict) =
    VerdictNote(String(get(raw, "kind", "")), String(get(raw, "subject", "")),
        String(get(raw, "text", "")))

struct Verdict
    kind::String
    element_id::String
    element::String
    holds::Bool
    condition::String
    instance_id::Int64
    instance_type_id::String
    error::String
    failure_reason::String
    requirement_id::String
    engine::String
    strength::String
    bounds::Vector{Bound}
    instance_path::String
    question::String
    status::String
    witness::Vector{WitnessAssignment}
    notes::Vector{VerdictNote}
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    verifications::Vector{VerificationVerdict}
end
function Verdict(raw::AbstractDict; instances=Instance[], diagnostics=Diagnostic[],
                 verifications=VerificationVerdict[])
    Verdict(String(get(raw, "kind", "")), String(get(raw, "elementId", "")),
        String(get(raw, "element", get(raw, "elementId", ""))), Bool(get(raw, "holds", false)),
        String(get(raw, "condition", "")), parse(Int64, string(get(raw, "instanceId", 0))),
        String(get(raw, "instanceTypeId", "")), String(get(raw, "error", "")),
        String(get(raw, "failureReason", "")), String(get(raw, "requirementId", "")),
        String(get(raw, "engine", "")), String(get(raw, "strength", "")),
        Bound[Bound(b) for b in get(raw, "bounds", Any[])],
        String(get(raw, "instancePath", "")), String(get(raw, "question", "")),
        String(get(raw, "status", "")),
        WitnessAssignment[WitnessAssignment(w) for w in get(raw, "witness", Any[])],
        VerdictNote[VerdictNote(n) for n in get(raw, "notes", Any[])],
        Instance[instances...], Diagnostic[diagnostics...],
        VerificationVerdict[verifications...])
end
"""Return whether a verification verdict holds."""
holds(v::Verdict) = v.holds
Base.Bool(v::Verdict) = v.holds
function explain(v::Verdict)
    mark = v.holds ? "✓" : "✗"
    line = "$(mark) $(v.kind) $(v.element)"
    isempty(v.error) || (line *= " — $(v.error)")
    isempty(v.condition) || v.holds || (line *= " — $(v.condition)")
    line
end
Base.show(io::IO, v::Verdict) = print(io, explain(v))

struct Validation
    verdicts::Vector{Verdict}
    summary::Union{Nothing,Verdict}
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    verifications::Vector{VerificationVerdict}
    bounded::Bool
    standing::Standing
end
function Validation(verdicts, summary; instances=Instance[], diagnostics=Diagnostic[],
                    verifications=VerificationVerdict[], bounded=false)
    Validation(Verdict[verdicts...], summary, Instance[instances...], Diagnostic[diagnostics...],
        VerificationVerdict[verifications...], Bool(bounded),
        summary === nothing ? Standing() : Standing(Dict("engine" => summary.engine,
            "strength" => summary.strength, "bounds" => [Dict("name" => b.name,
            "limit" => b.limit, "reached" => b.reached) for b in summary.bounds])))
end
"""Return whether validation completed and every verdict holds."""
valid(v::Validation) = v.summary !== nothing && v.summary.holds && isempty(v.summary.error) && !v.bounded
"""Return the evaluated verdicts that were violated."""
violated(v::Validation) = Verdict[x for x in v.verdicts if !x.holds && isempty(x.error)]
"""Return verdicts that could not be evaluated."""
undecided(v::Validation) = Verdict[x for x in v.verdicts if !isempty(x.error)]
Base.Bool(v::Validation) = valid(v)
Base.length(v::Validation) = length(v.verdicts)
Base.iterate(v::Validation, state...) = iterate(v.verdicts, state...)
Base.getindex(v::Validation, i::Int) = v.verdicts[i]
Base.show(io::IO, v::Validation) = print(io, join(explain.(v.verdicts), "\n"))

struct CalcResult
    value::Any
    outputs::Dict{String,Any}
    diagnostics::Vector{Diagnostic}
    standing::Standing
end
CalcResult(value, outputs; diagnostics=Diagnostic[], standing=Standing()) =
    CalcResult(value, Dict{String,Any}(outputs), Diagnostic[diagnostics...], standing)
Base.show(io::IO, result::CalcResult) =
    print(io, isempty(result.outputs) ? string(result.value) : join(["$(k) = $(v)" for (k,v) in result.outputs], ", "))

struct CaseEvaluation
    function_id::String
    arguments::Vector{Any}
    result::Any
    error::String
    selected::Bool
    tied::Bool
end

"""The input file a previewed tool call would be given."""
struct ToolDryRunFile
    name::String
    format::String
    data::String
end
ToolDryRunFile(d::AbstractDict) = ToolDryRunFile(String(get(d, "name", "")),
    String(get(d, "format", "")), String(get(d, "data", "")))

"""One value a previewed tool call would hand over."""
struct ToolDryRunInput
    variable::String
    parameter::String
    value::String
    unit::String
end
ToolDryRunInput(d::AbstractDict) = ToolDryRunInput(String(get(d, "variable", "")),
    String(get(d, "parameter", "")), String(get(d, "value", "")), String(get(d, "unit", "")))

"""One result a previewed tool call would read back."""
struct ToolDryRunOutput
    variable::String
    parameter::String
end
ToolDryRunOutput(d::AbstractDict) = ToolDryRunOutput(String(get(d, "variable", "")),
    String(get(d, "parameter", "")))

"""
The external tool call an analysis would make, previewed without running the tool: what
`run_analysis(...; tool_dry_run=true)` stops at, as `sysml -tool-dry-run` prints it.
"""
struct ToolDryRun
    action::String
    tool::String
    version::String
    manifest::String
    executable::String
    protocol::String
    args::Vector{String}
    env::Vector{String}
    cwd::String
    stdin::String
    stdin_data::String
    input_file::Union{Nothing,ToolDryRunFile}
    output_dir::Bool
    inputs::Vector{ToolDryRunInput}
    outputs::Vector{ToolDryRunOutput}
    reply_source::String
    lines::Vector{String}
end
function ToolDryRun(d::AbstractDict)
    str(key) = String(get(d, key, ""))
    strs(key) = String[String(v) for v in get(d, key, Any[])]
    file = get(d, "inputFile", nothing)
    ToolDryRun(str("action"), str("tool"), str("version"), str("manifest"), str("executable"),
        str("protocol"), strs("args"), strs("env"), str("cwd"), str("stdin"), str("stdinData"),
        file === nothing ? nothing : ToolDryRunFile(file), Bool(get(d, "outputDir", false)),
        ToolDryRunInput[ToolDryRunInput(i) for i in get(d, "inputs", Any[])],
        ToolDryRunOutput[ToolDryRunOutput(o) for o in get(d, "outputs", Any[])],
        str("replySource"), strs("lines"))
end

struct AnalysisResult
    outputs::Dict{String,Any}
    verdicts::Vector{Verdict}
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    verifications::Vector{VerificationVerdict}
    evaluations::Vector{CaseEvaluation}
    standing::Standing
    tool_dry_run::Union{Nothing,ToolDryRun}
end
function AnalysisResult(outputs, verdicts; instances=Instance[], diagnostics=Diagnostic[],
                        verifications=VerificationVerdict[], evaluations=CaseEvaluation[],
                        standing=Standing(), tool_dry_run=nothing)
    AnalysisResult(Dict{String,Any}(outputs), Verdict[verdicts...], Instance[instances...],
        Diagnostic[diagnostics...], VerificationVerdict[verifications...],
        CaseEvaluation[evaluations...], standing, tool_dry_run)
end
selected(result::AnalysisResult) = CaseEvaluation[e for e in result.evaluations if e.selected]
satisfied(result::AnalysisResult) = all(v.holds for v in result.verdicts)
Base.Bool(result::AnalysisResult) = satisfied(result)

struct SweepRow
    inputs::Dict{String,Any}
    outputs::Dict{String,Any}
    verdicts::Vector{Verdict}
    seconds::Float64
    error::String
    evaluations::Vector{CaseEvaluation}
end
selected(row::SweepRow) = CaseEvaluation[e for e in row.evaluations if e.selected]
failed(row::SweepRow) = !isempty(row.error)
Base.Bool(row::SweepRow) = !failed(row) && all(v.holds for v in row.verdicts)

struct SweepTable
    rows::Vector{SweepRow}
    parameters::Vector{String}
    sampled::Bool
    seed::UInt64
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    standing::Standing
end
failures(table::SweepTable) = SweepRow[row for row in table.rows if failed(row)]
Base.length(table::SweepTable) = length(table.rows)
Base.iterate(table::SweepTable, state...) = iterate(table.rows, state...)
Base.getindex(table::SweepTable, index::Int) = table.rows[index]
Base.Bool(table::SweepTable) = !isempty(table.rows) && all(Bool, table.rows)
