struct VerificationVerdict
    case_id::String
    kind::String
    detail::String
    subcase::Bool
    requirement_id::String
end
VerificationVerdict(raw::AbstractDict) =
    VerificationVerdict(String(get(raw, "caseId", "")), String(get(raw, "kind", "")),
        String(get(raw, "detail", "")), Bool(get(raw, "subcase", false)),
        String(get(raw, "requirementId", "")))
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

struct AnalysisResult
    outputs::Dict{String,Any}
    verdicts::Vector{Verdict}
    instances::Vector{Instance}
    diagnostics::Vector{Diagnostic}
    verifications::Vector{VerificationVerdict}
    evaluations::Vector{CaseEvaluation}
    standing::Standing
end
function AnalysisResult(outputs, verdicts; instances=Instance[], diagnostics=Diagnostic[],
                        verifications=VerificationVerdict[], evaluations=CaseEvaluation[],
                        standing=Standing())
    AnalysisResult(Dict{String,Any}(outputs), Verdict[verdicts...], Instance[instances...],
        Diagnostic[diagnostics...], VerificationVerdict[verifications...],
        CaseEvaluation[evaluations...], standing)
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
