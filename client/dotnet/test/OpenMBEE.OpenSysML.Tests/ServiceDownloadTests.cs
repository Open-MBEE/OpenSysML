using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The real sysml-grpc published as a release by a loopback release host: a private service is started from it,
/// downloaded into a cache of the test's own, verified against its pin and run from its snapshot, and a later
/// open finds it there without fetching it again.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceDownloadTests
{
    private const string Tag = "v9.9.9";

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task APrivateServiceIsStartedFromAReleaseDownloadedVerifiedAndCachedAndReusedAfterwards(ClientWire wire)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var bytes = await File.ReadAllBytesAsync(binary, Token);
        var digest = TestCache.Sha256(bytes);
        using var cache = new TestCache();
        await using var server = new ReleaseServer();
        server.Publish(Tag, bytes);
        var source = server.Source(ReleaseServer.Pinning(Tag, bytes));
        var registry = new ServiceRegistry(PrivateService.StartAsync);
        var options = new ConnectionOptions { DownloadVersion = Tag, Protocol = wire.Protocol(), Encoding = wire.Encoding() };
        string? Environment(string name) => name is "HOME" or "USERPROFILE" ? cache.Root : null;

        int processId;
        await using (var connection = await Connection.OpenAsync(options, Environment, registry, source, Token))
        {
            var entry = Assert.Single(registry.Entries());
            processId = entry.ProcessId!.Value;
            Assert.Equal((BinaryResolver.Canonical(cache.Layout.Snapshot(digest)), digest), (entry.Identity.Path, entry.Identity.Sha256));
            Assert.True(connection.ServerInfo.Answered);
            var model = (await connection.LoadContentAsync("package Demo { part def Vehicle { attribute mass default = 1500.0; } }", cancellationToken: Token)).RequireNoErrors();
            var vehicle = await model.LookupAsync("Vehicle", Token);
            Assert.Equal("1500.0", (await model.EvalAsync("mass", new EvalOptions { ContextSymbolId = vehicle.Id }, Token)).ToString());
        }

        await FakeChild.AssertGoneAsync(processId);
        Assert.Equal(new CacheMetadata(Tag, digest, ReleaseServer.Repo), CacheMetadata.Read(cache.Layout.Metadata).Metadata);
        Assert.Equal(1, server.Requested.Count(path => path == ReleaseServer.AssetPath(Tag)));

        var fetched = server.Requested.Count;
        await using (await Connection.OpenAsync(options, Environment, registry, source, Token))
        {
            Assert.Equal(digest, Assert.Single(registry.Entries()).Identity.Sha256);
        }

        Assert.Equal(fetched, server.Requested.Count);
        Assert.Empty(registry.Entries());
    }
}
