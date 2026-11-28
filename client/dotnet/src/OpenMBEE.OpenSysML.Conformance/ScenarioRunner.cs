using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>How a <see cref="ScenarioRunner"/> runs and reports.</summary>
internal sealed record ScenarioRunnerOptions
{
    /// <summary>Whether to print each normalized response, the observation a scenario was judged against.</summary>
    internal bool Verbose { get; init; }

    /// <summary>Whether to leave out the GetServerInfo scenarios, as a run that withholds capabilities does.</summary>
    internal bool OmitHandshakeScenarios { get; init; }

    /// <summary>How long one parse or call may take; the reference runner allows a minute.</summary>
    internal TimeSpan CallTimeout { get; init; } = TimeSpan.FromSeconds(60);

    /// <summary>Where the protocol's totals are printed.</summary>
    internal TextWriter Output { get; init; } = TextWriter.Null;

    /// <summary>Where each scenario's outcome is printed.</summary>
    internal TextWriter ScenarioLog { get; init; } = TextWriter.Null;

    /// <summary>The clock durations and deadlines are measured by.</summary>
    internal TimeProvider TimeProvider { get; init; } = TimeProvider.System;
}

/// <summary>
/// Runs scenarios through one <see cref="IScenarioClient"/> by the rules of conformance/README.md:
/// a scenario needing a capability the service does not report is skipped, or judged by its
/// without-capability expectation when it states one; its model is parsed once per run; its
/// request is built with the placeholders resolved; and the answer is judged by status, then by
/// the normalized response.
/// </summary>
internal sealed class ScenarioRunner
{
    private const string NoCall = "-";

    private readonly IScenarioClient client;
    private readonly FixtureDirectory fixtures;
    private readonly ScenarioRunnerOptions options;
    private readonly IReadOnlyList<string> capabilities;
    private readonly Dictionary<string, string> models = new(StringComparer.Ordinal);

