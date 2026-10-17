"""One schedule replayed: pass `schedule` as a run's schedule to follow it."""
struct CheckWitness
    schedule::String
    draws::Vector{String}
    choices::Vector{String}
    inputs::Vector{String}
    file_name::String
    file_content::String
end
CheckWitness(raw::AbstractDict) = CheckWitness(_render_string(raw, "schedule"),
    String[string(d) for d in get(raw, "draws", Any[])],
    String[string(c) for c in get(raw, "choices", Any[])],
    String[string(i) for i in get(raw, "inputs", Any[])],
    _render_string(raw, "fileName"), _render_string(raw, "fileContent"))
_witness(raw::AbstractDict, key) = haskey(raw, key) ? CheckWitness(raw[key]) : nothing

"""One violation witnessed: a property false at a state, a deadlock or a failure."""
struct CheckViolation
    kind::String
    property::String
    error::String
    depth::Int
    probability::Float64
    witness::Union{CheckWitness,Nothing}
    text::String
end
CheckViolation(raw::AbstractDict) = CheckViolation(_render_string(raw, "kind"),
    _render_string(raw, "property"), _render_string(raw, "error"), Int(get(raw, "depth", 0)),
    Float64(get(raw, "probability", 0.0)), _witness(raw, "witness"), _render_string(raw, "text"))

"""One final value a feature reaches and the schedule reaching it."""
struct CheckDivergentValue
    value::String
    witness::Union{CheckWitness,Nothing}
end

"""A feature whose final value the schedule decides."""
struct CheckDivergence
    feature::String
    values::Vector{CheckDivergentValue}
end
CheckDivergence(raw::AbstractDict) = CheckDivergence(_render_string(raw, "feature"),
    CheckDivergentValue[CheckDivergentValue(_render_string(v, "value"), _witness(v, "witness"))
                        for v in get(raw, "values", Any[])])

"""One final outcome of an exhaustive search."""
struct CheckFinal
    outcome::String
    values::Dict{String,String}
    witness::Union{CheckWitness,Nothing}
end
CheckFinal(raw::AbstractDict) = CheckFinal(_render_string(raw, "outcome"),
    Dict{String,String}(string(k) => string(v) for (k, v) in get(raw, "values", Dict())),
    _witness(raw, "witness"))

"""The explicit-state search's statistics."""
struct CheckSearch
    states::Int
    moves::Int
    max_depth::Int
    bounds_hit::Vector{String}
    horizon::Float64
    horizon_set::Bool
    mass_bounded::Bool
    notes::Vector{String}
    scope::Vector{String}
    status::String
end
CheckSearch(raw::AbstractDict) = CheckSearch(Int(get(raw, "states", 0)), Int(get(raw, "moves", 0)),
    Int(get(raw, "maxDepth", 0)), String[string(b) for b in get(raw, "boundsHit", Any[])],
    Float64(get(raw, "horizon", 0.0)), Bool(get(raw, "horizonSet", false)),
    Bool(get(raw, "massBounded", false)), String[string(n) for n in get(raw, "notes", Any[])],
    String[string(s) for s in get(raw, "scope", Any[])], _render_string(raw, "status"))

"""
What `CheckBehavior` answered: the verdict (`holds`, `fails` or `undecided`) over
every schedule of a behavior, with each violation, divergence and final outcome
witnessed by a schedule that replays it, and the report `sysml -check-property`
prints as `lines`.
"""
struct CheckResult
    verdict::String
    lines::Vector{String}
    search::Union{CheckSearch,Nothing}
    violations::Vector{CheckViolation}
    divergences::Vector{CheckDivergence}
    finals::Vector{CheckFinal}
    claim::String
    free_inputs::Vector{String}
    assumptions::Vector{String}
    witness::Union{CheckWitness,Nothing}
    contrast::Union{CheckWitness,Nothing}
    standing::Standing
    reason::String
end
CheckResult(raw::AbstractDict) = CheckResult(_render_string(raw, "verdict"),
    String[string(l) for l in get(raw, "lines", Any[])],
    haskey(raw, "search") ? CheckSearch(raw["search"]) : nothing,
    CheckViolation[CheckViolation(v) for v in get(raw, "violations", Any[])],
    CheckDivergence[CheckDivergence(d) for d in get(raw, "divergences", Any[])],
    CheckFinal[CheckFinal(f) for f in get(raw, "finals", Any[])],
    _render_string(raw, "claim"),
    String[string(i) for i in get(raw, "freeInputs", Any[])],
    String[string(a) for a in get(raw, "assumptions", Any[])],
    _witness(raw, "witness"), _witness(raw, "contrast"), Standing(raw),
    _render_string(raw, "reason"))

"""Whether every schedule kept every property."""
holds(result::CheckResult) = result.verdict == "holds"

"""
    check_action(model, action_id; inputs=Dict(), performer=nothing, properties=(),
                 diverge=(), free_inputs=(), assume=(), witness=false, depth=0,
                 states=0, unroll=0, timeout=0.0, engine=nothing)

Search every schedule of an action for a violated property, a deadlock, a failure
or a divergent value, as `sysml -check-property` does. `properties` and `assume`
name constraints or requirements, `diverge` features whose final value the
schedule may decide (`this.<name>` is the performer's), `free_inputs` the inputs
the symbolic engine ranges over; the bounds are 0 for the engine's default.
"""
function check_action(model::Model, action_id::AbstractString; inputs=Dict(), kwargs...)
    return _check_behavior(model, String(action_id), "", _encoded_inputs(model, inputs); kwargs...)
end

"""Search every schedule of a state machine; see [`check_action`](@ref)."""
function check_state_machine(model::Model, state_machine_id::AbstractString; kwargs...)
    return _check_behavior(model, "", String(state_machine_id), Dict{String,Any}(); kwargs...)
end

function _check_behavior(model::Model, action_id::String, state_machine_id::String, encoded;
                         performer=nothing, properties=(), diverge=(), free_inputs=(), assume=(),
                         witness::Bool=false, depth::Integer=0, states::Integer=0,
                         unroll::Integer=0, timeout::Real=0.0, engine=nothing)
    conn = model.connection
    needed = String[CAPABILITY_CHECK]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    foreach(c -> require_capability(conn, c), needed)
    request = Dict{String,Any}("modelHash" => model.hash, "actionSymbolId" => action_id,
        "stateMachineSymbolId" => state_machine_id, "inputs" => encoded,
        "checkProperty" => String[string(p) for p in properties],
        "checkDiverge" => String[string(d) for d in diverge],
        "checkInput" => String[string(i) for i in free_inputs],
        "checkAssume" => String[string(a) for a in assume],
        "checkWitness" => witness, "checkDepth" => Int(depth), "checkStates" => Int(states),
        "checkUnroll" => Int(unroll), "checkTimeout" => Float64(timeout))
    performer !== nothing && (request["performerSymbolId"] = String(performer))
    engine !== nothing && (request["engine"] = String(engine))
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "CheckBehavior", request)
    end
    _check_error(answer, "CheckBehavior")
    return CheckResult(answer)
end
