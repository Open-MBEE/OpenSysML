using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>How a scenario ended.</summary>
internal enum ScenarioOutcome
{
    /// <summary>The answer held every expectation.</summary>
    Pass,

    /// <summary>The answer disagreed with the scenario.</summary>
    Fail,

    /// <summary>The scenario was not run: the service lacks a capability it needs, or the client cannot express its request.</summary>
    Skip,

    /// <summary>The scenario itself is wrong, or could not be run: an unknown RPC, an unreadable fixture, a model that does not parse.</summary>
    Error,
}

/// <summary>One scenario's result.</summary>
/// <param name="Id">The scenario's id.</param>
/// <param name="Outcome">How it ended.</param>
/// <param name="Rpc">The bare RPC it called.</param>
/// <param name="Reason">Why it was skipped or in error, or which expectation applied; empty otherwise.</param>
/// <param name="Failures">What the answer disagreed about, one entry per mismatch.</param>
/// <param name="Status">The status the call ended with, by canonical name; <c>-</c> when no call was judged.</param>
/// <param name="WithoutCapability">Whether the without-capability expectation applied.</param>
/// <param name="MissingCapabilities">The capabilities the scenario needs that the service does not report.</param>
/// <param name="DurationMs">How long the scenario took, in milliseconds to the microsecond.</param>
internal sealed record ScenarioResult(
    string Id,
    ScenarioOutcome Outcome,
    string Rpc,
    string Reason,
    IReadOnlyList<string> Failures,
    string Status,
    bool WithoutCapability,
    IReadOnlyList<string> MissingCapabilities,
    double DurationMs);

/// <summary>One protocol's run of the suite.</summary>
/// <param name="Protocol">The protocol.</param>
/// <param name="Service">What answered.</param>
/// <param name="Capabilities">The capabilities the service reported, sorted.</param>
/// <param name="Results">One result per scenario run, in suite order.</param>
internal sealed record ProtocolSummary(
    string Protocol,
    string Service,
    IReadOnlyList<string> Capabilities,
    IReadOnlyList<ScenarioResult> Results)
{
    /// <summary>The totals of the results.</summary>
    internal Totals Totals => Totals.Of(Results);
}

/// <summary>The results of every protocol under one capability configuration of the service.</summary>
/// <param name="Name">The configuration's name: <c>default</c>, or what it withholds.</param>
/// <param name="UnavailableCapabilities">The capabilities withheld from the service; empty for the default.</param>
/// <param name="Protocols">One summary per protocol, in run order.</param>
internal sealed record ConfigurationSummary(
    string Name,
    IReadOnlyList<string> UnavailableCapabilities,
    IReadOnlyList<ProtocolSummary> Protocols)
{
    /// <summary>The totals of every protocol.</summary>
    internal Totals Totals => Totals.Sum(Protocols.Select(protocol => protocol.Totals));
}

/// <summary>A run's machine-readable report, in the shape the reference runner writes.</summary>
/// <param name="Service">The service the run tested.</param>
/// <param name="Configurations">One summary per capability configuration, in run order.</param>
internal sealed record ConformanceReport(string Service, IReadOnlyList<ConfigurationSummary> Configurations)
{
    /// <summary>The totals of every configuration.</summary>
    internal Totals Totals => Totals.Sum(Configurations.Select(configuration => configuration.Totals));
}

/// <summary>Counts of results by outcome, and of those judged by the without-capability expectation.</summary>
/// <param name="Total">Every result.</param>
/// <param name="Passed">Those that passed.</param>
/// <param name="Failed">Those that failed.</param>
/// <param name="Skipped">Those skipped.</param>
/// <param name="Errored">Those in error.</param>
/// <param name="WithoutCapability">Those judged by the without-capability expectation, whatever their outcome.</param>
internal sealed record Totals(int Total, int Passed, int Failed, int Skipped, int Errored, int WithoutCapability)
{
    /// <summary>The totals of a list of results.</summary>
    internal static Totals Of(IReadOnlyList<ScenarioResult> results)
    {
        ArgumentNullException.ThrowIfNull(results);
        return new Totals(
            results.Count,
            results.Count(result => result.Outcome == ScenarioOutcome.Pass),
            results.Count(result => result.Outcome == ScenarioOutcome.Fail),
            results.Count(result => result.Outcome == ScenarioOutcome.Skip),
            results.Count(result => result.Outcome == ScenarioOutcome.Error),
            results.Count(result => result.WithoutCapability));
    }

    /// <summary>The sum of several totals.</summary>
    internal static Totals Sum(IEnumerable<Totals> parts) =>
        parts.Aggregate(
            new Totals(0, 0, 0, 0, 0, 0),
            (sum, part) => new Totals(
                sum.Total + part.Total,
                sum.Passed + part.Passed,
                sum.Failed + part.Failed,
                sum.Skipped + part.Skipped,
                sum.Errored + part.Errored,
                sum.WithoutCapability + part.WithoutCapability));
}
