using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// A <c>sysml-grpc</c> this process started as its child, on a port the kernel chose, which only this
/// process is told of. It is started with <c>-exit-with-parent</c>, so it exits at end of file on its
/// input; this class holds the only write end of that pipe and never writes to it. Whatever ends this
/// process, the kernel closes the pipe and the child exits, so no exit handler is needed to avoid an
/// orphan. Its output is read for as long as it runs, keeping the last lines for error messages, so a
/// chatty child never blocks on a full pipe.
/// </summary>
internal sealed partial class PrivateService : IServiceProcess
{
    /// <summary>Lines of the child's output kept for error messages.</summary>
    internal const int TailLines = 20;

    /// <summary>Characters of one line of output kept; the rest of the line is dropped.</summary>
    internal const int MaxLineLength = 1024;

    private const string Truncated = "…";
    private static readonly TimeSpan KillWait = TimeSpan.FromSeconds(10);
    private static readonly TimeSpan DrainWait = TimeSpan.FromSeconds(2);

    private readonly Process process;
    private readonly string binary;
    private readonly TimeSpan shutdownTimeout;
    private readonly ILogger logger;
    private readonly TaskCompletionSource<string?> firstLine = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private readonly Queue<string> tail = new();
    private readonly Lock tailLock = new();
    private readonly Task pumps;
    private Task? stopping;
    private int inputClosed;
    private int killed;

    private PrivateService(Process process, ServiceIdentity identity, ILogger logger)
    {
        this.process = process;
        binary = identity.Path;
        shutdownTimeout = identity.ShutdownTimeout;
        this.logger = logger;
        ProcessId = process.Id;
        pumps = Task.WhenAll(
            Pump(process.StandardOutput.BaseStream, "stdout", reportsAddress: true),
            Pump(process.StandardError.BaseStream, "stderr", reportsAddress: false));
        Exited = WatchExitAsync();
    }

    /// <summary>The arguments every private service is started with, after the binary.</summary>
    internal static IReadOnlyList<string> Arguments { get; } = ["-port", "0", "-health-port", "0", "-report-address", "-exit-with-parent"];

    /// <summary>How long to wait before each retry of a start the kernel refused because the binary was open for writing.</summary>
    internal static IReadOnlyList<TimeSpan> BusyRetryDelays { get; } =
        [TimeSpan.FromMilliseconds(50), TimeSpan.FromMilliseconds(100), TimeSpan.FromMilliseconds(200), TimeSpan.FromMilliseconds(400), TimeSpan.FromMilliseconds(800)];

    /// <inheritdoc/>
    public Uri Address { get; private set; } = null!;

    /// <inheritdoc/>
    public int ProcessId { get; }

    /// <inheritdoc/>
    public Task<string> Exited { get; }

    /// <summary>The write end of the child's input, which nothing in the client writes to; the orphan probe's leak control duplicates it.</summary>
    internal Stream Input => process.StandardInput.BaseStream;

    /// <summary>The last lines the child wrote, at most <see cref="TailLines"/>, each at most <see cref="MaxLineLength"/> characters and a mark.</summary>
    internal IReadOnlyList<string> Tail
    {
        get
        {
            lock (tailLock)
            {
                return [.. tail];
            }
        }
    }

