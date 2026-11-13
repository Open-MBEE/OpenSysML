using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Runs the suite through the client's public API as the reference runner runs it through the wire:
/// one configuration of the service by default and one more withholding capabilities when asked,
/// each protocol over its own <see cref="Connection"/> to the one service of that configuration.
/// The run fails for a scenario that failed or is in error, for a skip the expected skips and the
/// configuration do not account for, for a listed skip that ran, and for a withheld capability no
/// without-capability expectation was executed for.
/// </summary>
internal static class ConformanceRun
{
    /// <summary>Runs the suite.</summary>
    /// <param name="options">What to run.</param>
    /// <param name="output">Where the scenarios and totals are logged.</param>
    /// <param name="errors">Where what failed the run is written.</param>
    /// <param name="cancellationToken">Cancels the run; it is thrown.</param>
    /// <returns>The exit status: 0 when the run passed, 1 otherwise.</returns>
    internal static async Task<int> RunAsync(RunOptions options, TextWriter output, TextWriter errors, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(options);
        ArgumentNullException.ThrowIfNull(output);
        ArgumentNullException.ThrowIfNull(errors);

        IReadOnlyList<Scenario> scenarios;
        FixtureDirectory fixtures;
        ExpectedSkips expected;
        try
        {
            scenarios = ScenarioSuite.Load(Path.Combine(options.Directory, "scenarios"));
            fixtures = new FixtureDirectory(Path.Combine(options.Directory, "fixtures"));
            expected = ExpectedSkips.Load(options.ExpectedSkips);
        }
        catch (Exception e) when (e is InvalidDataException or IOException or UnauthorizedAccessException)
        {
            return await FailAsync(errors, [e.Message]).ConfigureAwait(false);
        }

        if (expected.Problems(scenarios, ScenarioRequests.Service, ApiAdapters.All.Keys) is { Count: > 0 } stale)
        {
            return await FailAsync(errors, stale).ConfigureAwait(false);
        }

        var configurations = new List<(string Name, IReadOnlyList<string> Unavailable)> { ("default", []) };
        if (options.Withhold.Count > 0)
        {
            configurations.Add(($"without-{string.Join("-", options.Withhold)}", options.Withhold));
        }

        var served = options.Binary ?? options.Service!;
        var problems = new List<string>();
        var defaults = new Dictionary<string, IReadOnlyList<string>>(StringComparer.Ordinal);
        var summaries = new List<ConfigurationSummary>();
        foreach (var (name, unavailable) in configurations)
        {
            await output.WriteAsync($"\nConfiguration {name}\n").ConfigureAwait(false);
            var protocols = new List<ProtocolSummary>();
            ServiceProcess? started = null;
            try
            {
                string address;
                if (options.Binary is { } binary)
                {
                    started = await ServiceProcess.StartAsync(
                        binary,
                        options.Transport,
                        cancellationToken,
                        unavailable.Count > 0 ? string.Join(",", unavailable) : null,
                        scenarios.Count + 16).ConfigureAwait(false);
                    address = started.Address;
                }
                else
                {
                    address = options.Service!;
                }

                foreach (var protocol in options.Protocols)
                {
                    var summary = await RunProtocolAsync(options, protocol, address, served, unavailable.Count > 0, scenarios, fixtures, output, cancellationToken).ConfigureAwait(false);
                    if (unavailable.Count == 0)
                    {
                        defaults[protocol] = summary.Capabilities;
                    }
                    else if (SkipAccounting.WithheldSet(defaults[protocol], summary.Capabilities, unavailable) is { } mismatch)
                    {
                        return await FailAsync(errors, [$"{name} {protocol}: {mismatch}"]).ConfigureAwait(false);
                    }

                    protocols.Add(summary);
                    problems.AddRange(SkipAccounting.Skips(name, unavailable, summary, expected, options.AllowSkips));
                    if (unavailable.Count > 0)
                    {
                        var (missing, notes) = SkipAccounting.Fallbacks(name, unavailable, summary, expected);
                        problems.AddRange(missing);
                        foreach (var note in notes)
                        {
                            await output.WriteAsync($"note: {note}\n").ConfigureAwait(false);
                        }
                    }
                }
            }
            catch (Exception e) when (e is OpenSysMLException or InvalidOperationException or IOException or Win32Exception
                || (e is OperationCanceledException && !cancellationToken.IsCancellationRequested))
            {
                return await FailAsync(errors, [$"{name}: {e.Message}"]).ConfigureAwait(false);
            }
            finally
            {
                if (started is not null)
                {
                    await started.DisposeAsync().ConfigureAwait(false);
                }
            }

            summaries.Add(new ConfigurationSummary(name, unavailable, protocols));
        }

        var report = new ConformanceReport(served, summaries);
        try
        {
            await WriteReportsAsync(options, report, output, cancellationToken).ConfigureAwait(false);
        }
        catch (IOException e)
        {
            return await FailAsync(errors, [e.Message]).ConfigureAwait(false);
        }

        var totals = report.Totals;
        if (totals.Failed + totals.Errored > 0)
        {
            problems.Insert(0, $"{totals.Failed + totals.Errored} of {totals.Total} scenarios failed");
        }

        return problems.Count > 0 ? await FailAsync(errors, problems).ConfigureAwait(false) : 0;
    }

