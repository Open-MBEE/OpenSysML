using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Whether the service under test has an SMT solver to answer a holds or satisfiable question, as its own solve engine
/// reports. A developer without one skips the test that needs an answer. Where one was promised by setting
/// OPENSYSML_REQUIRE_SMT, its absence is the failure that test exists to catch, so the skip fails instead.
/// </summary>
/// <param name="environment">Reads an environment variable; null when it is unset.</param>
internal sealed class SolverGate(Func<string, string?> environment)
{
    /// <summary>Set where a solver is provided, so a missing one fails instead of skipping.</summary>
    internal const string RequireVariable = "OPENSYSML_REQUIRE_SMT";

    /// <summary>The engine the service answers solver questions with.</summary>
    internal const string Engine = "solve";

    /// <summary>The gate over this process's environment.</summary>
    internal static SolverGate FromProcess { get; } = new(Environment.GetEnvironmentVariable);

    /// <summary>Whether the environment promised a solver, making its absence a failure.</summary>
    internal bool Required
    {
        get
        {
            var value = environment(RequireVariable)?.Trim();
            return !(string.IsNullOrEmpty(value)
                || value == "0"
                || string.Equals(value, "false", StringComparison.OrdinalIgnoreCase)
                || string.Equals(value, "no", StringComparison.OrdinalIgnoreCase));
        }
    }

    /// <summary>Decides, without acting on it, whether the test needing a solver runs, skips or fails.</summary>
    /// <param name="engines">The engines the service lists.</param>
    internal Decision Decide(IEnumerable<EngineInfo> engines)
    {
        var solve = engines.FirstOrDefault(engine => engine.Name == Engine);
        if (solve is { Ready: true })
        {
            return new Decision(Runs: true, null, Fails: false);
        }

        var reason = solve is null
            ? $"the service lists no {Engine} engine"
            : $"the service's {Engine} engine is unavailable: {solve.Unavailable}";
        return Required
            ? new Decision(Runs: false, $"${RequireVariable} is set, so this test needs a solver's answer, but {reason}.", Fails: true)
            : new Decision(Runs: false, reason, Fails: false);
    }

    /// <summary>Returns when the connection's service has a solver; skips or fails the running test otherwise.</summary>
    internal async Task RequireAsync(Connection connection, CancellationToken cancellationToken)
    {
        var decision = Decide(await connection.ListEnginesAsync(cancellationToken: cancellationToken));
        if (decision.Runs)
        {
            return;
        }

        if (decision.Fails)
        {
            Assert.Fail(decision.Reason);
        }

        Assert.Skip(decision.Reason!);
    }

    /// <summary>What the gate decided: run, or skip or fail for <paramref name="Reason"/>.</summary>
    /// <param name="Runs">Whether a solver answers, so the test runs.</param>
    /// <param name="Reason">Why none does, when none does.</param>
    /// <param name="Fails">Whether its absence fails the test rather than skipping it.</param>
    internal sealed record Decision(bool Runs, string? Reason, bool Fails);
}
