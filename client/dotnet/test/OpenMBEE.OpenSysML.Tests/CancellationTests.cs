using System;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class CancellationTests
{
    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ACallerTokenCancelledBeforehandSendsNothing(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        using var cancel = new CancellationTokenSource();
        await cancel.CancelAsync();

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => connection.CallGetSymbolAsync(cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.Empty(handler.RequestsFor("GetSymbol"));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ACallerTokenCancelledMidFlightIsAnOperationCanceledExceptionCarryingIt(ClientWire wire)
    {
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                started.SetResult();
                await Task.Delay(Timeout.Infinite, token);
                throw new InvalidOperationException("unreachable");
            },
        };
        await using var connection = await FakeConnection.OpenAsync(handler);
        using var cancel = new CancellationTokenSource();

        var call = connection.CallGetSymbolAsync(cancel.Token);
        await started.Task.WaitAsync(TestContext.Current.CancellationToken);
        await cancel.CancelAsync();
        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => call);

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.IsNotType<TransportException>(error.InnerException?.InnerException);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ADeadlineWithoutCancellationIsATransportDeadlineExceeded(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                await Task.Delay(Timeout.Infinite, token);
                throw new InvalidOperationException("unreachable");
            },
        };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, requestTimeout: TimeSpan.FromMilliseconds(200)));

        // Only the client's own deadline can end the call; the caller's token is a backstop that fails the test instead of hanging it.
        using var backstop = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        backstop.CancelAfter(TimeSpan.FromSeconds(10));
        var error = await Assert.ThrowsAsync<TransportException>(() => connection.CallGetSymbolAsync(backstop.Token));

        Assert.Equal(StatusCode.DeadlineExceeded, error.StatusCode);
        Assert.Contains("did not complete before its deadline", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task WhenTheDeadlineAndTheCallerBothFiredTheCallerWins(ClientWire wire)
    {
        using var cancel = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                // The deadline cancels the request first; the caller then cancels before the failure is mapped.
                await Task.Delay(Timeout.Infinite, token).ContinueWith(_ => { }, TaskScheduler.Default);
                await cancel.CancelAsync();
                throw new OperationCanceledException(token);
            },
        };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, requestTimeout: TimeSpan.FromMilliseconds(200)));

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => connection.CallGetSymbolAsync(cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
    }

    [Fact]
    public async Task ATransportDeadlineReportedAfterTheCallerCancelledIsTheCallers()
    {
        using var cancel = new CancellationTokenSource();
        var transport = new FakeTransport(async (method, _, _) =>
        {
            if (method == "GetServerInfo")
            {
                return new Wire.ServerInfoResponse();
            }

            await cancel.CancelAsync();
            throw new TransportException(StatusCode.DeadlineExceeded, "late", "sysml.SysMLService/GetSymbol");
        });
        await using var connection = await FakeTransport.OpenAsync(transport, new ConnectionOptions());

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => connection.CallGetSymbolAsync(cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.IsType<TransportException>(error.InnerException);
    }

    [Fact]
    public async Task ACompletedResponseIsNotTurnedIntoACancellation()
    {
        using var cancel = new CancellationTokenSource();
        var transport = new FakeTransport(async (method, _, _) =>
        {
            if (method == "GetServerInfo")
            {
                return new Wire.ServerInfoResponse();
            }

            await cancel.CancelAsync();
            return new Wire.SymbolResponse { Error = "answered" };
        });
        await using var connection = await FakeTransport.OpenAsync(transport, new ConnectionOptions());

        var response = await connection.CallGetSymbolAsync(cancel.Token);

        Assert.Equal("answered", response.Error);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AServiceThatSaysCancelledWithoutTheCallerCancellingIsTheServices(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.RefuseOn(wire, StatusCode.Cancelled, "shutting down")) };
        await using var connection = await FakeConnection.OpenAsync(handler);
        using var cancel = new CancellationTokenSource();

        var error = await Assert.ThrowsAsync<ServiceException>(() => connection.CallGetSymbolAsync(cancel.Token));

        Assert.Equal(StatusCode.Cancelled, error.StatusCode);
    }
}