    /// <summary>
    /// Starts a private service and waits for the address it reports. Any failure, cancellation
    /// included, stops the child, drains its output and leaves no process behind.
    /// </summary>
    /// <param name="identity">The binary and the limits to start it under.</param>
    /// <param name="logger">Where its life is logged.</param>
    /// <param name="cancellationToken">Cancels the start.</param>
    /// <returns>The running service.</returns>
    /// <exception cref="ServiceStartException">It could not be started, or did not report an address it can be reached at in time.</exception>
    /// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
    internal static async Task<IServiceProcess> StartAsync(ServiceIdentity identity, ILogger logger, CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var process = await LaunchAsync(identity.Path, logger, cancellationToken).ConfigureAwait(false);
        var service = new PrivateService(process, identity, logger);
        LogStarting(logger, identity.Path, service.ProcessId);
        string? failure;
        try
        {
            failure = await service.AwaitAddressAsync(identity.StartupTimeout, cancellationToken).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            await service.StopQuietlyAsync().ConfigureAwait(false);
            throw;
        }

        if (failure is null)
        {
            LogStarted(logger, identity.Path, service.ProcessId, service.Address);
            return service;
        }

        await service.StopQuietlyAsync().ConfigureAwait(false);
        var ending = service.Exited.IsCompleted ? $" It {await service.Exited.ConfigureAwait(false)}." : string.Empty;
        throw new ServiceStartException($"sysml-grpc at {identity.Path} (process {service.ProcessId}) {failure}.{ending}{service.DescribeTail()}");
    }

    /// <summary>
    /// Starts the process. Linux refuses to execute a file any process holds open for writing
    /// (<c>ETXTBSY</c>), and a child another thread forks holds a copy of every descriptor this process has
    /// open until it executes, so a binary this process has just written, as the cache does, can be briefly
    /// busy: that refusal alone is retried, after <see cref="BusyRetryDelays"/>.
    /// </summary>
    private static async Task<Process> LaunchAsync(string path, ILogger logger, CancellationToken cancellationToken)
    {
        for (var attempt = 0; ; attempt++)
        {
            var process = new Process { StartInfo = StartInfo(path) };
            try
            {
                process.Start();
                return process;
            }
            catch (Win32Exception error) when (IsTextFileBusy(error) && attempt < BusyRetryDelays.Count)
            {
                process.Dispose();
                LogBusy(logger, path, BusyRetryDelays[attempt]);
                await Task.Delay(BusyRetryDelays[attempt], cancellationToken).ConfigureAwait(false);
            }
            catch (Exception error) when (error is Win32Exception or InvalidOperationException or PlatformNotSupportedException)
            {
                process.Dispose();
                throw new ServiceStartException($"sysml-grpc at {path} could not be started: {error.Message}", error);
            }
        }
    }

    // ETXTBSY is 26 on Linux, macOS and the BSDs; Windows reports its own error codes.
    private static bool IsTextFileBusy(Win32Exception error) => !OperatingSystem.IsWindows() && error.NativeErrorCode == 26;

    private static ProcessStartInfo StartInfo(string path)
    {
        var start = new ProcessStartInfo(path)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var argument in Arguments)
        {
            start.ArgumentList.Add(argument);
        }

