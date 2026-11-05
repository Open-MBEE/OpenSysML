using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Who closes a model's connection: a model loaded through a connection borrows it, a one-shot model
/// owns its own and closes it exactly once, and a one-shot parse that fails closes it before failing.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelOwnershipTests
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelLoadedThroughAConnectionBorrowsIt(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }.Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        await model.DisposeAsync();
        await connection.ModelByHash("h").DisposeAsync();

        Assert.False(model.OwnsConnection);
        Assert.Equal("h", (await connection.LoadAsync()).Hash);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AOneShotModelOwnsItsConnectionAndClosesItOnce(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }.Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        var logger = new ListLogger();
        Connection? opened = null;

        var model = await OpenSysMLClient.LoadOwnedAsync(
            async token => opened = await Connection.OpenAsync(FakeConnection.Options(handler, logger: logger), token),
            (connection, token) => connection.LoadContentAsync("package P;", null, token),
            TestContext.Current.CancellationToken);
        await Task.WhenAll(Enumerable.Range(0, 4).Select(_ => model.DisposeAsync().AsTask()));
        await model.DisposeAsync();

        Assert.True(model.OwnsConnection);
        Assert.Same(opened, model.Connection);
        Assert.Equal("h", model.Hash);
        Assert.Single(logger.Entries, entry => entry.Message.StartsWith("Closed the connection", StringComparison.Ordinal));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.SymbolAsync("P", TestContext.Current.CancellationToken));
        Assert.Equal(0, handler.Disposals);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ConcurrentModelDisposalsAllWaitForTheConnectionToDrain(ClientWire wire)
    {
        var entered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var cancelled = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        var respond = handler.Respond;
        handler.Respond = async (sent, token) =>
        {
            if (sent.Method != "Evaluate")
            {
                return await respond(sent, token);
            }

            using var registration = token.Register(() => cancelled.TrySetResult());
            entered.TrySetResult();
            await release.Task.WaitAsync(TestContext.Current.CancellationToken);
            token.ThrowIfCancellationRequested();
            return handler.Answer(new Wire.EvaluateResponse { Result = new Wire.Value { IntValue = 1 } });
        };
        await using var model = await OpenSysMLClient.LoadContentAsync(
            "package P;", FakeConnection.Options(handler), cancellationToken: TestContext.Current.CancellationToken);
        var evaluation = model.EvalAsync("1", cancellationToken: TestContext.Current.CancellationToken);
        await entered.Task.WaitAsync(TestContext.Current.CancellationToken);

        var first = model.DisposeAsync().AsTask();
        var second = model.DisposeAsync().AsTask();
        try
        {
            await cancelled.Task.WaitAsync(TestContext.Current.CancellationToken);
            Assert.False(first.IsCompleted);
            Assert.False(second.IsCompleted);
        }
        finally
        {
            release.TrySetResult();
            await Assert.ThrowsAsync<ObjectDisposedException>(() => evaluation);
            await Task.WhenAll(first, second);
        }
    }

    public static TheoryData<ClientWire, string> Failures()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var failure in new[] { "in band", "refused", "capability", "cancelled" })
            {
                data.Add(wire, failure);
            }
        }

        return data;
    }

    [Theory]
    [MemberData(nameof(Failures))]
    public async Task AOneShotParseThatFailsClosesTheConnectionItOpened(ClientWire wire, string failure)
    {
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.StrictConformance])] };
        handler.Respond = async (sent, token) =>
        {
            switch (failure)
            {
                case "in band":
                    return handler.Answer(new Wire.ParseFileResponse { Error = "gave up" });
                case "refused":
                    return handler.Refuse(StatusCode.InvalidArgument, "language must be sysml or kerml");
                default:
                    await cancel.CancelAsync();
                    await Task.Delay(Timeout.Infinite, token);
                    throw new InvalidOperationException("unreachable");
            }
        };
        Connection? opened = null;
        var parse = new ParseOptions { StrictConformance = failure == "capability" };

        Task Load() => OpenSysMLClient.LoadOwnedAsync(
            async token => opened = await Connection.OpenAsync(FakeConnection.Options(handler), token),
            (connection, token) => connection.LoadContentAsync("package P;", parse, token),
            cancel.Token);

        Exception error = failure switch
        {
            "in band" => await Assert.ThrowsAsync<ModelException>(Load),
            "refused" => await Assert.ThrowsAsync<ServiceException>(Load),
            "capability" => await Assert.ThrowsAsync<CapabilityException>(Load),
            _ => await Assert.ThrowsAnyAsync<OperationCanceledException>(Load),
        };

        Assert.NotNull(error);
        Assert.NotNull(opened);
        await Assert.ThrowsAsync<ObjectDisposedException>(() => opened!.LoadContentAsync("x", cancellationToken: TestContext.Current.CancellationToken));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheOneShotLoadsOpenTheirOwnConnection(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .Refusing("GetDiagnostics", StatusCode.NotFound, "model not found: h");
        var options = FakeConnection.Options(handler);

        await using var inline = await OpenSysMLClient.LoadContentAsync("package P;", options, cancellationToken: TestContext.Current.CancellationToken);
        await using var file = await OpenSysMLClient.LoadAsync("a.sysml", options, cancellationToken: TestContext.Current.CancellationToken);

        Assert.True(inline.OwnsConnection);
        Assert.NotSame(inline.Connection, file.Connection);
        Assert.Equal(["a.sysml"], file.Documents);
        Assert.Equal(2, handler.RequestsFor("GetServerInfo").Count);
        await Assert.ThrowsAsync<ModelNotFoundException>(() => file.GetDiagnosticsAsync(cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => OpenSysMLClient.LoadAsync(string.Empty, options, cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() => OpenSysMLClient.LoadContentAsync(null!, options, cancellationToken: TestContext.Current.CancellationToken));
    }

    [Fact]
    public async Task AOneShotLoadWhoseOpenFailsParsesNothing()
    {
        var parsed = false;

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => OpenSysMLClient.LoadOwnedAsync(
            token => Connection.OpenAsync(new ConnectionOptions { AutoStart = false }, _ => null, token),
            (connection, token) =>
            {
                parsed = true;
                return connection.LoadContentAsync("package P;", null, token);
            },
            TestContext.Current.CancellationToken));

        Assert.Contains("AutoStart is false", error.Message, StringComparison.Ordinal);
        Assert.False(parsed);
    }

    [Fact]
    public async Task ARecordedOwnershipIsCarriedOnlyByTheOneShotHandle()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities }.Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        await using var connection = await FakeConnection.OpenAsync(handler);
        var borrowed = await connection.LoadAsync();

        var owning = borrowed.Owning();

        Assert.False(borrowed.OwnsConnection);
        Assert.True(owning.OwnsConnection);
        Assert.Equal(borrowed.Hash, owning.Hash);
        Assert.Equal(borrowed.Roots, owning.Roots);
        Assert.Equal(borrowed.ParseDiagnostics, owning.ParseDiagnostics);
        Assert.Equal("model h (1 root, 0 errors)", borrowed.ToString());
        IReadOnlyList<string> documents = owning.Documents;
        Assert.Empty(documents);
    }
}
