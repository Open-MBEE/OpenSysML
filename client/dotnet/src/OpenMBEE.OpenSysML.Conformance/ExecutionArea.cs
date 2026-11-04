using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Running and checking behavior: ExecuteAction and ExecuteState through <see cref="Model"/>'s execute and
/// explore calls, chosen by whether the request's schedule explores, and CheckBehavior through its check
/// calls. A run the service reports failed reaches the caller as an <see cref="ExecutionException"/>, and a
/// check it refuses in its answer as a <see cref="CheckException"/>; each is rendered back in band with what
/// it kept of the answer.
/// </summary>
internal static class ExecutionArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ExecuteActionRequest>(
            "ExecuteAction",
            ["model_hash", "action_symbol_id", "inputs", "schedule", "performer_symbol_id", "advance", "seed", "draws", "clock_step", "trace", "events"],
            ExecuteActionAsync),
        RpcAdapter.Of<Wire.ExecuteStateRequest>(
            "ExecuteState",
            ["model_hash", "state_machine_symbol_id", "events", "schedule", "performer_symbol_id", "trace", "advance", "seed", "draws", "clock_step"],
            ExecuteStateAsync),
        RpcAdapter.Of<Wire.CheckBehaviorRequest>(
            "CheckBehavior",
            [
                "model_hash", "action_symbol_id", "state_machine_symbol_id", "inputs", "performer_symbol_id", "check_property", "check_diverge",
                "check_input", "check_assume", "check_witness", "check_depth", "check_states", "check_unroll", "check_timeout", "engine",
            ],
            CheckBehaviorAsync),
    ];

    private static async Task<ScenarioAnswer> ExecuteActionAsync(Connection connection, Wire.ExecuteActionRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = Options(
            request.Schedule,
            request.PerformerSymbolId,
            request.Trace,
            request.HasAdvance ? request.Advance : null,
            request.HasSeed ? request.Seed : null,
            request.Draws,
            request.ClockStep,
            request.Events);
        var inputs = RequestValues.Map(request.Inputs, "inputs");
        var response = new Wire.ExecuteActionResponse();
        try
        {
            if (options.Explores)
            {
                var exploration = await model.ExploreActionAsync(request.ActionSymbolId, inputs, options, cancellationToken).ConfigureAwait(false);
                response.Outcomes.AddRange(Rendering.Outcomes(exploration));
                response.Exploration = Rendering.ExplorationStatus(exploration);
            }
            else
            {
                response = Rendering.ActionRun(await model.ExecuteActionAsync(request.ActionSymbolId, inputs, options, cancellationToken).ConfigureAwait(false));
            }
        }
        catch (ExecutionException failure)
        {
            response.Error = failure.Message;
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
            response.FinalTime = failure.FinalTime;
            response.Trace.AddRange(Rendering.Trace(failure.Trace));
            response.TraceDropped = failure.TraceDropped;
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> ExecuteStateAsync(Connection connection, Wire.ExecuteStateRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = Options(
            request.Schedule,
            request.PerformerSymbolId,
            request.Trace,
            request.HasAdvance ? request.Advance : null,
            request.HasSeed ? request.Seed : null,
            request.Draws,
            request.ClockStep);
        var response = new Wire.ExecuteStateResponse();
        try
        {
            if (options.Explores)
            {
                var exploration = await model.ExploreStateAsync(request.StateMachineSymbolId, request.Events, options, cancellationToken).ConfigureAwait(false);
                response.Outcomes.AddRange(Rendering.Outcomes(exploration));
                response.Exploration = Rendering.ExplorationStatus(exploration);
            }
            else
            {
                response = Rendering.StateRun(await model.ExecuteStateAsync(request.StateMachineSymbolId, request.Events, options, cancellationToken).ConfigureAwait(false));
            }
        }
        catch (ExecutionException failure)
        {
            response.Error = failure.Message;
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
            response.FinalTime = failure.FinalTime;
            response.Trace.AddRange(Rendering.Trace(failure.Trace));
            response.TraceDropped = failure.TraceDropped;
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> CheckBehaviorAsync(Connection connection, Wire.CheckBehaviorRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.ActionSymbolId.Length > 0 && request.StateMachineSymbolId.Length > 0)
        {
            throw new UnexpressibleRequestException("the public API checks one behavior, so it cannot name both an action and a state machine");
        }

        var inputs = RequestValues.Map(request.Inputs, "inputs");
        if (request.StateMachineSymbolId.Length > 0 && inputs is not null)
        {
            throw new UnexpressibleRequestException("the public API binds inputs only to an action it checks, so it cannot send them with a state machine");
        }

        var options = new CheckOptions
        {
            Properties = request.CheckProperty,
            Diverge = request.CheckDiverge,
            FreeInputs = request.CheckInput,
            Assume = request.CheckAssume,
            Witness = request.CheckWitness,
            Depth = request.CheckDepth,
            States = request.CheckStates,
            Unroll = request.CheckUnroll,
            Timeout = request.CheckTimeout,
            Engine = request.Engine.Length > 0 ? request.Engine : null,
            PerformerSymbolId = request.PerformerSymbolId.Length > 0 ? request.PerformerSymbolId : null,
        };
        try
        {
            // A request naming neither behavior is an action check naming the empty action, which is what it sends.
            var result = request.StateMachineSymbolId.Length > 0
                ? await model.CheckStateAsync(request.StateMachineSymbolId, options, cancellationToken).ConfigureAwait(false)
                : await model.CheckActionAsync(request.ActionSymbolId, inputs, options, cancellationToken).ConfigureAwait(false);
            return new ScenarioAnswer.Answered(Rendering.CheckResult(result));
        }
        catch (CheckException failure)
        {
            var response = Rendering.CheckResult(failure.Partial);
            response.Error = failure.Message;
            return new ScenarioAnswer.Answered(response);
        }
    }

    // A field the request leaves empty is an option the API leaves null; the runner has a RunOptions of its own.
    private static OpenSysML.RunOptions Options(string schedule, string performer, bool trace, double? advance, ulong? seed, string draws, double clockStep, IReadOnlyList<string>? events = null) =>
        new()
        {
            Schedule = schedule.Length > 0 ? schedule : null,
            PerformerSymbolId = performer.Length > 0 ? performer : null,
            Trace = trace,
            Advance = advance,
            Seed = seed,
            Draws = draws.Length > 0 ? draws : null,
            ClockStep = clockStep,
            Events = events ?? [],
        };
}