    private static async Task<ProtocolSummary> RunProtocolAsync(
        RunOptions options,
        string protocol,
        string address,
        string served,
        bool withholds,
        IReadOnlyList<Scenario> scenarios,
        FixtureDirectory fixtures,
        TextWriter output,
        CancellationToken cancellationToken)
    {
        var connectionOptions = new ConnectionOptions
        {
            Address = address,
            Protocol = protocol == "grpc" ? Protocol.Grpc : Protocol.Connect,
            Encoding = protocol == "connect-json" ? MessageEncoding.Json : MessageEncoding.Protobuf,
        };
        var connection = await Connection.OpenAsync(connectionOptions, cancellationToken).ConfigureAwait(false);
        await using (connection.ConfigureAwait(false))
        {
            IScenarioClient client = new ApiScenarioClient(protocol, served, connection);
            if (options.Mutation != Mutation.None)
            {
                client = new MutatingScenarioClient(client, options.Mutation);
            }

            var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions
            {
                Verbose = options.Verbose,
                OmitHandshakeScenarios = withholds,
                Output = output,
                ScenarioLog = output,
            });
            return await runner.RunAllAsync(scenarios, options.Filter, cancellationToken).ConfigureAwait(false);
        }
    }

    private static async Task WriteReportsAsync(RunOptions options, ConformanceReport report, TextWriter output, CancellationToken cancellationToken)
    {
        if (options.Report == "-")
        {
            using var buffer = new MemoryStream();
            ReportJson.Write(report, buffer);
            await output.WriteAsync(Encoding.UTF8.GetString(buffer.ToArray())).ConfigureAwait(false);
        }
        else if (options.Report is { } path)
        {
            await WriteFileAsync(path, "JSON", stream => ReportJson.Write(report, stream), cancellationToken).ConfigureAwait(false);
        }

        if (options.JUnit is { } junit)
        {
            await WriteFileAsync(junit, "JUnit", stream => JUnitReport.Write(report, stream), cancellationToken).ConfigureAwait(false);
        }
    }

    private static async Task WriteFileAsync(string path, string format, Action<Stream> write, CancellationToken cancellationToken)
    {
        try
        {
            if (Path.GetDirectoryName(path) is { Length: > 0 } parent)
            {
                Directory.CreateDirectory(parent);
            }

            using var buffer = new MemoryStream();
            write(buffer);
            await File.WriteAllBytesAsync(path, buffer.ToArray(), cancellationToken).ConfigureAwait(false);
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException)
        {
            throw new IOException($"cannot write {format} report to {Spelling.Quote(path)}: {e.Message}", e);
        }
    }

    private static async Task<int> FailAsync(TextWriter errors, IEnumerable<string> problems)
    {
        foreach (var problem in problems)
        {
            await errors.WriteAsync($"conformance: {problem}\n").ConfigureAwait(false);
        }

        return 1;
    }
}
