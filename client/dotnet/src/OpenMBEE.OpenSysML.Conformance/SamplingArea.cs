using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Running behaviors repeatedly, self-checking and recording runs through <see cref="Model"/>'s calls. RunRuns is
/// answered by the action or the Monte Carlo case call, chosen by which the request names. Runs the service could not
/// table reach the caller as a <see cref="RunsException"/> keeping the table made, a self-check that could not start as
/// a <see cref="ModelException"/>, and a recording not made as a <see cref="RecordException"/> keeping the runs; each
/// is rendered back in band with what it kept of the answer.
/// </summary>
internal static class SamplingArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.RunRunsRequest>(
            "RunRuns",
            [
                "model_hash", "action_symbol_id", "analysis_symbol_id", "inputs", "performer_symbol_id", "subject_symbol_id", "arguments", "named_arguments",
                "runs", "seed", "draws", "clock_step", "observe", "engine",
            ],
            RunRunsAsync),
        RpcAdapter.Of<Wire.SelfCheckRequest>("SelfCheck", ["model_hash", "self_check_package"], SelfCheckAsync),
        RpcAdapter.Of<Wire.RecordRunRequest>(
            "RecordRun",
            ["model_hash", "analysis_symbol_id", "subject_symbol_id", "arguments", "named_arguments", "engine", "record_into", "ranges", "runs", "seed", "accept_documents"],
            RecordRunAsync),
    ];

    private static async Task<ScenarioAnswer> RunRunsAsync(Connection connection, Wire.RunRunsRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        ulong? seed = request.HasSeed ? request.Seed : null;
        try
        {
            if (request.AnalysisSymbolId.Length == 0)
            {
                if (request.SubjectSymbolId.Length > 0 || request.Arguments.Count > 0 || request.NamedArguments.Count > 0)
                {
                    throw new UnexpressibleRequestException("the public API binds a subject and arguments only to a Monte Carlo case it runs, so it cannot send them with an action");
                }

                var options = new ActionRunsOptions
                {
                    PerformerSymbolId = Named(request.PerformerSymbolId),
                    Observe = request.Observe,
                    Seed = seed,
                    Draws = Named(request.Draws),
                    ClockStep = request.ClockStep,
                    Engine = Named(request.Engine),
                };
                var inputs = RequestValues.Map(request.Inputs, "inputs");
                return new ScenarioAnswer.Answered(Rendering.Runs(await model.RunActionRunsAsync(request.ActionSymbolId, request.Runs, inputs, options, cancellationToken).ConfigureAwait(false)));
            }

            if (request.ActionSymbolId.Length > 0)
            {
                throw new UnexpressibleRequestException("the public API runs one behavior, so it cannot name both an action and a Monte Carlo case");
            }

            if (request.Inputs.Count > 0 || request.PerformerSymbolId.Length > 0 || request.Observe.Count > 0)
            {
                throw new UnexpressibleRequestException("the public API binds inputs, a performer and observables only to an action it runs, so it cannot send them with a Monte Carlo case");
            }

            var caseOptions = new AnalysisRunsOptions
            {
                SubjectSymbolId = Named(request.SubjectSymbolId),
                Arguments = RequestValues.List(request.Arguments, "arguments"),
                NamedArguments = RequestValues.Map(request.NamedArguments, "named_arguments") ?? [],
                Seed = seed,
                Draws = Named(request.Draws),
                ClockStep = request.ClockStep,
                Engine = Named(request.Engine),
            };
            return new ScenarioAnswer.Answered(Rendering.Runs(await model.RunAnalysisRunsAsync(request.AnalysisSymbolId, request.Runs, caseOptions, cancellationToken).ConfigureAwait(false)));
        }
        catch (RunsException failure)
        {
            var response = Rendering.Runs(failure.Partial);
            response.Error = failure.Message;
            response.FailureReason = Rendering.Reason(failure.Reason);
            return new ScenarioAnswer.Answered(response);
        }
    }

    private static async Task<ScenarioAnswer> SelfCheckAsync(Connection connection, Wire.SelfCheckRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        try
        {
            return new ScenarioAnswer.Answered(Rendering.SelfCheck(await model.SelfCheckAsync(request.SelfCheckPackage, cancellationToken).ConfigureAwait(false)));
        }
        catch (ModelException failure)
        {
            return new ScenarioAnswer.Answered(new Wire.SelfCheckResponse { Error = failure.Message, Diagnostics = { Rendering.Diagnostics(failure.Diagnostics) } });
        }
    }

    private static async Task<ScenarioAnswer> RecordRunAsync(Connection connection, Wire.RecordRunRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (!request.AcceptDocuments)
        {
            throw new UnexpressibleRequestException("the public API reads every document a recording answers, so it always accepts them");
        }

        var options = new RecordOptions
        {
            SubjectSymbolId = Named(request.SubjectSymbolId),
            Arguments = RequestValues.List(request.Arguments, "arguments"),
            NamedArguments = RequestValues.Map(request.NamedArguments, "named_arguments") ?? [],
            Engine = Named(request.Engine),
            RecordInto = Named(request.RecordInto),
            Ranges = request.Ranges.Select((range, i) => VerificationArea.Range(range, $"ranges[{i}]")).ToList(),
            Runs = request.Runs,
            Seed = request.HasSeed ? request.Seed : null,
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.Recording(await model.RecordRunAsync(request.AnalysisSymbolId, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (RecordException failure)
        {
            return new ScenarioAnswer.Answered(Rendering.Recording(failure));
        }
    }

    // A field the request leaves empty is an option the API leaves null.
    private static string? Named(string value) => value.Length > 0 ? value : null;
}