        // The child serves loopback for this process alone: a token meant for a named service must not become one it demands.
        RemoveToken(start.Environment, OperatingSystem.IsWindows());
        return start;
    }

    /// <summary>
    /// Removes <see cref="ConnectionOptions.TokenVariable"/> from a child's environment: on Windows under any case,
    /// since names are matched that way there and the service reads the variable so, elsewhere only the exact name.
    /// </summary>
    /// <param name="environment">The child's environment, which a <see cref="ProcessStartInfo"/> fills from this process's.</param>
    /// <param name="windows">Whether the child runs on Windows.</param>
    internal static void RemoveToken(IDictionary<string, string?> environment, bool windows)
    {
        var comparison = windows ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal;
        foreach (var name in environment.Keys.Where(name => string.Equals(name, ConnectionOptions.TokenVariable, comparison)).ToList())
        {
            environment.Remove(name);
        }
    }

    /// <inheritdoc/>
    public Task StopAsync()
    {
        if (Volatile.Read(ref stopping) is { } started)
        {
            return started;
        }

        var stop = new Task<Task>(StopCoreAsync);
        var unwrapped = stop.Unwrap();
        if (Interlocked.CompareExchange(ref stopping, unwrapped, null) is { } winner)
        {
            return winner;
        }

        stop.Start(TaskScheduler.Default);
        return unwrapped;
    }

    /// <inheritdoc/>
    public void CloseInput()
    {
        if (Interlocked.Exchange(ref inputClosed, 1) != 0)
        {
            return;
        }

        try
        {
            process.StandardInput.Close();
        }
        catch (Exception error) when (error is IOException or ObjectDisposedException)
        {
            // The child has already gone, taking the other end of the pipe with it.
        }
    }

    /// <inheritdoc/>
    public string DescribeTail()
    {
        var lines = Tail;
        return lines.Count == 0 ? string.Empty : "\n  it last wrote:\n    " + string.Join("\n    ", lines);
    }

    /// <summary>Waits for the first line of output and reads it as an address.</summary>
    /// <returns>Null when the address was read; else why the start failed.</returns>
    private async Task<string?> AwaitAddressAsync(TimeSpan startupTimeout, CancellationToken cancellationToken)
    {
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(startupTimeout);
        string? line;
        try
        {
            line = await firstLine.Task.WaitAsync(timeout.Token).ConfigureAwait(false);
        }
        catch (OperationCanceledException)
        {
            cancellationToken.ThrowIfCancellationRequested();
            return $"did not report the address it listens on within {startupTimeout}, and was stopped";
        }

        if (line is null)
        {
            return "ended its output without reporting the address it listens on";
        }

        Uri address;
        try
        {
            address = ServiceAddress.Parse(line, "The address sysml-grpc reported");
        }
        catch (InvalidRequestException)
        {
            return $"reported \"{line}\" where the address it listens on belongs, and was stopped";
        }

        if (!address.IsLoopback)
        {
            return $"reported {line}, which is not a loopback address only this process reaches, and was stopped";
        }

        Address = address;
        return null;
    }

    private async Task StopQuietlyAsync()
    {
        try
        {
            await StopAsync().ConfigureAwait(false);
        }
#pragma warning disable CA1031 // The start's own failure is the one reported; this one is logged.
        catch (Exception error)
#pragma warning restore CA1031
        {
            LogStopFailed(logger, ProcessId, error);
        }
    }

    private async Task StopCoreAsync()
    {
        CloseInput();
        try
        {
            await Exited.WaitAsync(shutdownTimeout).ConfigureAwait(false);
        }
        catch (TimeoutException)
        {
            Volatile.Write(ref killed, 1);
            LogKilling(logger, ProcessId, shutdownTimeout);
            Exception? killFailure = null;
            try
            {
                process.Kill(entireProcessTree: true);
            }
            catch (Exception error) when (error is Win32Exception or InvalidOperationException or NotSupportedException or AggregateException)
            {
                killFailure = error;
            }

            try
            {
                await Exited.WaitAsync(KillWait).ConfigureAwait(false);
            }
            catch (TimeoutException)
            {
                throw new OpenSysMLException(
                    $"The private sysml-grpc (process {ProcessId}, {binary}) did not exit within {shutdownTimeout} of its input closing, nor within {KillWait} of being killed.",
                    killFailure);
            }
        }

        try
        {
            await pumps.WaitAsync(DrainWait).ConfigureAwait(false);
        }
        catch (TimeoutException)
        {
            // A process outside the tree that was killed still holds the output open; it is left to end.
            LogUndrained(logger, ProcessId, DrainWait);
        }

        var ending = await Exited.ConfigureAwait(false);
        LogStopped(logger, ProcessId, ending);
        process.Dispose();
    }

    private async Task<string> WatchExitAsync()
    {
        await process.WaitForExitAsync(CancellationToken.None).ConfigureAwait(false);
        return Volatile.Read(ref killed) != 0
            ? $"was killed, not having exited within {shutdownTimeout} of its input closing"
            : $"exited with code {process.ExitCode}";
    }

    private Task Pump(Stream stream, string name, bool reportsAddress) =>
        Task.Factory.StartNew(
            () => Read(stream, name, reportsAddress),
            CancellationToken.None,
            TaskCreationOptions.LongRunning | TaskCreationOptions.DenyChildAttach,
            TaskScheduler.Default);

    private void Read(Stream stream, string name, bool reportsAddress)
    {
        var decoder = Encoding.UTF8.GetDecoder();
        var bytes = new byte[8192];
        var chars = new char[Encoding.UTF8.GetMaxCharCount(bytes.Length)];
        var line = new StringBuilder();
        var truncated = false;
        var awaitingAddress = reportsAddress;
        try
        {
            int read;
            while ((read = stream.Read(bytes)) > 0)
            {
                var count = decoder.GetChars(bytes, 0, read, chars, 0, flush: false);
                for (var i = 0; i < count; i++)
                {
                    if (chars[i] == '\n')
                    {
                        Emit(line, truncated, name, ref awaitingAddress);
                        line.Clear();
                        truncated = false;
                    }
                    else if (line.Length < MaxLineLength)
                    {
                        line.Append(chars[i]);
                    }
                    else
                    {
                        truncated = true;
                    }
                }
            }
        }
        catch (Exception error) when (error is IOException or ObjectDisposedException)
        {
            // The pipe broke as the child went; that ends its output.
        }
        finally
        {
            if (line.Length > 0 || truncated)
            {
                Emit(line, truncated, name, ref awaitingAddress);
            }

            if (awaitingAddress)
            {
                firstLine.TrySetResult(null);
            }
        }
    }

    private void Emit(StringBuilder line, bool truncated, string name, ref bool awaitingAddress)
    {
        var text = line.ToString().TrimEnd('\r') + (truncated ? Truncated : string.Empty);
        if (awaitingAddress)
        {
            awaitingAddress = false;
            firstLine.TrySetResult(text);
            return;
        }

        lock (tailLock)
        {
            if (tail.Count == TailLines)
            {
                tail.Dequeue();
            }

            tail.Enqueue(name == "stderr" ? text : $"({name}) {text}");
        }

        if (logger.IsEnabled(LogLevel.Trace))
        {
            LogOutput(logger, ProcessId, name, text);
        }
    }

    [LoggerMessage(Level = LogLevel.Debug, Message = "Starting the private service {Path} as process {ProcessId}")]
    private static partial void LogStarting(ILogger logger, string path, int processId);

    [LoggerMessage(Level = LogLevel.Debug, Message = "The private service {Path} is open for writing, which the kernel refuses to execute; retrying in {Delay}")]
    private static partial void LogBusy(ILogger logger, string path, TimeSpan delay);

    [LoggerMessage(Level = LogLevel.Debug, Message = "The private service {Path}, process {ProcessId}, listens at {Address}")]
    private static partial void LogStarted(ILogger logger, string path, int processId, Uri address);

    [LoggerMessage(Level = LogLevel.Trace, Message = "sysml-grpc process {ProcessId} {Stream}: {Line}")]
    private static partial void LogOutput(ILogger logger, int processId, string stream, string line);

    [LoggerMessage(Level = LogLevel.Warning, Message = "The private service, process {ProcessId}, did not exit within {Grace} of its input closing; killing it and every process it started")]
    private static partial void LogKilling(ILogger logger, int processId, TimeSpan grace);

    [LoggerMessage(Level = LogLevel.Warning, Message = "The output of the private service, process {ProcessId}, was still open {Wait} after it exited; it is left to end")]
    private static partial void LogUndrained(ILogger logger, int processId, TimeSpan wait);

    [LoggerMessage(Level = LogLevel.Debug, Message = "The private service, process {ProcessId}, stopped: it {Ending}")]
    private static partial void LogStopped(ILogger logger, int processId, string ending);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Stopping the private service, process {ProcessId}, failed")]
    private static partial void LogStopFailed(ILogger logger, int processId, Exception error);
}
