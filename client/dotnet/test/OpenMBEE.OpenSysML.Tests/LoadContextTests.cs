using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The scope of the service registry, proved with real copies of the client: the load-context probe loads
/// three copies into collectible load contexts, each starting a private service of its own, then closes
/// or unloads them one at a time. Each copy's service must stop with that copy alone, every copy must be
/// collected, and the probe must exit by itself, which a foreground thread left running would prevent.
/// </summary>
public sealed class LoadContextTests
{
    private const int Copies = 3;

    [Theory]
    [Trait(Category.Name, Category.Integration)]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task EachCopyStartsItsOwnServiceWhichClosingStopsAloneAndEveryCopyIsCollected(ClientWire wire)
    {
        await using var probe = ProbeRun.Start(ProbeRun.LoadContext, "dispose", ProbeRun.Argument(wire), ServiceGate.FromProcess.RequireBinary());
        var events = await probe.ReadToExitAsync();

        var opened = Steps(events, "opened");
        AssertSeparateCopies(opened);
        Assert.All(opened, step => Assert.Equal(2, step.GetProperty("holders").GetInt32()));
        Assert.All(opened, step => Assert.True(step.GetProperty("sameAddress").GetBoolean(), "the connections of one copy share its service"));
        var closed = Steps(events, "closed");
        for (var i = 0; i < Copies; i++)
        {
            Assert.Equal(0, closed[i].GetProperty("entries").GetInt32());
            AssertOnlyLaterCopiesRun(closed[i], i);
        }

        AssertCollected(events);
    }

    [Theory]
    [Trait(Category.Name, Category.Integration)]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task UnloadingACopyStopsTheServiceItStillHoldsAndTheCopyIsCollected(ClientWire wire)
    {
        await using var probe = ProbeRun.Start(ProbeRun.LoadContext, "unload", ProbeRun.Argument(wire), ServiceGate.FromProcess.RequireBinary());
        var events = await probe.ReadToExitAsync();

        AssertSeparateCopies(Steps(events, "opened"));
        var unloaded = Steps(events, "unloaded");
        for (var i = 0; i < Copies; i++)
        {
            Assert.True(unloaded[i].GetProperty("exited").GetBoolean(), $"the service of copy {i} outlived its unloading");
            AssertOnlyLaterCopiesRun(unloaded[i], i);
        }

        AssertCollected(events);
    }

    [Fact]
    [Trait(Category.Name, Category.Unit)]
    public async Task UnloadingACopyWhoseServiceIsStillStartingStopsItInOrder()
    {
        var children = Enumerable.Range(0, Copies)
            .Select(_ => FakeChild.Create(child => new FakeScript { ProcessIdFile = child.ProcessIdFile, ExitMarker = child.ExitMarker }))
            .ToList();
        try
        {
            var arguments = new List<string> { "unload-starting", ProbeRun.Argument(ClientWire.Grpc) };
            arguments.AddRange(children.SelectMany(child => new[] { child.Path, child.ProcessIdFile }));
            await using var probe = ProbeRun.Start(ProbeRun.LoadContext, [.. arguments]);
            var events = await probe.ReadToExitAsync();

            var starting = Steps(events, "starting");
            Assert.Equal(Copies, starting.Select(step => step.GetProperty("service").GetInt32()).Distinct().Count());
            var unloaded = Steps(events, "unloaded");
            for (var i = 0; i < Copies; i++)
            {
                Assert.Equal(await children[i].ProcessIdAsync(), starting[i].GetProperty("service").GetInt32());
                Assert.True(unloaded[i].GetProperty("exited").GetBoolean(), $"the starting service of copy {i} outlived its unloading");
                Assert.True(File.Exists(children[i].ExitMarker), $"the starting service of copy {i} was not stopped by closing its input");
                AssertOnlyLaterCopiesRun(unloaded[i], i);
            }

            AssertCollected(events);
        }
        finally
        {
            children.ForEach(child => child.Dispose());
        }
    }

    private static List<JsonElement> Steps(IReadOnlyList<JsonElement> events, string step)
    {
        var steps = events.Where(e => e.GetProperty("event").GetString() == step).OrderBy(e => e.GetProperty("copy").GetInt32()).ToList();
        Assert.Equal(Enumerable.Range(0, Copies), steps.Select(e => e.GetProperty("copy").GetInt32()));
        return steps;
    }

    private static void AssertSeparateCopies(List<JsonElement> opened)
    {
        Assert.Equal(Copies, opened.Select(step => step.GetProperty("service").GetInt32()).Distinct().Count());
        Assert.Equal(Copies, opened.Select(step => step.GetProperty("context").GetString()).Distinct().Count());
        Assert.All(opened, step => Assert.True(step.GetProperty("collectible").GetBoolean(), "each copy of the client is loaded in a collectible context"));
    }

    // After copy i is closed or unloaded, its service and every earlier one is gone, and every later one runs.
    private static void AssertOnlyLaterCopiesRun(JsonElement step, int index)
    {
        var running = step.GetProperty("running").EnumerateArray().Select(value => value.GetBoolean()).ToList();
        Assert.Equal(Enumerable.Range(0, Copies).Select(copy => copy > index), running);
    }

    private static void AssertCollected(IReadOnlyList<JsonElement> events)
    {
        var collected = Assert.Single(events, e => e.GetProperty("event").GetString() == "collected");
        Assert.All(collected.GetProperty("copies").EnumerateArray(), copy => Assert.True(copy.GetBoolean(), $"a copy of the client was never collected: {collected}"));
        Assert.False(collected.GetProperty("libraryInDefault").GetBoolean(), "the client was loaded into the default context, so the copies were not alone");
    }
}
