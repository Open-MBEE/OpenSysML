using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string EvaluateCalcMethod = "sysml.SysMLService/EvaluateCalc";
    internal const string RunAnalysisMethod = "sysml.SysMLService/RunAnalysis";
    internal const string RunSweepMethod = "sysml.SysMLService/RunSweep";

    /// <summary>
    /// Invokes a calculation, as the REPL's <c>%calc</c> does: with its arguments bound positionally, or, for a calc
    /// usage named with none, from its own members, reporting every output feature it computes.
    /// </summary>
    /// <param name="symbolId">The qualified name of the calc definition or usage.</param>
    /// <param name="arguments">Values for its <c>in</c> parameters, in declaration order; null for none. A calculation takes no argument by name.</param>
    /// <param name="options">The engine; null for <see cref="CalcOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The value an invocation returned, or the output features a calc usage computed.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An argument is null.</exception>
    /// <exception cref="InvalidRequestException">The engine is an empty string, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="VerificationException">The calculation could not be evaluated: the symbol does not resolve or names no calc (<see cref="FailureReason.WrongKind"/>), an argument does not read, the evaluation failed.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Calculation> CalcAsync(string symbolId, IEnumerable<Value>? arguments = null, CalcOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        var bound = new ValueList<Value>(arguments ?? []);
        Connection.ThrowIfDisposed();
        options ??= CalcOptions.Default;
        RefuseEmptyEngine(options.Engine, nameof(CalcOptions));
        var sent = ValueWire.ToWire(bound, "arguments");
        Connection.Capabilities.Require(Capabilities.Verification);
        Engines.Require(options.Engine, Connection.Capabilities);
        Fit(sent);
        var request = new Wire.EvaluateCalcRequest { ModelHash = Hash, SymbolId = symbolId, Arguments = { sent }, Engine = options.Engine ?? string.Empty };
        var response = await Connection.ModelCallAsync<Wire.EvaluateCalcRequest, Wire.EvaluateCalcResponse>(EvaluateCalcMethod, request, cancellationToken).ConfigureAwait(false);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        var standing = ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds);
        if (response.Error.Length > 0)
        {
            Unanswered(EvaluateCalcMethod, ("result", response.Result is not null), ("outputs", response.Outputs.Count > 0));
            throw new VerificationException(response.Error, EvaluateCalcMethod, diagnostics, VerificationWire.Reason(response.FailureReason), [], [], standing);
        }

        if (response.Result is null && response.Outputs.Count == 0)
        {
            throw new TransportException(StatusCode.Internal, $"{EvaluateCalcMethod} answered neither a value nor a failure for {symbolId}.", EvaluateCalcMethod);
        }

        return new Calculation(
            response.Result is { } result ? ValueWire.FromWire(result, EvaluateCalcMethod, "result") : null,
            VerificationWire.Outputs(response.Outputs, EvaluateCalcMethod, "outputs"),
            diagnostics,
            standing);
    }

    /// <summary>
    /// Runs an analysis case once, as the REPL's <c>%analysis</c> does: its subject and <c>in</c> parameters are bound,
    /// its body runs, and every output it declares is reported with the verdict of its objective and of each assertion.
    /// </summary>
    /// <param name="symbolId">The qualified name of the analysis or verification case definition or usage.</param>
    /// <param name="options">The subject, arguments, schedule, engine and tool dry run; null for <see cref="AnalysisOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>What the run computed and decided; a verdict of false is a verdict, not an exception.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, the schedule or the engine explores, which <see cref="ExploreAnalysisAsync"/> answers, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> for a schedule, <see cref="Capabilities.Verification"/>, <see cref="Capabilities.Engines"/> for an engine, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="AnalysisException">The case could not be run to its end; <see cref="AnalysisException.Partial"/> keeps what it computed first.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a schedule or an engine it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Analysis> RunAnalysisAsync(string symbolId, AnalysisOptions? options = null, CancellationToken cancellationToken = default)
    {
        var response = await AnalysisAsync(symbolId, options, explore: false, cancellationToken).ConfigureAwait(false);
        return VerificationWire.Analysis(response, RunAnalysisMethod);
    }

    /// <summary>
    /// Runs an analysis case once per order of its choice points the library admits, within a budget; runs agreeing on
    /// the case's outputs and verdicts are one <see cref="Outcome"/>, whose outputs name each verdict as
    /// <c>objective &lt;name&gt;</c>, <c>assertion &lt;name&gt;</c> or <c>verdict &lt;case&gt;</c>.
    /// </summary>
    /// <param name="symbolId">The qualified name of the analysis case definition or usage.</param>
    /// <param name="options">
    /// The subject and arguments, made anew for each run, and the exploring schedule, <see cref="RunOptions.Explore"/>
    /// when neither it nor the <c>explore</c> engine is named; null for <see cref="AnalysisOptions.Default"/>.
    /// </param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Every distinct outcome reached, how the exploration ended, and the standing of the engine that answered.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, the schedule runs once, which <see cref="RunAnalysisAsync"/> answers, a tool dry run is asked for, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Schedule"/> or <see cref="Capabilities.ScheduleExplore"/> for the schedule, <see cref="Capabilities.Verification"/>, <see cref="Capabilities.Engines"/> for an engine, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="AnalysisException">The case could not be explored at all.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a budget or an engine it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Exploration> ExploreAnalysisAsync(string symbolId, AnalysisOptions? options = null, CancellationToken cancellationToken = default)
    {
        var response = await AnalysisAsync(symbolId, options, explore: true, cancellationToken).ConfigureAwait(false);
        return ExecutionWire.Exploration(
            response.Outcomes,
            response.Exploration,
            ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds),
            RunAnalysisMethod);
    }

    /// <summary>
    /// Runs an analysis case or a calc once per row of a parameter sweep, as <c>sysml -sweep</c> does: every row is an
    /// ordinary run with the swept parameters bound to the row's values. A run that fails is that row's error, beside
    /// what it computed first, never an exception.
    /// </summary>
    /// <param name="symbolId">The qualified name of the analysis case or calc, definition or usage.</param>
    /// <param name="ranges">The swept parameters, several of which make one row per point of their cartesian product, the first varying slowest.</param>
    /// <param name="options">The subject, the arguments every row binds, the sampling and the engine; null for <see cref="SweepOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The table, one row per run, in the order the runs were made.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> or <paramref name="ranges"/> is null.</exception>
    /// <exception cref="ArgumentException">A range or an argument is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or a value is one no service reads as an argument; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Verification"/>, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, or a capability a value needs; nothing was sent.</exception>
    /// <exception cref="VerificationException">No run was made: the symbol does not resolve or is neither a case nor a calc (<see cref="FailureReason.WrongKind"/>), a range names no parameter of the target or no values follow from it, the samples are refused.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Sweep> RunSweepAsync(string symbolId, IEnumerable<SweepRange> ranges, SweepOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        var swept = new ValueList<SweepRange>(ranges);
        Connection.ThrowIfDisposed();
        options ??= SweepOptions.Default;
        if (options.SubjectSymbolId is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("SweepOptions names an empty subject or engine; leave SubjectSymbolId or Engine null to name none.");
        }

        var positional = ValueWire.ToWire(new ValueList<Value>(options.Arguments), "arguments");
        var named = ValueWire.ToWire(new ValueDictionary<Value>(options.NamedArguments), "named_arguments");
        var sentRanges = swept.Select((range, i) => Range(range, $"ranges[{i}]")).ToList();
        Connection.Capabilities.Require(Capabilities.Verification);
        Engines.Require(options.Engine, Connection.Capabilities);
        Fit(positional);
        Fit(named);
        foreach (var range in sentRanges)
        {
            ValueWire.Fit(range.Start, Connection.Capabilities);
            ValueWire.Fit(range.End, Connection.Capabilities);
            if (range.Step is { } step)
            {
                ValueWire.Fit(step, Connection.Capabilities);
            }
        }

        var request = new Wire.RunSweepRequest
        {
            ModelHash = Hash,
            SymbolId = symbolId,
            SubjectSymbolId = options.SubjectSymbolId ?? string.Empty,
            Arguments = { positional },
            Ranges = { sentRanges },
            Samples = options.Samples,
            Seed = options.Seed,
            Engine = options.Engine ?? string.Empty,
        };
        AddInputs(request.NamedArguments, named);
        var response = await Connection.ModelCallAsync<Wire.RunSweepRequest, Wire.RunSweepResponse>(RunSweepMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            Unanswered(
                RunSweepMethod,
                ("rows", response.Rows.Count > 0),
                ("parameters", response.Parameters.Count > 0),
                ("sampled", response.Sampled),
                ("seed", response.Seed != 0));
            throw new VerificationException(
                response.Error,
                RunSweepMethod,
                ModelWire.Diagnostics(response.Diagnostics),
                VerificationWire.Reason(response.FailureReason),
                VerificationWire.Instances(response.Instances, RunSweepMethod, "instances"),
                [],
                ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds));
        }

        return VerificationWire.Sweep(response, RunSweepMethod);
    }

    private static Wire.SweepRange Range(SweepRange range, string path)
    {
        var sent = new Wire.SweepRange
        {
            Parameter = range.Parameter,
            Start = ValueWire.ToWire(range.Start, path + ".start"),
            End = ValueWire.ToWire(range.End, path + ".end"),
        };
        if (range.Step is { } step)
        {
            sent.Step = ValueWire.ToWire(step, path + ".step");
        }

        return sent;
    }

    // Checks a case run's options in the order the service does, sends it, and holds its answer to the kind asked for.
    private async Task<Wire.RunAnalysisResponse> AnalysisAsync(string symbolId, AnalysisOptions? options, bool explore, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        Connection.ThrowIfDisposed();
        options ??= AnalysisOptions.Default;
        if (options.SubjectSymbolId is { Length: 0 } || options.Schedule is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("AnalysisOptions names an empty subject, schedule or engine; leave SubjectSymbolId, Schedule or Engine null to name none.");
        }

        var schedule = AnalysisSchedule(options, explore);
        var positional = ValueWire.ToWire(new ValueList<Value>(options.Arguments), "arguments");
        var named = ValueWire.ToWire(new ValueDictionary<Value>(options.NamedArguments), "named_arguments");
        var capabilities = Connection.Capabilities;
        if (schedule is not null)
        {
            capabilities.Require(Capabilities.Schedule);
            if (RunOptions.IsExploring(schedule))
            {
                capabilities.Require(Capabilities.ScheduleExplore);
            }
        }

        capabilities.Require(Capabilities.Verification);
        Engines.Require(options.Engine, capabilities);
        Fit(positional);
        Fit(named);
        var request = new Wire.RunAnalysisRequest
        {
            ModelHash = Hash,
            SymbolId = symbolId,
            SubjectSymbolId = options.SubjectSymbolId ?? string.Empty,
            Arguments = { positional },
            Schedule = schedule ?? string.Empty,
            Engine = options.Engine ?? string.Empty,
            ToolDryRun = options.ToolDryRun,
        };
        AddInputs(request.NamedArguments, named);
        var response = await Connection.ModelCallAsync<Wire.RunAnalysisRequest, Wire.RunAnalysisResponse>(RunAnalysisMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            Unanswered(RunAnalysisMethod, ("outcomes", response.Outcomes.Count > 0), ("exploration", response.Exploration is not null));
            throw new AnalysisException(
                response.Error, RunAnalysisMethod, VerificationWire.Reason(response.FailureReason), VerificationWire.Analysis(response, RunAnalysisMethod));
        }

        RequireShape(
            RunAnalysisMethod,
            explore,
            response.Exploration,
            response.Outcomes.Count,
            ("outputs", response.Outputs.Count > 0),
            ("verdicts", response.Verdicts.Count > 0),
            ("instances", response.Instances.Count > 0),
            ("diagnostics", response.Diagnostics.Count > 0),
            ("verification_verdicts", response.VerificationVerdicts.Count > 0),
            ("evaluations", response.Evaluations.Count > 0),
            ("tool_dry_run", response.ToolDryRun is not null));
        return response;
    }

    // The schedule a case run sends: the one named, or, for an exploration that neither names one nor the explore
    // engine, explore. A run and an exploration answer differently, so each call takes only its own kind.
    private static string? AnalysisSchedule(AnalysisOptions options, bool explore)
    {
        var exploring = options.Schedule is { } named && RunOptions.IsExploring(named);
        if (!explore && (exploring || options.EngineExplores))
        {
            throw new InvalidRequestException(
                $"AnalysisOptions explores every order (schedule {options.Schedule ?? "unset"}, engine {options.Engine ?? "unset"}), which {nameof(ExploreAnalysisAsync)} answers; a run takes one order.");
        }

        if (explore && options.Schedule is { } once && !exploring)
        {
            throw new InvalidRequestException(
                $"The schedule {once} runs once, which {nameof(RunAnalysisAsync)} answers; an exploration takes {RunOptions.Explore} or {RunOptions.Explore}:runs=<n>,depth=<d>.");
        }

        if (explore && options.ToolDryRun)
        {
            throw new InvalidRequestException("AnalysisOptions.ToolDryRun previews one run of the case, which an exploration makes none of; leave it false.");
        }

        return explore && options.Schedule is null && !options.EngineExplores ? RunOptions.Explore : options.Schedule;
    }
}
