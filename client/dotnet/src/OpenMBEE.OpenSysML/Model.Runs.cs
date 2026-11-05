using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string RunRunsMethod = "sysml.SysMLService/RunRuns";
    internal const string SelfCheckMethod = "sysml.SysMLService/SelfCheck";
    internal const string RecordRunMethod = "sysml.SysMLService/RecordRun";

    /// <summary>
    /// Runs an action a number of times and tables what the runs observed, as <c>sysml -runs</c> does: every run is an
    /// ordinary run in a context of its own, drawing its modeled randomness from a seed derived from
    /// <see cref="ActionRunsOptions.Seed"/>, so the same seed makes the same runs. A run that fails is that row's error,
    /// never an exception.
    /// </summary>
    /// <param name="actionSymbolId">The qualified name of the action definition or usage.</param>
    /// <param name="runs">How many runs to make; the service refuses a count that is not positive.</param>
    /// <param name="inputs">Values for the action's <c>in</c> parameters, by name, as every run binds them; null for none.</param>
    /// <param name="options">The performer, the observables, the seed, the draws, the clock step and the engine; null for <see cref="ActionRunsOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One row per run, in run order, and each observable's distribution over the completed runs.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="actionSymbolId"/> is null.</exception>
    /// <exception cref="ArgumentException">An input's name or value is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or an input is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Runs"/>, <see cref="Capabilities.Performer"/> for a performer, a capability an input needs, <see cref="Capabilities.Engines"/> for an engine, or <see cref="Capabilities.ScheduleExplore"/> for the explore engine; nothing was sent.</exception>
    /// <exception cref="RunsException">The runs could not be tabled: the action or the performer does not resolve, an input does not read, or no completed run produced a feature asked to be observed. <see cref="RunsException.Partial"/> keeps the rows made.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a count that is not positive, random draws without a seed, an unknown draw policy or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Runs> RunActionRunsAsync(
        string actionSymbolId,
        long runs,
        IReadOnlyDictionary<string, Value>? inputs = null,
        ActionRunsOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(actionSymbolId);
        Connection.ThrowIfDisposed();
        options ??= ActionRunsOptions.Default;
        if (options.PerformerSymbolId is { Length: 0 } || options.Draws is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("ActionRunsOptions names an empty performer, draw policy or engine; leave PerformerSymbolId, Draws or Engine null for the default.");
        }

        var sent = Inputs(inputs);
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.Runs);
        if (options.PerformerSymbolId is not null)
        {
            capabilities.Require(Capabilities.Performer);
        }

        Fit(sent);
        Engines.Require(options.Engine, capabilities);
        var request = new Wire.RunRunsRequest
        {
            ModelHash = Hash,
            ActionSymbolId = actionSymbolId,
            PerformerSymbolId = options.PerformerSymbolId ?? string.Empty,
            Runs = runs,
            Draws = options.Draws ?? string.Empty,
            ClockStep = options.ClockStep,
            Observe = { options.Observe },
            Engine = options.Engine ?? string.Empty,
        };
        AddInputs(request.Inputs, sent);
        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        return await RunsAsync(request, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// Runs a <c>Simulation::MonteCarlo</c> analysis case a number of times, as <c>sysml -runs</c> does: each run reports
    /// the value the case declares it observes, drawing its modeled randomness from a seed derived from
    /// <see cref="AnalysisRunsOptions.Seed"/>, and the case is concluded over the sample, which settles every row's
    /// verdicts. A run that fails is that row's error, never an exception.
    /// </summary>
    /// <param name="analysisSymbolId">The qualified name of the Monte Carlo analysis case definition or usage.</param>
    /// <param name="runs">How many runs to make; the service refuses a count that is not positive.</param>
    /// <param name="options">The subject, the arguments, the seed, the draws, the clock step and the engine; null for <see cref="AnalysisRunsOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One row per run, in run order, the observable's distribution, and the case's conclusion or why it has none.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="analysisSymbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or an argument is a value no service reads as one; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Runs"/>, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, or a capability an argument needs; nothing was sent.</exception>
    /// <exception cref="RunsException">No run was made: the case does not resolve, is not a Monte Carlo case, or its subject or an argument does not read. <see cref="RunsException.Partial"/> keeps the standing of the engine that refused.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a count that is not positive, random draws without a seed, an unknown draw policy or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Runs> RunAnalysisRunsAsync(string analysisSymbolId, long runs, AnalysisRunsOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(analysisSymbolId);
        Connection.ThrowIfDisposed();
        options ??= AnalysisRunsOptions.Default;
        if (options.SubjectSymbolId is { Length: 0 } || options.Draws is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("AnalysisRunsOptions names an empty subject, draw policy or engine; leave SubjectSymbolId, Draws or Engine null for the default.");
        }

        var positional = ValueWire.ToWire(new ValueList<Value>(options.Arguments), "arguments");
        var named = ValueWire.ToWire(new ValueDictionary<Value>(options.NamedArguments), "named_arguments");
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.Runs);
        Engines.Require(options.Engine, capabilities);
        Fit(positional);
        Fit(named);
        var request = new Wire.RunRunsRequest
        {
            ModelHash = Hash,
            AnalysisSymbolId = analysisSymbolId,
            SubjectSymbolId = options.SubjectSymbolId ?? string.Empty,
            Arguments = { positional },
            Runs = runs,
            Draws = options.Draws ?? string.Empty,
            ClockStep = options.ClockStep,
            Engine = options.Engine ?? string.Empty,
        };
        AddInputs(request.NamedArguments, named);
        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        return await RunsAsync(request, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// Applies the <c>SysMLValidation</c> constraints, and those of any rule packages, reflectively to every element of
    /// the model, as <c>sysml -self-check</c> does. A violated constraint is a finding, never an exception.
    /// </summary>
    /// <param name="rulePackages">
    /// The qualified names of packages whose constraint definitions apply after the bundled ones
    /// (<c>sysml -self-check-package</c>), each a package the model or its libraries declare; null for none.
    /// </param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Every finding, the counts, and the verdict over them.</returns>
    /// <exception cref="ArgumentException">A package name is null.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.SelfCheck"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The check could not start: a rule package named is not declared.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<SelfCheck> SelfCheckAsync(IEnumerable<string>? rulePackages = null, CancellationToken cancellationToken = default)
    {
        var packages = new ValueList<string>(rulePackages ?? []);
        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.SelfCheck);
        var request = new Wire.SelfCheckRequest { ModelHash = Hash, SelfCheckPackage = { packages } };
        var response = await Connection.ModelCallAsync<Wire.SelfCheckRequest, Wire.SelfCheckResponse>(SelfCheckMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            Unanswered(
                SelfCheckMethod,
                ("status", response.Status.Length > 0),
                ("findings", response.Findings.Count > 0),
                ("elements", response.Elements != 0),
                ("checks", response.Checks != 0),
                ("violations", response.Violations != 0),
                ("unevaluated", response.Unevaluated != 0),
                ("evaluation_errors", response.EvaluationErrors != 0),
                ("applications", response.Applications.Count > 0),
                ("summary", response.Summary.Length > 0));
            throw new ModelException(response.Error, SelfCheckMethod, ModelWire.Diagnostics(response.Diagnostics));
        }

        if (response.Status.Length == 0)
        {
            throw new TransportException(StatusCode.Internal, $"{SelfCheckMethod} answered neither a status nor a failure.", SelfCheckMethod);
        }

        return RunsWire.SelfCheck(response);
    }

    /// <summary>
    /// Runs an analysis case and records the runs into the model as <c>AnalysisRecords</c> declarations, as
    /// <c>sysml -record-run</c> does: once, over the rows of a sweep, or as a Monte Carlo sample, by
    /// <see cref="RecordOptions"/>. The records make a new model the service holds, which
    /// <see cref="Recording.Model"/> names; this model is left as it was, and nothing is written to disk.
    /// </summary>
    /// <param name="analysisSymbolId">The qualified name of the analysis case definition or usage.</param>
    /// <param name="options">The subject, the arguments, the sweep or sample, the engine and the package the records go into; null for <see cref="RecordOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The run, sweep or sample made, the records generated, the notation they went into, and the recorded model.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="analysisSymbolId"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or a value is one no service reads as an argument; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.RecordRun"/>, <see cref="Capabilities.Engines"/> for an engine, <see cref="Capabilities.ScheduleExplore"/> for the explore engine, or a capability a value needs; nothing was sent.</exception>
    /// <exception cref="RecordException">Nothing was recorded: the case could not be found or run, no run completed, the package could not take the records, or they would not analyze cleanly.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does both ranges and runs, a negative number of runs, runs without a seed or an engine it does not know.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Recording> RecordRunAsync(string analysisSymbolId, RecordOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(analysisSymbolId);
        Connection.ThrowIfDisposed();
        options ??= RecordOptions.Default;
        if (options.SubjectSymbolId is { Length: 0 } || options.Engine is { Length: 0 } || options.RecordInto is { Length: 0 })
        {
            throw new InvalidRequestException("RecordOptions names an empty subject, engine or package; leave SubjectSymbolId, Engine or RecordInto null for the default.");
        }

        var positional = ValueWire.ToWire(new ValueList<Value>(options.Arguments), "arguments");
        var named = ValueWire.ToWire(new ValueDictionary<Value>(options.NamedArguments), "named_arguments");
        var sentRanges = options.Ranges.Select((range, i) => Range(range, $"ranges[{i}]")).ToList();
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.RecordRun);
        Engines.Require(options.Engine, capabilities);
        Fit(positional);
        Fit(named);
        foreach (var range in sentRanges)
        {
            ValueWire.Fit(range.Start, capabilities);
            ValueWire.Fit(range.End, capabilities);
            if (range.Step is { } step)
            {
                ValueWire.Fit(step, capabilities);
            }
        }

        // The recording is read whole, so the request always accepts the documents a model of several answers.
        var request = new Wire.RecordRunRequest
        {
            ModelHash = Hash,
            AnalysisSymbolId = analysisSymbolId,
            SubjectSymbolId = options.SubjectSymbolId ?? string.Empty,
            Arguments = { positional },
            Engine = options.Engine ?? string.Empty,
            RecordInto = options.RecordInto ?? string.Empty,
            Ranges = { sentRanges },
            Runs = options.Runs,
            AcceptDocuments = true,
        };
        AddInputs(request.NamedArguments, named);
        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        var response = await Connection.ModelCallAsync<Wire.RecordRunRequest, Wire.RecordRunResponse>(RecordRunMethod, request, cancellationToken).ConfigureAwait(false);
        var (run, sweep, sample, ranError, ranReason) = Ran(response, options.Mode);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            Unanswered(RecordRunMethod, ("content", response.Content.Length > 0), ("documents", response.Documents.Count > 0), ("model_hash", response.ModelHash.Length > 0));
            if (ranError.Length > 0 && (ranError != response.Error || ranReason != response.FailureReason))
            {
                throw new TransportException(StatusCode.Internal, $"{RecordRunMethod} answered a failure of its runs that is not the failure it reported.", RecordRunMethod);
            }

            throw new RecordException(
                response.Error,
                RecordRunMethod,
                VerificationWire.Reason(response.FailureReason),
                diagnostics,
                run,
                sweep,
                sample,
                ranError.Length > 0,
                response.Package,
                response.Definition,
                response.Records,
                response.Source,
                response.Skipped,
                response.Reasons);
        }

        if (ranError.Length > 0)
        {
            throw new TransportException(StatusCode.Internal, $"{RecordRunMethod} answered a recording beside a failure of the runs it recorded.", RecordRunMethod);
        }

        if (response.ModelHash.Length == 0 || (run is null && sweep is null && sample is null))
        {
            throw new TransportException(StatusCode.Internal, $"{RecordRunMethod} answered neither a recorded model with the runs it recorded nor a failure.", RecordRunMethod);
        }

        return new Recording(
            run,
            sweep,
            sample,
            response.Package,
            response.Definition,
            response.Records,
            response.Source,
            response.Content,
            RunsWire.Documents(response.Documents),
            response.Skipped,
            response.Reasons,
            diagnostics,
            new Model(Connection, response.ModelHash, RecordRunMethod, default, default, default, null, default));
    }

    // Sends runs and reads their table; a failure the answer reports is raised with the table made beside it.
    private async Task<Runs> RunsAsync(Wire.RunRunsRequest request, CancellationToken cancellationToken)
    {
        var response = await Connection.ModelCallAsync<Wire.RunRunsRequest, Wire.RunRunsResponse>(RunRunsMethod, request, cancellationToken).ConfigureAwait(false);
        var read = RunsWire.Runs(response, RunRunsMethod);
        if (response.Error.Length > 0)
        {
            throw new RunsException(response.Error, RunRunsMethod, VerificationWire.Reason(response.FailureReason), read);
        }

        if (read.Rows.IsEmpty)
        {
            throw new TransportException(StatusCode.Internal, $"{RunRunsMethod} answered neither a run nor a failure.", RunRunsMethod);
        }

        return read;
    }

    // The runs a recording reports, of the one kind the request asked for, with the failure they report themselves.
    private static (Analysis? Run, Sweep? Sweep, Runs? Sample, string Error, Wire.FailureReason Reason) Ran(Wire.RecordRunResponse response, RecordMode mode)
    {
        var answered = new (RecordMode Mode, bool Set)[] { (RecordMode.Run, response.Run is not null), (RecordMode.Sweep, response.Sweep is not null), (RecordMode.Sample, response.Sample is not null) }
            .Where(answer => answer.Set)
            .Select(answer => answer.Mode)
            .ToList();
        if (answered.Count > 1 || answered.Count == 1 && answered[0] != mode)
        {
            throw new TransportException(
                StatusCode.Internal,
                $"{RecordRunMethod} answered a recording made by {string.Join(" and ", answered).ToLowerInvariant()}, which a request for {mode.ToString().ToLowerInvariant()} does not make.",
                RecordRunMethod);
        }

        if (response.Run is { } run)
        {
            if (run.Outcomes.Count > 0 || run.Exploration is not null)
            {
                throw new TransportException(StatusCode.Internal, $"{RecordRunMethod} answered a recorded run with an exploration's outcomes or status.", RecordRunMethod);
            }

            return (VerificationWire.Analysis(run, RecordRunMethod, "run"), null, null, run.Error, run.FailureReason);
        }

        if (response.Sweep is { } sweep)
        {
            return (null, VerificationWire.Sweep(sweep, RecordRunMethod, "sweep"), null, sweep.Error, sweep.FailureReason);
        }

        if (response.Sample is { } sample)
        {
            return (null, null, RunsWire.Runs(sample, RecordRunMethod, "sample"), sample.Error, sample.FailureReason);
        }

        return (null, null, null, string.Empty, Wire.FailureReason.Unspecified);
    }
}