    /// <summary>A runner for one protocol.</summary>
    /// <param name="client">What answers the scenarios.</param>
    /// <param name="fixtures">The suite's fixtures.</param>
    /// <param name="options">How to run and report; the defaults when null.</param>
    internal ScenarioRunner(IScenarioClient client, FixtureDirectory fixtures, ScenarioRunnerOptions? options = null)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(fixtures);
        this.client = client;
        this.fixtures = fixtures;
        this.options = options ?? new ScenarioRunnerOptions();
        capabilities = [.. client.Capabilities.Order(Spelling.CodePointOrder)];
    }

    /// <summary>Runs every scenario whose id the filter matches, reporting each as it ends.</summary>
    /// <param name="scenarios">The scenarios, in suite order.</param>
    /// <param name="filter">Selects scenarios by id, anywhere in it; null runs them all.</param>
    /// <param name="cancellationToken">Cancels the run.</param>
    /// <returns>The protocol's summary.</returns>
    internal async Task<ProtocolSummary> RunAllAsync(IEnumerable<Scenario> scenarios, Regex? filter, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(scenarios);
        cancellationToken.ThrowIfCancellationRequested();
        var results = new List<ScenarioResult>();
        foreach (var scenario in scenarios)
        {
            cancellationToken.ThrowIfCancellationRequested();
            if (filter is not null && !filter.IsMatch(scenario.Id))
            {
                continue;
            }

            if (options.OmitHandshakeScenarios && scenario.Method == "GetServerInfo")
            {
                continue;
            }

            var result = await RunAsync(scenario, cancellationToken).ConfigureAwait(false);
            results.Add(result);
            Log(result);
        }

        cancellationToken.ThrowIfCancellationRequested();
        var summary = new ProtocolSummary(client.Protocol, client.Service, capabilities, results);
        var totals = summary.Totals;
        await options.Output.WriteAsync(
            $"\n[{client.Protocol}] {totals.Total} scenarios: {totals.Passed} passed, {totals.Failed} failed, "
                + $"{totals.Skipped} skipped, {totals.Errored} in error\n").ConfigureAwait(false);
        cancellationToken.ThrowIfCancellationRequested();
        return summary;
    }

    /// <summary>Runs one scenario: parses the model it names, makes the call and judges the answer.</summary>
    /// <param name="scenario">The scenario.</param>
    /// <param name="cancellationToken">Cancels the run; it is thrown, not recorded as an outcome.</param>
    /// <returns>Its result.</returns>
    internal async Task<ScenarioResult> RunAsync(Scenario scenario, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(scenario);
        cancellationToken.ThrowIfCancellationRequested();
        var started = options.TimeProvider.GetTimestamp();
        ScenarioResult result;
        try
        {
            result = await JudgeAsync(scenario, cancellationToken).ConfigureAwait(false);
        }
        catch (OperationCanceledException e) when (cancellationToken.IsCancellationRequested)
        {
            throw new OperationCanceledException("The conformance run was cancelled.", e, cancellationToken);
        }

        cancellationToken.ThrowIfCancellationRequested();
        var elapsed = options.TimeProvider.GetElapsedTime(started);
        return result with { DurationMs = elapsed.Ticks / TimeSpan.TicksPerMicrosecond / 1000.0 };
    }

    private async Task<ScenarioResult> JudgeAsync(Scenario scenario, CancellationToken cancellationToken)
    {
        var result = new ScenarioResult(scenario.Id, ScenarioOutcome.Pass, scenario.Method, "", [], "OK", false, [], 0);

        var expect = scenario.Expect;
        var missing = scenario.RequiresCapabilities.Where(capability => !capabilities.Contains(capability, StringComparer.Ordinal)).ToList();
        if (missing.Count > 0)
        {
            result = result with { MissingCapabilities = missing };
            if (scenario.ExpectWithoutCapability is null)
            {
                return Skipped(result, $"the service does not report {string.Join(", ", missing)}");
            }

            expect = scenario.ExpectWithoutCapability;
            result = result with
            {
                WithoutCapability = true,
                Reason = $"the service does not report {string.Join(", ", missing)}, so the without-capability expectation applies",
            };
        }

        string modelHash;
        IMessage request;
        try
        {
            modelHash = scenario.Model is { } model ? await ModelHashAsync(model, cancellationToken).ConfigureAwait(false) : "";
            request = ScenarioRequests.Build(scenario, modelHash, fixtures);
        }
        catch (InvalidDataException e)
        {
            return Errored(result, e.Message);
        }

        cancellationToken.ThrowIfCancellationRequested();
        ScenarioAnswer answer;
        using (var deadline = new CancellationTokenSource(options.CallTimeout, options.TimeProvider))
        using (var call = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, deadline.Token))
        {
            try
            {
                answer = await AwaitAsync(
                    client.CallAsync(ScenarioRequests.Method(scenario.Method), request, call.Token), call.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested && deadline.IsCancellationRequested)
            {
                answer = new ScenarioAnswer.Refused(StatusCode.DeadlineExceeded, $"the call did not end within {Seconds(options.CallTimeout)}");
            }
            catch (Exception e) when (e is not OperationCanceledException || !cancellationToken.IsCancellationRequested)
            {
                return Errored(result, $"the {client.Protocol} client failed: {e.GetType().FullName}: {e.Message}");
            }
        }

        cancellationToken.ThrowIfCancellationRequested();
        return answer switch
        {
            ScenarioAnswer.Unexpressible unexpressible => Skipped(result, unexpressible.Reason),
            ScenarioAnswer.Refused refused => JudgeRefusal(result, expect, refused),
            ScenarioAnswer.Answered answered => await JudgeResponseAsync(result, expect, answered.Response, modelHash).ConfigureAwait(false),
            _ => throw new InvalidOperationException($"the {client.Protocol} client answered a {answer.GetType()}"),
        };
    }

    private static ScenarioResult JudgeRefusal(ScenarioResult result, Expectation expect, ScenarioAnswer.Refused refused)
    {
        result = result with { Status = StatusNames.Name(refused.Status) };
        if (!StatusNames.Same(expect.WantStatus, refused.Status))
        {
            return Failed(result, $"status: {result.Status} ({refused.Message}), want {expect.WantStatus}");
        }

        if (expect.StatusMessageContains.Length > 0 && !refused.Message.Contains(expect.StatusMessageContains, StringComparison.Ordinal))
        {
            return Failed(result, $"status message {Spelling.Quote(refused.Message)} does not contain {Spelling.Quote(expect.StatusMessageContains)}");
        }

        return result;
    }

    private async Task<ScenarioResult> JudgeResponseAsync(ScenarioResult result, Expectation expect, IMessage response, string modelHash)
    {
        if (!StatusNames.Same(expect.WantStatus, StatusCode.Ok))
        {
            return Failed(result, $"the call succeeded, want status {expect.WantStatus}");
        }

        var observed = new Normalizer(modelHash).Normalize(response);
        if (options.Verbose)
        {
            await options.ScenarioLog.WriteAsync(Indent(ObservationJson.Render(observed)) + "\n").ConfigureAwait(false);
        }

        var failures = Checks.Check(expect, observed);
        return failures.Count == 0 ? result : result with { Outcome = ScenarioOutcome.Fail, Failures = failures };
    }

    /// <summary>
    /// Parses a scenario's model once per run and returns the hash the service gave it. A model
    /// that cannot be read, is refused, reports a failure or an error diagnostic, or comes back
    /// without a hash is an error in the suite.
    /// </summary>
    private async Task<string> ModelHashAsync(ScenarioModel model, CancellationToken cancellationToken)
    {
        if (models.TryGetValue(model.Key, out var cached))
        {
            return cached;
        }

        var documents = model.FixtureNames.Select(name => new FixtureSource(name, fixtures.Text(name))).ToList();
        var named = Spelling.Quote(string.Join(", ", model.FixtureNames));
        ModelParse parse;
        using (var deadline = new CancellationTokenSource(options.CallTimeout, options.TimeProvider))
        using (var call = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, deadline.Token))
        {
            try
            {
                parse = await AwaitAsync(client.ParseAsync(model, documents, call.Token), call.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException e) when (!cancellationToken.IsCancellationRequested && deadline.IsCancellationRequested)
            {
                throw new InvalidDataException($"parsing fixture {named}: the parse did not end within {Seconds(options.CallTimeout)}", e);
            }
            catch (Exception e) when (e is not OperationCanceledException || !cancellationToken.IsCancellationRequested)
            {
                throw new InvalidDataException($"parsing fixture {named}: {e.Message}", e);
            }
        }

        cancellationToken.ThrowIfCancellationRequested();
        if (parse.Error.Length > 0)
        {
            throw new InvalidDataException($"parsing fixture {named}: {parse.Error}");
        }

        if (parse.Diagnostics.FirstOrDefault(diagnostic => string.Equals(diagnostic.Severity, "error", StringComparison.OrdinalIgnoreCase)) is { } error)
        {
            throw new InvalidDataException($"fixture {named} does not parse clean: {error.Message}");
        }

        if (parse.Hash.Length == 0)
        {
            throw new InvalidDataException($"parsing fixture {named} returned no model hash");
        }

        models[model.Key] = parse.Hash;
        return parse.Hash;
    }

    private static async Task<T> AwaitAsync<T>(Task<T> pending, CancellationToken cancellationToken)
    {
        try
        {
            return await pending.WaitAsync(cancellationToken).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            // An adapter may finish after the runner stops waiting; observe any late fault.
            _ = pending.ContinueWith(
                completed => _ = completed.Exception,
                CancellationToken.None,
                TaskContinuationOptions.OnlyOnFaulted | TaskContinuationOptions.ExecuteSynchronously,
                TaskScheduler.Default);
            throw;
        }
    }

    private void Log(ScenarioResult result)
    {
        var mark = result.Outcome switch
        {
            ScenarioOutcome.Pass => "PASS",
            ScenarioOutcome.Fail => "FAIL",
            ScenarioOutcome.Skip => "SKIP",
            _ => "ERR ",
        };
        var log = options.ScenarioLog;
        log.Write($"{mark} {result.Id,-46} {result.Status}\n");
        if (result.Reason.Length > 0)
        {
            log.Write($"       {result.Reason}\n");
        }

        foreach (var failure in result.Failures)
        {
            log.Write($"       {failure}\n");
        }
    }

    private static ScenarioResult Skipped(ScenarioResult result, string reason) =>
        result with { Outcome = ScenarioOutcome.Skip, Status = NoCall, Reason = reason };

    private static ScenarioResult Errored(ScenarioResult result, string reason) =>
        result with { Outcome = ScenarioOutcome.Error, Status = NoCall, Reason = reason };

    private static ScenarioResult Failed(ScenarioResult result, string failure) =>
        result with { Outcome = ScenarioOutcome.Fail, Failures = [failure] };

    private static string Indent(string text) => "       " + text.Replace("\n", "\n       ", StringComparison.Ordinal);

    private static string Seconds(TimeSpan timeout) =>
        $"{timeout.TotalSeconds.ToString("0.###", CultureInfo.InvariantCulture)} s";
}
