using System;
using System.Diagnostics;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// What outlives an application that is killed: the orphan probe opens a private service of the real
/// sysml-grpc, starts unrelated processes beside it and is killed alone, and the service must then end
/// on its own while the siblings run on. The leak control proves the detector: a service whose input
/// a sibling holds open must be seen to survive, and to end once that sibling goes.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class OrphanTests
{
    private const int RaceRounds = 3;
    private const int RaceSiblings = 8;

    // The child exits within milliseconds of the pipe closing; the margin is for a loaded machine.
    private static readonly TimeSpan Ends = TimeSpan.FromSeconds(10);
    private static readonly TimeSpan Survives = TimeSpan.FromSeconds(3);

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task KillingTheApplicationEndsItsServiceWhileItsSiblingsRunOn(ClientWire wire)
    {
        await using var probe = ProbeRun.Start(ProbeRun.Orphan, "serve", Binary, ProbeRun.Argument(wire), "2");
        var report = await probe.ReadAsync();

        probe.KillProbe();

        await AssertOrphanedCorrectlyAsync(report);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task SiblingsStartedTheMomentTheServiceStartsNeverKeepItAlive(ClientWire wire)
    {
        for (var round = 0; round < RaceRounds; round++)
        {
            await using var probe = ProbeRun.Start(ProbeRun.Orphan, "race", Binary, ProbeRun.Argument(wire), RaceSiblings.ToString(System.Globalization.CultureInfo.InvariantCulture));
            var report = await probe.ReadAsync();
            Assert.Equal(RaceSiblings, report.GetProperty("siblings").GetArrayLength());

            probe.KillProbe();

            await AssertOrphanedCorrectlyAsync(report);
        }
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task TheDetectorSeesAServiceWhoseInputLeakedIntoASiblingSurvive(ClientWire wire)
    {
        await using var probe = ProbeRun.Start(ProbeRun.Orphan, "leak", Binary, ProbeRun.Argument(wire));
        var report = await probe.ReadAsync();
        var service = report.GetProperty("service").GetInt32();
        var holder = Assert.Single(report.GetProperty("siblings").EnumerateArray()).GetInt32();

        probe.KillProbe();

        Assert.False(await EndsWithinAsync(service, Survives), $"service {service} ended although sibling {holder} holds its input open");
        Assert.True(FakeChild.IsRunning(holder));
        FakeChild.Kill(holder);
        Assert.True(await EndsWithinAsync(service, Ends), $"service {service} outlived the sibling holding its input by {Ends}");
    }

    /// <summary>Whether a process ends within a time: the orphan detector.</summary>
    internal static async Task<bool> EndsWithinAsync(int processId, TimeSpan within)
    {
        var watch = Stopwatch.StartNew();
        while (FakeChild.IsRunning(processId))
        {
            if (watch.Elapsed > within)
            {
                return false;
            }

            await Task.Delay(10, TestContext.Current.CancellationToken);
        }

        return true;
    }

    private static string Binary => ServiceGate.FromProcess.RequireBinary();

    private static async Task AssertOrphanedCorrectlyAsync(JsonElement report)
    {
        var service = report.GetProperty("service").GetInt32();
        var siblings = report.GetProperty("siblings").EnumerateArray().Select(sibling => sibling.GetInt32()).ToList();

        Assert.True(await EndsWithinAsync(service, Ends), $"service {service} outlived the application that started it by {Ends}");
        Assert.All(siblings, sibling => Assert.True(FakeChild.IsRunning(sibling), $"sibling {sibling} ended with the application, which only owned the service"));
    }
}