[Trait(Category.Name, Category.Unit)]
public sealed class DisposalTests
{
    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ACallAfterDisposalIsRefusedAndSendsNothing(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        var connection = await FakeConnection.OpenAsync(handler);
        await connection.DisposeAsync();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.GetSymbolAsync());
        await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.CallAsync<Wire.ServerInfoRequest, Wire.ServerInfoResponse>("Nope", new Wire.ServerInfoRequest(), TestContext.Current.CancellationToken));

        Assert.Single(handler.Requests);
        Assert.Equal(0, handler.Disposals);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task DisposingCancelsACallInFlightAndWaitsForIt(ClientWire wire)
    {
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var finished = 0;
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                started.SetResult();
                try
                {
                    await Task.Delay(Timeout.Infinite, token);
                    throw new InvalidOperationException("unreachable");
                }
                finally
                {
                    Interlocked.Exchange(ref finished, 1);
                }
            },
        };
        var connection = await FakeConnection.OpenAsync(handler);

        var call = connection.GetSymbolAsync();
        await started.Task.WaitAsync(TestContext.Current.CancellationToken);
        await connection.DisposeAsync();

        Assert.Equal(1, Volatile.Read(ref finished));
        var error = await Assert.ThrowsAsync<ObjectDisposedException>(() => call);
        Assert.Contains("its connection was disposed", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task DisposingTwiceOrConcurrentlyClosesOnceAndNeverThrows()
    {
        var transport = new FakeTransport((_, _, _) => Task.FromResult<object>(new Wire.ServerInfoResponse()));
        var connection = await FakeTransport.OpenAsync(transport, new ConnectionOptions());

        await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => Task.Run(async () => await connection.DisposeAsync())));
        await connection.DisposeAsync();

        Assert.Equal(1, transport.Disposals);
    }

    [Fact]
    public async Task AFailureToCloseIsLoggedNotThrown()
    {
        var logger = new ListLogger();
        var transport = new FakeTransport((_, _, _) => Task.FromResult<object>(new Wire.ServerInfoResponse()))
        {
            DisposeFailure = new InvalidOperationException("channel would not close"),
        };
        var connection = await FakeTransport.OpenAsync(transport, new ConnectionOptions { Logger = logger });

        await connection.DisposeAsync();

        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.Contains("Closing the connection to http://fake.invalid:1/ failed", warning.Message, StringComparison.Ordinal);
        Assert.Equal("channel would not close", warning.Exception?.Message);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ABorrowedHandlerOutlivesEveryConnectionThroughIt(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(wire, new Wire.SymbolResponse())) };
        var first = await FakeConnection.OpenAsync(handler);
        await using var second = await FakeConnection.OpenAsync(handler);

        await first.DisposeAsync();
        await second.GetSymbolAsync();

        Assert.Equal(0, handler.Disposals);
    }
}

[Trait(Category.Name, Category.Unit)]
public sealed class RawCallTests
{
    public static MatrixTheoryData<ClientWire, string> QualifiedAndBare =>
        new([ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson], ["GetSymbol", "sysml.SysMLService/GetSymbol"]);

    public static MatrixTheoryData<ClientWire, string> NotMethods =>
        new([ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson], ["GetSymbols", "getSymbol", "/sysml.SysMLService/GetSymbol", "other.Service/GetSymbol"]);

    [Theory]
    [MemberData(nameof(QualifiedAndBare))]
    public async Task ARawCallNamesItsMethodBareOrQualifiedAndReturnsTheResponseUninterpreted(ClientWire wire, string method)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(wire, new Wire.SymbolResponse { Error = "symbol not found: A" })) };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var request = new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" };

        var response = await connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(method, request, TestContext.Current.CancellationToken);

        Assert.Equal("symbol not found: A", response.Error);
        Assert.Equal(new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" }, request);
    }

    [Theory]
    [MemberData(nameof(NotMethods))]
    public async Task AnUnknownMethodSendsNothing(ClientWire wire, string method)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(method, new Wire.GetSymbolRequest(), TestContext.Current.CancellationToken));

        Assert.Contains("is not a method of sysml.SysMLService", error.Message, StringComparison.Ordinal);
        Assert.Single(handler.Requests);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task WrongMessageTypesSendNothing(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var request = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.ParseFileResponse>("ParseFile", new Wire.GetSymbolRequest(), TestContext.Current.CancellationToken));
        var response = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.ParseFileResponse>("GetSymbol", new Wire.GetSymbolRequest(), TestContext.Current.CancellationToken));

        Assert.Contains("sysml.SysMLService/ParseFile takes OpenMBEE.OpenSysML.Wire.ParseFileRequest", request.Message, StringComparison.Ordinal);
        Assert.Contains("OpenMBEE.OpenSysML.Wire.ParseFileResponse", response.Message, StringComparison.Ordinal);
        Assert.Single(handler.Requests);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task NullArgumentsAreRefused(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);

        await Assert.ThrowsAsync<ArgumentNullException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(null!, new Wire.GetSymbolRequest(), TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>("GetSymbol", null!, TestContext.Current.CancellationToken));
        Assert.Single(handler.Requests);
    }
}
