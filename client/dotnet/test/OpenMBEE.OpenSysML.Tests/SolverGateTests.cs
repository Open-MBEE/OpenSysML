using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Whether a solver answers is the service's solve engine's to say, and whether its absence skips or fails.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SolverGateTests
{
    [Theory]
    [InlineData("1", true)]
    [InlineData(" yes ", true)]
    [InlineData(null, false)]
    [InlineData("", false)]
    [InlineData("0", false)]
    [InlineData("FALSE", false)]
    [InlineData("no", false)]
    public void OnlyAPromiseOfASolverRequiresOne(string? value, bool required)
    {
        Assert.Equal(required, Gate(value).Required);
    }

    [Fact]
    public void AReadySolveEngineRunsTheTestWhetherOrNotOneWasPromised()
    {
        EngineInfo[] engines = [Engine("run", ready: true, string.Empty), Engine(SolverGate.Engine, ready: true, string.Empty)];

        Assert.Equal(new SolverGate.Decision(Runs: true, null, Fails: false), Gate(null).Decide(engines));
        Assert.Equal(new SolverGate.Decision(Runs: true, null, Fails: false), Gate("1").Decide(engines));
    }

    [Fact]
    public void AnUnavailableOrMissingSolverSkipsUnlessOneWasPromisedAndThenFails()
    {
        EngineInfo[] unavailable = [Engine(SolverGate.Engine, ready: false, "z3 not found")];
        EngineInfo[] missing = [Engine("run", ready: true, string.Empty)];

        var skipped = Gate(null).Decide(unavailable);
        var failed = Gate("1").Decide(unavailable);

        Assert.Equal(new SolverGate.Decision(Runs: false, "the service's solve engine is unavailable: z3 not found", Fails: false), skipped);
        Assert.Equal(
            new SolverGate.Decision(Runs: false, $"${SolverGate.RequireVariable} is set, so this test needs a solver's answer, but {skipped.Reason}.", Fails: true),
            failed);
        Assert.Equal(new SolverGate.Decision(Runs: false, "the service lists no solve engine", Fails: false), Gate(null).Decide(missing));
        Assert.True(Gate("yes").Decide(missing).Fails);
    }

    private static SolverGate Gate(string? required) =>
        new(name => name == SolverGate.RequireVariable ? required : null);

    private static EngineInfo Engine(string name, bool ready, string unavailable) =>
        new(name, "proved", ["holds"], [], "z3", string.Empty, ready, unavailable, "builtin", string.Empty, string.Empty, string.Empty, string.Empty, served: true);
}
