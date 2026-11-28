using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Verifying, validating, calculating, analyzing and sweeping through <see cref="Model"/>'s calls, and listing the
/// engines through <see cref="Connection.ListEnginesAsync"/>. RunAnalysis is answered by the run or the explore call,
/// chosen by whether the request's schedule or engine explores. A request the service could not answer at all reaches
/// the caller as a <see cref="VerificationException"/>, an analysis that failed as an <see cref="AnalysisException"/>
/// keeping its partial run; each is rendered back in band with what it kept of the answer.
/// </summary>
internal static class VerificationArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.VerifyConstraintRequest>(
            "VerifyConstraint",
            ["model_hash", "symbol_id", "subject_symbol_id", "engine", "question", "arguments", "named_arguments"],
            VerifyConstraintAsync),
        RpcAdapter.Of<Wire.VerifyRequirementRequest>(
            "VerifyRequirement",
            ["model_hash", "symbol_id", "subject_symbol_id", "engine", "question", "arguments", "named_arguments"],
            VerifyRequirementAsync),
        RpcAdapter.Of<Wire.VerifySatisfactionRequest>("VerifySatisfaction", ["model_hash", "symbol_id", "engine", "question"], VerifySatisfactionAsync),
        RpcAdapter.Of<Wire.ValidateInstanceRequest>("ValidateInstance", ["model_hash", "symbol_id", "engine"], ValidateInstanceAsync),
        RpcAdapter.Of<Wire.EvaluateCalcRequest>("EvaluateCalc", ["model_hash", "symbol_id", "arguments", "engine"], EvaluateCalcAsync),
        RpcAdapter.Of<Wire.RunAnalysisRequest>(
            "RunAnalysis",
            ["model_hash", "symbol_id", "subject_symbol_id", "arguments", "named_arguments", "schedule", "engine", "tool_dry_run"],
            RunAnalysisAsync),
        RpcAdapter.Of<Wire.RunSweepRequest>(
            "RunSweep",
            ["model_hash", "symbol_id", "subject_symbol_id", "arguments", "named_arguments", "ranges", "samples", "seed", "engine"],
            RunSweepAsync),
        RpcAdapter.Of<Wire.ListEnginesRequest>("ListEngines", ["probe"], ListEnginesAsync),
    ];

    private static async Task<ScenarioAnswer> VerifyConstraintAsync(Connection connection, Wire.VerifyConstraintRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = Options(request.SubjectSymbolId, request.Question, request.Engine, request.Arguments, request.NamedArguments);
        var response = new Wire.VerifyConstraintResponse();
        try
        {
            var verification = await model.VerifyConstraintAsync(request.SymbolId, options, cancellationToken).ConfigureAwait(false);
            response.Verdict = Rendering.Verdict(verification.Verdict);
            response.Instances.AddRange(Rendering.Instances(verification.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(verification.Diagnostics));
        }
        catch (VerificationException failure)
        {
            response.Error = failure.Message;
            response.Instances.AddRange(Rendering.Instances(failure.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> VerifyRequirementAsync(Connection connection, Wire.VerifyRequirementRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = Options(request.SubjectSymbolId, request.Question, request.Engine, request.Arguments, request.NamedArguments);
        var response = new Wire.VerifyRequirementResponse();
        try
        {
            var verification = await model.VerifyRequirementAsync(request.SymbolId, options, cancellationToken).ConfigureAwait(false);
            response.Verdict = Rendering.Verdict(verification.Verdict);
            response.Instances.AddRange(Rendering.Instances(verification.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(verification.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(verification.Verifications));
        }
        catch (VerificationException failure)
        {
            response.Error = failure.Message;
            response.Instances.AddRange(Rendering.Instances(failure.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(failure.Verifications));
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> VerifySatisfactionAsync(Connection connection, Wire.VerifySatisfactionRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = new VerifyOptions { Question = Named(request.Question), Engine = Named(request.Engine) };
        var response = new Wire.VerifySatisfactionResponse();
        try
        {
            var satisfaction = await model.VerifySatisfactionAsync(Named(request.SymbolId), options, cancellationToken).ConfigureAwait(false);
            response.Verdicts.AddRange(Rendering.Verdicts(satisfaction.Verdicts));
            response.Instances.AddRange(Rendering.Instances(satisfaction.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(satisfaction.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(satisfaction.Verifications));
        }
        catch (VerificationException failure)
        {
            response.Error = failure.Message;
            response.FailureReason = Rendering.Reason(failure.Reason);
            response.Instances.AddRange(Rendering.Instances(failure.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(failure.Verifications));
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> ValidateInstanceAsync(Connection connection, Wire.ValidateInstanceRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var response = new Wire.ValidateInstanceResponse();
        try
        {
            var validation = await model.ValidateInstanceAsync(request.SymbolId, new ValidateOptions { Engine = Named(request.Engine) }, cancellationToken).ConfigureAwait(false);
            response.Summary = Rendering.Verdict(validation.Summary);
            response.Verdicts.AddRange(Rendering.Verdicts(validation.Verdicts));
            response.Instances.AddRange(Rendering.Instances(validation.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(validation.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(validation.Verifications));
            response.Bounded = validation.Bounded;
        }
        catch (VerificationException failure)
        {
            response.Error = failure.Message;
            response.FailureReason = Rendering.Reason(failure.Reason);
            response.Instances.AddRange(Rendering.Instances(failure.Instances));
            response.Diagnostics.AddRange(Rendering.Diagnostics(failure.Diagnostics));
            response.VerificationVerdicts.AddRange(Rendering.Verifications(failure.Verifications));
        }

        return new ScenarioAnswer.Answered(response);
    }

    private static async Task<ScenarioAnswer> EvaluateCalcAsync(Connection connection, Wire.EvaluateCalcRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var arguments = RequestValues.List(request.Arguments, "arguments");
        try
        {
            var calculation = await model.CalcAsync(request.SymbolId, arguments, new CalcOptions { Engine = Named(request.Engine) }, cancellationToken).ConfigureAwait(false);
            return new ScenarioAnswer.Answered(Rendering.Calculation(calculation));
        }
        catch (VerificationException failure)
        {
            var response = new Wire.EvaluateCalcResponse
            {
                Error = failure.Message,
                FailureReason = Rendering.Reason(failure.Reason),
                Diagnostics = { Rendering.Diagnostics(failure.Diagnostics) },
                Engine = failure.Standing.Engine,
                Strength = failure.Standing.Strength,
                Bounds = { Rendering.Bounds(failure.Standing) },
            };
            return new ScenarioAnswer.Answered(response);
        }
    }

    private static async Task<ScenarioAnswer> RunAnalysisAsync(Connection connection, Wire.RunAnalysisRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var options = new AnalysisOptions
        {
            SubjectSymbolId = Named(request.SubjectSymbolId),
            Arguments = RequestValues.List(request.Arguments, "arguments"),
            NamedArguments = RequestValues.Map(request.NamedArguments, "named_arguments") ?? [],
            Schedule = Named(request.Schedule),
            Engine = Named(request.Engine),
            ToolDryRun = request.ToolDryRun,
        };
        try
        {
            if (options.Explores)
            {
                var exploration = await model.ExploreAnalysisAsync(request.SymbolId, options, cancellationToken).ConfigureAwait(false);
                return new ScenarioAnswer.Answered(new Wire.RunAnalysisResponse
                {
                    Outcomes = { Rendering.Outcomes(exploration) },
                    Exploration = Rendering.ExplorationStatus(exploration),
                    Engine = exploration.Standing.Engine,
                    Strength = exploration.Standing.Strength,
                    Bounds = { Rendering.Bounds(exploration.Standing) },
                });
            }

            return new ScenarioAnswer.Answered(Rendering.Analysis(await model.RunAnalysisAsync(request.SymbolId, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (AnalysisException failure)
        {
            var response = Rendering.Analysis(failure.Partial);
            response.Error = failure.Message;
            response.FailureReason = Rendering.Reason(failure.Reason);
            return new ScenarioAnswer.Answered(response);
        }
    }

    private static async Task<ScenarioAnswer> RunSweepAsync(Connection connection, Wire.RunSweepRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var ranges = request.Ranges.Select((range, i) => Range(range, $"ranges[{i}]")).ToList();
        var options = new SweepOptions
        {
            SubjectSymbolId = Named(request.SubjectSymbolId),
            Arguments = RequestValues.List(request.Arguments, "arguments"),
            NamedArguments = RequestValues.Map(request.NamedArguments, "named_arguments") ?? [],
            Samples = request.Samples,
            Seed = request.Seed,
            Engine = Named(request.Engine),
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.Sweep(await model.RunSweepAsync(request.SymbolId, ranges, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (VerificationException failure)
        {
            var response = new Wire.RunSweepResponse
            {
                Error = failure.Message,
                FailureReason = Rendering.Reason(failure.Reason),
                Instances = { Rendering.Instances(failure.Instances) },
                Diagnostics = { Rendering.Diagnostics(failure.Diagnostics) },
                Engine = failure.Standing.Engine,
                Strength = failure.Standing.Strength,
                Bounds = { Rendering.Bounds(failure.Standing) },
            };
            return new ScenarioAnswer.Answered(response);
        }
    }

    private static async Task<ScenarioAnswer> ListEnginesAsync(Connection connection, Wire.ListEnginesRequest request, CancellationToken cancellationToken)
    {
        var engines = await connection.ListEnginesAsync(new ListEnginesOptions { Probe = request.Probe }, cancellationToken).ConfigureAwait(false);
        return new ScenarioAnswer.Answered(new Wire.ListEnginesResponse { Engines = { engines.Select(Rendering.Engine) } });
    }

    private static VerifyOptions Options(
        string subject,
        string question,
        string engine,
        Google.Protobuf.Collections.RepeatedField<Wire.Value> arguments,
        Google.Protobuf.Collections.MapField<string, Wire.Value> named) =>
        new()
        {
            SubjectSymbolId = Named(subject),
            Question = Named(question),
            Engine = Named(engine),
            Arguments = RequestValues.List(arguments, "arguments"),
            NamedArguments = RequestValues.Map(named, "named_arguments") ?? [],
        };

    /// <summary>A request's sweep range, as the public API holds one: with both endpoints, which a request may leave out.</summary>
    /// <exception cref="UnexpressibleRequestException">The range leaves out an endpoint, or a value cannot be held as spelled.</exception>
    internal static SweepRange Range(Wire.SweepRange range, string path) =>
        range.Start is null || range.End is null
            ? throw new UnexpressibleRequestException($"the public API holds a sweep range with both endpoints, which {path} does not state")
            : new SweepRange(
                range.Parameter,
                RequestValues.Read(range.Start, path + ".start"),
                RequestValues.Read(range.End, path + ".end"),
                range.Step is { } step ? RequestValues.Read(step, path + ".step") : null);

    // A field the request leaves empty is an option the API leaves null.
    private static string? Named(string value) => value.Length > 0 ? value : null;
}
