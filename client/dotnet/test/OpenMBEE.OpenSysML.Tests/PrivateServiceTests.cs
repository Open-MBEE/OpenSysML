using System;
using System.ComponentModel;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// One private service's life, against the FakeService stand-in: its exact arguments, the first line
/// as its address, every way a start fails, bounded reading of a chatty child, and stopping it in
/// order or by force. Every test leaves no process behind.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class PrivateServiceTests
{
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    public async Task ItIsStartedWithTheExactArgumentsAndTheFirstLineIsItsAddress()
    {
        using var child = FakeChild.Serving();

        var service = await StartAsync(child);
        Assert.Equal(new Uri("http://127.0.0.1:50051/"), service.Address);
        Assert.True(FakeChild.IsRunning(service.ProcessId));
        Assert.Equal(["-port", "0", "-health-port", "0", "-report-address", "-exit-with-parent"], await File.ReadAllLinesAsync(child.ArgumentsFile, Token));
        Assert.Equal(PrivateService.Arguments, await File.ReadAllLinesAsync(child.ArgumentsFile, Token));

        await service.StopAsync();

        Assert.Equal("exited with code 0", await service.Exited);
        Assert.True(File.Exists(child.ExitMarker), "it exits in order when its input closes");
        await FakeChild.AssertGoneAsync(service.ProcessId);
    }

    [Theory]
    [InlineData("listening on 127.0.0.1:50051", "reported \"listening on 127.0.0.1:50051\" where the address it listens on belongs")]
    [InlineData("127.0.0.1", "reported \"127.0.0.1\" where the address it listens on belongs")]
    [InlineData("http://127.0.0.1:1/path", "reported \"http://127.0.0.1:1/path\" where the address it listens on belongs")]
    [InlineData("", "reported \"\" where the address it listens on belongs")]
    [InlineData("192.0.2.7:50051", "reported 192.0.2.7:50051, which is not a loopback address")]
    [InlineData("example.com:50051", "reported example.com:50051, which is not a loopback address")]
    public async Task AFirstLineThatIsNoLoopbackAddressFailsTheStartAndStopsTheChild(string report, string expected)
    {
        using var child = FakeChild.Create(fake => new FakeScript { Report = report, ExitMarker = fake.ExitMarker });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => StartAsync(child));

        Assert.Contains(expected, error.Message, StringComparison.Ordinal);
        Assert.Contains(child.Path, error.Message, StringComparison.Ordinal);
        Assert.True(File.Exists(child.ExitMarker), "the child is stopped in order");
        await FakeChild.AssertGoneAsync(ProcessIdIn(error));
    }

    [Fact]
    public async Task AChildThatExitsBeforeReportingFailsWithItsExitAndWhatItLastWrote()
    {
        using var child = FakeChild.Create(_ => new FakeScript { Stderr = "flag provided but not defined: -report-address", ExitCode = 2 });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => StartAsync(child));

        Assert.Contains("ended its output without reporting the address it listens on. It exited with code 2.", error.Message, StringComparison.Ordinal);
        Assert.Contains("it last wrote:\n    flag provided but not defined: -report-address", error.Message, StringComparison.Ordinal);
        await FakeChild.AssertGoneAsync(ProcessIdIn(error));
    }

    [Fact]
    public async Task AChildThatClosesItsOutputButLingersIsKilledOnceItsGraceIsOver()
    {
        using var child = FakeChild.Create(_ => new FakeScript { CloseStdout = true, IgnoreStdinEnd = true });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => StartAsync(child, shutdown: TimeSpan.FromMilliseconds(300)));

        Assert.Contains("ended its output without reporting", error.Message, StringComparison.Ordinal);
        Assert.Contains("It was killed, not having exited within 00:00:00.3000000 of its input closing.", error.Message, StringComparison.Ordinal);
        await FakeChild.AssertGoneAsync(ProcessIdIn(error));
    }

    [Fact]
    public async Task AChildThatReportsNothingInTimeIsStopped()
    {
        using var child = FakeChild.Create(fake => new FakeScript { Stderr = "loading", ExitMarker = fake.ExitMarker });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => StartAsync(child, startup: TimeSpan.FromMilliseconds(500)));

        Assert.Contains("did not report the address it listens on within 00:00:00.5000000, and was stopped. It exited with code 0.", error.Message, StringComparison.Ordinal);
        Assert.Contains("loading", error.Message, StringComparison.Ordinal);
        Assert.True(File.Exists(child.ExitMarker));
        await FakeChild.AssertGoneAsync(ProcessIdIn(error));
    }

    [Fact]
    public async Task CancellingTheStartStopsTheChildAndCarriesTheCallersToken()
    {
        using var child = FakeChild.Create(fake => new FakeScript { ArgumentsFile = fake.ArgumentsFile, ExitMarker = fake.ExitMarker });
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);

        var starting = PrivateService.StartAsync(Identity(child), NullLogger.Instance, cancel.Token);
        await ServiceRegistryTests.WaitUntilAsync(() => File.Exists(child.ArgumentsFile));
        await cancel.CancelAsync();

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => starting);
        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.True(File.Exists(child.ExitMarker), "the start does not return before the child is stopped");
    }

    [Fact]
    public async Task AChattyChildBlocksNeitherItsStartNorItsStopAndOnlyItsLastLinesAreKept()
    {
        using var child = FakeChild.Create(_ => new FakeScript { StderrBytesBeforeReport = 4 * 1024 * 1024, Report = "127.0.0.1:50051", FloodAfterReport = true });

        var service = (PrivateService)await StartAsync(child, startup: TimeSpan.FromSeconds(20));
        await Task.Delay(300, Token);

        Assert.Equal(PrivateService.TailLines, service.Tail.Count);
        Assert.All(service.Tail, line => Assert.True(line.Length <= PrivateService.MaxLineLength + "(stdout) ".Length, $"a kept line of {line.Length} characters"));
        await service.StopAsync();
        Assert.Equal("exited with code 0", await service.Exited);
        await FakeChild.AssertGoneAsync(service.ProcessId);
    }

    [Fact]
    public async Task AnEndlessLineIsKeptOnlyUpToItsBoundAndMarkedCut()
    {
        using var child = FakeChild.Create(_ => new FakeScript { StderrBytesBeforeReport = 1024 * 1024, FloodLineLength = 0, Report = "127.0.0.1:50051" });

        var service = (PrivateService)await StartAsync(child);
        await service.StopAsync();

        var line = Assert.Single(service.Tail);
        Assert.Equal(new string('x', PrivateService.MaxLineLength) + "…", line);
    }

    [Fact]
    public async Task AnOverlongFirstLineIsNoAddress()
    {
        using var child = FakeChild.Create(_ => new FakeScript { Report = "127.0.0.1:" + new string('1', 5000) });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => StartAsync(child));

        Assert.Contains("1…\" where the address it listens on belongs", error.Message, StringComparison.Ordinal);
        Assert.True(error.Message.Length < 2 * PrivateService.MaxLineLength, "the message quotes the line only up to its bound");
        await FakeChild.AssertGoneAsync(ProcessIdIn(error));
    }

    [Fact]
    public async Task AChildThatIgnoresItsInputIsKilledWithItsTreeAfterItsGrace()
    {
        using var child = FakeChild.Create(fake => new FakeScript { Report = "127.0.0.1:50051", IgnoreStdinEnd = true, ExitMarker = fake.ExitMarker });
        var service = await StartAsync(child, shutdown: TimeSpan.FromMilliseconds(300));

        await service.StopAsync();

        Assert.Equal("was killed, not having exited within 00:00:00.3000000 of its input closing", await service.Exited);
        Assert.False(File.Exists(child.ExitMarker));
        await FakeChild.AssertGoneAsync(service.ProcessId);
    }

    [Fact]
    public async Task AChildKilledFromOutsideIsSeenToHaveExited()
    {
        using var child = FakeChild.Serving();
        var service = await StartAsync(child);

        FakeChild.Kill(service.ProcessId);

        Assert.StartsWith("exited with code ", await service.Exited.WaitAsync(TimeSpan.FromSeconds(10), Token), StringComparison.Ordinal);
        await service.StopAsync();
    }

    [Fact]
    public async Task StoppingConcurrentlyAndAgainWaitsForTheOneStop()
    {
        using var child = FakeChild.Serving();
        var service = await StartAsync(child);

        var stops = Enumerable.Range(0, 8).Select(_ => Task.Run(service.StopAsync, Token)).ToArray();
        await Task.WhenAll(stops);
        await service.StopAsync();

        Assert.Same(service.StopAsync(), service.StopAsync());
        await FakeChild.AssertGoneAsync(service.ProcessId);
    }

    [Fact]
    public async Task ClosingItsInputEndsTheChildInOrder()
    {
        using var child = FakeChild.Serving();
        var service = await StartAsync(child);

        service.CloseInput();
        service.CloseInput();

        Assert.Equal("exited with code 0", await service.Exited.WaitAsync(TimeSpan.FromSeconds(10), Token));
        Assert.True(File.Exists(child.ExitMarker));
        await service.StopAsync();
    }

    [Fact]
    public async Task AFileThatCannotBeExecutedFailsTheStartWithTheOperatingSystemsReason()
    {
        using var child = FakeChild.Serving();
        var text = Path.Combine(child.Directory, OperatingSystem.IsWindows() ? "text.exe" : "text");
        await File.WriteAllTextAsync(text, "not a program", Token);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(text, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => PrivateService.StartAsync(Identity(child) with { Path = text }, NullLogger.Instance, Token));

        Assert.StartsWith($"sysml-grpc at {text} could not be started: ", error.Message, StringComparison.Ordinal);
        Assert.IsType<Win32Exception>(error.InnerException);
    }

    [Fact]
    public async Task AStartAlreadyCancelledStartsNothing()
    {
        using var child = FakeChild.Serving();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => PrivateService.StartAsync(Identity(child), NullLogger.Instance, new CancellationToken(canceled: true)));

        Assert.False(File.Exists(child.ArgumentsFile));
    }

    [Fact]
    public async Task ABinaryOpenForWritingIsStartedOnceItIsClosed()
    {
        Assert.SkipUnless(OperatingSystem.IsLinux(), "Only Linux refuses to execute a file that is open for writing.");
        using var child = FakeChild.Serving();
        var writer = new FileStream(child.Path, FileMode.Open, FileAccess.Write, FileShare.ReadWrite);
        var logged = 0;
        var logger = new ListLogger { Logged = _ => { if (Interlocked.Increment(ref logged) == 2) { writer.Dispose(); } } };

        IServiceProcess service;
        try
        {
            service = await PrivateService.StartAsync(Identity(child), logger, Token);
        }
        finally
        {
            await writer.DisposeAsync();
        }

        Assert.Equal(new Uri("http://127.0.0.1:50051/"), service.Address);
        var busy = logger.Entries.Where(e => e.Message.Contains("is open for writing", StringComparison.Ordinal)).ToList();
        Assert.Equal(2, busy.Count);
        Assert.All(busy, e => Assert.Equal(Microsoft.Extensions.Logging.LogLevel.Debug, e.Level));
        Assert.Contains(child.Path, busy[0].Message, StringComparison.Ordinal);
        await service.StopAsync();
        await FakeChild.AssertGoneAsync(service.ProcessId);
    }

    [Fact]
    public async Task ABinaryThatStaysOpenForWritingFailsTheStartWithTheKernelsReasonOnceTheRetriesAreSpent()
    {
        Assert.SkipUnless(OperatingSystem.IsLinux(), "Only Linux refuses to execute a file that is open for writing.");
        using var child = FakeChild.Serving();
        await using var writer = new FileStream(child.Path, FileMode.Open, FileAccess.Write, FileShare.ReadWrite);
        var logger = new ListLogger();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => PrivateService.StartAsync(Identity(child), logger, Token));

        Assert.StartsWith($"sysml-grpc at {child.Path} could not be started: ", error.Message, StringComparison.Ordinal);
        Assert.Equal(26, Assert.IsType<Win32Exception>(error.InnerException).NativeErrorCode);
        Assert.Equal(
            PrivateService.BusyRetryDelays.Select(delay => $"The private service {child.Path} is open for writing, which the kernel refuses to execute; retrying in {delay}"),
            logger.Entries.Select(e => e.Message));
        Assert.False(File.Exists(child.ArgumentsFile), "nothing ran");
    }

    [Fact]
    public async Task CancellingAStartWaitingOutABusyBinaryStartsNothing()
    {
        Assert.SkipUnless(OperatingSystem.IsLinux(), "Only Linux refuses to execute a file that is open for writing.");
        using var child = FakeChild.Serving();
        await using var writer = new FileStream(child.Path, FileMode.Open, FileAccess.Write, FileShare.ReadWrite);
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var logger = new ListLogger { Logged = _ => cancel.Cancel() };

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => PrivateService.StartAsync(Identity(child), logger, cancel.Token));

        Assert.Single(logger.Entries);
        await writer.DisposeAsync();
        Assert.False(File.Exists(child.ArgumentsFile), "nothing ran");
    }

    private static ServiceIdentity Identity(FakeChild child, TimeSpan? startup = null, TimeSpan? shutdown = null) =>
        new(child.Path, "unhashed", startup ?? TimeSpan.FromSeconds(30), shutdown ?? TimeSpan.FromSeconds(10), null);

    private static Task<IServiceProcess> StartAsync(FakeChild child, TimeSpan? startup = null, TimeSpan? shutdown = null) =>
        PrivateService.StartAsync(Identity(child, startup, shutdown), NullLogger.Instance, Token);

    private static int ProcessIdIn(ServiceStartException error) =>
        int.Parse(ProcessIdPattern().Match(error.Message).Groups[1].Value, System.Globalization.CultureInfo.InvariantCulture);

    [GeneratedRegex(@"\(process (\d+)\)")]
    private static partial Regex ProcessIdPattern();
}
