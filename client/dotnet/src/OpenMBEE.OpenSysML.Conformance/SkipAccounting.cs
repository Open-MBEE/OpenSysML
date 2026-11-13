using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// What a run must show beyond each scenario's own verdict: that every skip is one the expected skips
/// list for that reason or one a withheld capability explains, that no listed skip ran, that a
/// configuration withholding capabilities withholds exactly those, and that each withheld
/// capability's without-capability expectation was executed rather than skipped.
/// </summary>
internal static class SkipAccounting
{
    /// <summary>The skips of one protocol's run that the list and the configuration do not account for, and the listed skips that ran.</summary>
    /// <param name="configuration">The configuration's name, for messages.</param>
    /// <param name="unavailable">The capabilities the configuration withholds; empty for the default.</param>
    /// <param name="protocol">The protocol's results.</param>
    /// <param name="expected">The expected skips.</param>
    /// <param name="allowSkips">Whether every skip is accepted, as <c>-allow-skips</c> asks; a listed skip that ran is still reported.</param>
    /// <returns>Every problem, in result order.</returns>
    internal static IEnumerable<string> Skips(
        string configuration, IReadOnlyCollection<string> unavailable, ProtocolSummary protocol, ExpectedSkips expected, bool allowSkips)
    {
        ArgumentNullException.ThrowIfNull(protocol);
        ArgumentNullException.ThrowIfNull(expected);
        var where = $"{configuration} {protocol.Protocol}";
        foreach (var result in protocol.Results)
        {
            var reason = expected.ReasonFor(result.Id, result.Rpc);
            if (result.Outcome != ScenarioOutcome.Skip)
            {
                if (reason is not null)
                {
                    yield return $"{where}: {result.Id} is listed as an expected skip ({reason}), but it ran and ended {ReportJson.OutcomeName(result.Outcome)}";
                }

                continue;
            }

            if (allowSkips)
            {
                continue;
            }

            if (IsCapabilitySkip(result))
            {
                if (!result.MissingCapabilities.All(unavailable.Contains))
                {
                    yield return $"{where}: {result.Id} was skipped: {result.Reason}";
                }
            }
            else if (reason is null)
            {
                yield return $"{where}: {result.Id} was skipped, which the expected skips do not list: {result.Reason}";
            }
            else if (!string.Equals(reason, result.Reason, StringComparison.Ordinal))
            {
                yield return $"{where}: {result.Id} was skipped because {result.Reason}, but the expected skips give {reason}";
            }
        }
    }

    /// <summary>
    /// Whether each capability a configuration withholds had a without-capability expectation
    /// executed. One whose every such scenario is an expected skip is a note rather than a problem:
    /// the client cannot yet make that call at all.
    /// </summary>
    /// <param name="configuration">The configuration's name, for messages.</param>
    /// <param name="unavailable">The capabilities it withholds.</param>
    /// <param name="protocol">The protocol's results.</param>
    /// <param name="expected">The expected skips.</param>
    /// <returns>The problems, and the notes.</returns>
    internal static (IReadOnlyList<string> Problems, IReadOnlyList<string> Notes) Fallbacks(
        string configuration, IReadOnlyList<string> unavailable, ProtocolSummary protocol, ExpectedSkips expected)
    {
        ArgumentNullException.ThrowIfNull(unavailable);
        ArgumentNullException.ThrowIfNull(protocol);
        ArgumentNullException.ThrowIfNull(expected);
        var problems = new List<string>();
        var notes = new List<string>();
        var where = $"{configuration} {protocol.Protocol}";
        foreach (var capability in unavailable)
        {
            var fallbacks = protocol.Results.Where(result => result.WithoutCapability && result.MissingCapabilities.Contains(capability)).ToList();
            if (fallbacks.Any(result => result.Outcome != ScenarioOutcome.Skip))
            {
                continue;
            }

            if (fallbacks.Count > 0 && fallbacks.All(result => string.Equals(expected.ReasonFor(result.Id, result.Rpc), result.Reason, StringComparison.Ordinal)))
            {
                notes.Add($"{where}: no without-capability expectation of {capability} ran; each is an expected skip: {string.Join(", ", fallbacks.Select(result => result.Id))}");
            }
            else
            {
                problems.Add($"{where}: no without-capability expectation executed for {capability}");
            }
        }

        return (problems, notes);
    }

    /// <summary>
    /// Whether a service withholding capabilities advertises exactly the default service's set without
    /// them; null when it does. Each withheld capability must be one the default service advertises.
    /// </summary>
    /// <param name="defaults">What the default service advertised.</param>
    /// <param name="actual">What the withholding service advertised.</param>
    /// <param name="unavailable">What it was asked to withhold.</param>
    /// <returns>The problem, or null.</returns>
    internal static string? WithheldSet(IReadOnlyCollection<string> defaults, IReadOnlyCollection<string> actual, IReadOnlyList<string> unavailable)
    {
        ArgumentNullException.ThrowIfNull(defaults);
        ArgumentNullException.ThrowIfNull(actual);
        ArgumentNullException.ThrowIfNull(unavailable);
        if (unavailable.FirstOrDefault(capability => !defaults.Contains(capability)) is { } unknown)
        {
            return $"the default service does not advertise {Spelling.Quote(unknown)}";
        }

        var want = defaults.Except(unavailable, StringComparer.Ordinal).Order(Spelling.CodePointOrder).ToList();
        var got = actual.Order(Spelling.CodePointOrder).ToList();
        return want.SequenceEqual(got, StringComparer.Ordinal)
            ? null
            : $"capabilities = [{string.Join(" ", got)}], want the default set without [{string.Join(" ", unavailable)}]: [{string.Join(" ", want)}]";
    }

    // Skipped before any call, for a capability the service lacks and no without-capability expectation.
    private static bool IsCapabilitySkip(ScenarioResult result) => result.MissingCapabilities.Count > 0 && !result.WithoutCapability;
}
