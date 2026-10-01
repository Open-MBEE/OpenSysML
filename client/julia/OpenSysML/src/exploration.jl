struct Outcome
    outputs::Dict{String,Any}
    final_state::String
    states_visited::Vector{String}
    error::String
    linearizations::Int
    probability::Float64
    witness::Vector{String}
    diagnostics::Vector{Diagnostic}
end
function Outcome(raw::AbstractDict)
    outputs = Dict{String,Any}()
    for (name, value) in get(raw, "outputs", Dict{String,Any}())
        outputs[String(name)] = try
            decode_value(value)
        catch error
            error isa UnsupportedValueError ? error : UnsupportedValueError(sprint(showerror, error))
        end
    end
    Outcome(outputs, String(get(raw, "finalState", "")),
        String[String(v) for v in get(raw, "statesVisited", Any[])],
        String(get(raw, "error", "")), Int(get(raw, "linearizations", 0)),
        asreal(get(raw, "probability", 0.0)),
        String[String(v) for v in get(raw, "witness", Any[])],
        Diagnostic[Diagnostic(d) for d in get(raw, "diagnostics", Any[])])
end
"""Return whether an explored outcome records an execution error."""
failed(outcome::Outcome) = !isempty(outcome.error)
function raise_for_error(outcome::Outcome)
    isempty(outcome.error) || throw(ExecutionFailure(outcome.error, outcome.diagnostics))
    outcome
end

struct Exploration
    outcomes::Vector{Outcome}
    complete::Bool
    runs::Int
    budgets_hit::Vector{String}
    runs_budget::Int
    depth_budget::Int
    probabilities_lower_bound::Bool
end
function Exploration(answer::AbstractDict)
    status = get(answer, "exploration", Dict{String,Any}())
    Exploration(Outcome[Outcome(o) for o in get(answer, "outcomes", Any[])],
        Bool(get(status, "complete", false)), Int(get(status, "runs", 0)),
        String[String(v) for v in get(status, "budgetsHit", Any[])],
        Int(get(status, "runsBudget", 0)), Int(get(status, "depthBudget", 0)),
        Bool(get(status, "probabilitiesLowerBound", false)))
end
Base.iterate(exploration::Exploration, state...) = iterate(exploration.outcomes, state...)
Base.length(exploration::Exploration) = length(exploration.outcomes)
Base.Bool(exploration::Exploration) = exploration.complete &&
    !any(failed, exploration.outcomes)
"""Return the completion status of an exploration."""
function status(exploration::Exploration)
    exploration.complete && return "complete ($(exploration.runs) runs)"
    budgets = join(["$(budget) budget $(budget == "depth" ? exploration.depth_budget : exploration.runs_budget)"
                    for budget in exploration.budgets_hit], " and ")
    "incomplete: $(budgets) hit after $(exploration.runs) runs; probabilities are lower bounds"
end
raise_for_incomplete(exploration::Exploration) =
    exploration.complete ? exploration : throw(ExecutionFailure(status(exploration), Diagnostic[]))
Base.show(io::IO, exploration::Exploration) = print(io, status(exploration))

function _explore_action_request(model::Model, action_id, inputs, schedule, performer)
    _is_exploring(schedule) || throw(ArgumentError("schedule must be 'explore' or 'explore:runs=<n>,depth=<d>'"))
    conn = model.connection
    _schedule_preflight(conn, schedule)
    performer !== nothing && require_capability(conn, CAPABILITY_PERFORMER)
    encoded = _encoded_inputs(model, inputs)
    needed = String[CAPABILITY_SCHEDULE, CAPABILITY_SCHEDULE_EXPLORE]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "ExecuteAction", Dict{String,Any}("modelHash" => model.hash,
            "actionSymbolId" => String(action_id), "inputs" => encoded,
            "schedule" => String(schedule),
            "performerSymbolId" => performer === nothing ? "" : String(performer)))
    end
    _raise_answer_error(answer)
    Exploration(answer)
end

"""Explore every distinct outcome of an action run."""
function explore_action(model::Model, action_id::AbstractString;
                        inputs=Dict(), schedule="explore", performer=nothing)
    _explore_action_request(model, action_id, inputs, schedule, performer)
end

function _explore_state_request(model::Model, state_id, events, schedule, performer)
    _is_exploring(schedule) || throw(ArgumentError("schedule must be 'explore' or 'explore:runs=<n>,depth=<d>'"))
    conn = model.connection
    _schedule_preflight(conn, schedule)
    performer !== nothing && require_capability(conn, CAPABILITY_PERFORMER)
    needed = String[CAPABILITY_SCHEDULE, CAPABILITY_SCHEDULE_EXPLORE]
    performer !== nothing && push!(needed, CAPABILITY_PERFORMER)
    answer = _translate(; capabilities=Tuple(needed), connection=conn) do
        call(conn, "ExecuteState", Dict{String,Any}("modelHash" => model.hash,
            "stateMachineSymbolId" => String(state_id),
            "events" => String[String(e) for e in events], "schedule" => String(schedule),
            "performerSymbolId" => performer === nothing ? "" : String(performer)))
    end
    _raise_answer_error(answer)
    Exploration(answer)
end

"""Explore every distinct outcome of a state-machine run."""
function explore_state(model::Model, state_id::AbstractString;
                       events=Any[], schedule="explore", performer=nothing)
    _explore_state_request(model, state_id, events, schedule, performer)
end

"""Explore every distinct outcome of an analysis run."""
function explore_analysis(model::Model, symbol_id::AbstractString; subject=nothing,
                          arguments=Any[], named_arguments=Dict(), schedule="explore")
    _is_exploring(schedule) || throw(ArgumentError("schedule must be 'explore' or 'explore:runs=<n>,depth=<d>'"))
    answer = _analysis_request(model, symbol_id; subject=subject, arguments=arguments,
        named_arguments=named_arguments, schedule=schedule, allow_exploration=true)
    _raise_answer_error(answer)
    Exploration(answer)
end
