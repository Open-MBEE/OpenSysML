using System;
using System.Collections.Concurrent;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Runtime.Versioning;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Win32.SafeHandles;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The shared cache, held against other threads, other copies of this client in this process, other
/// processes and the other clients. Three locks are taken in order and released in reverse:
/// <list type="number">
/// <item>a semaphore of this copy of the client, so its threads wait without polling;</item>
/// <item>the <see cref="CacheLayout.Guard"/>, held with <c>flock</c> on POSIX and an unshared open on Windows,
/// both of which belong to the open file, so another copy of this client in another load context is shut out
/// before it opens the lock file, whose closing would drop this process's lock;</item>
/// <item>the <see cref="CacheLayout.Lock"/>, held as the Python, Java and Rust clients hold it: <c>lockf</c> on a
/// descriptor libc opened, never a <see cref="FileStream"/>'s, on POSIX, and <c>LockFileEx</c> over the whole
/// file on Windows.</item>
/// </list>
/// Each is attempted without blocking and retried with a growing pause, so a wait can be cancelled, and gives
/// up after a set time. The lock is not reentrant: a holder that acquires it again waits for itself until that
/// time. Descriptors are opened close-on-exec, so no process started while it is held inherits it, and the
/// kernel releases everything when the process ends, however it ends.
/// </summary>
internal sealed partial class CacheLock : IAsyncDisposable, IDisposable
{
    /// <summary>How long an acquisition waits for another holder by default, as long as the Java client waits.</summary>
    internal static readonly TimeSpan DefaultWait = TimeSpan.FromMinutes(2);

    /// <summary>How long an acquisition waits before it warns that another holder is keeping it waiting.</summary>
    internal static readonly TimeSpan PatienceBeforeWarning = TimeSpan.FromSeconds(2);

    private static readonly ConcurrentDictionary<string, SemaphoreSlim> Threads = new(StringComparer.Ordinal);

    private readonly SemaphoreSlim threads;
    private readonly ILogger logger;
    private SafeFileHandle? guard;
    private SafeFileHandle? shared;
    private int released;

    private CacheLock(CacheLayout layout, SemaphoreSlim threads, ILogger logger)
    {
        Layout = layout;
        this.threads = threads;
        this.logger = logger;
    }

    /// <summary>The cache held.</summary>
    internal CacheLayout Layout { get; }

    /// <summary>Takes the lock, waiting for whoever holds it.</summary>
    /// <param name="layout">The cache.</param>
    /// <param name="wait">How long to wait for another holder; <see cref="TimeSpan.Zero"/> to try once.</param>
    /// <param name="logger">Where waiting and holding are logged.</param>
    /// <param name="cancellationToken">Cancels the wait, leaving nothing held.</param>
    /// <returns>The held lock, which the caller disposes to release it.</returns>
    /// <exception cref="ServiceStartException">Another holder kept it for all of <paramref name="wait"/>.</exception>
    /// <exception cref="IOException">The cache directory or a lock file could not be made or opened.</exception>
    /// <exception cref="UnauthorizedAccessException">The cache directory or a lock file may not be made or opened.</exception>
    /// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
    internal static async Task<CacheLock> AcquireAsync(CacheLayout layout, TimeSpan wait, ILogger logger, CancellationToken cancellationToken)
    {
        ArgumentOutOfRangeException.ThrowIfLessThan(wait, TimeSpan.Zero);
        var watch = Stopwatch.StartNew();
        var threads = Threads.GetOrAdd(layout.Lock, _ => new SemaphoreSlim(1, 1));
        if (!await threads.WaitAsync(wait, cancellationToken).ConfigureAwait(false))
        {
            throw Busy(layout, wait, "another thread of this process");
        }

        var held = new CacheLock(layout, threads, logger);
        try
        {
            Directory.CreateDirectory(layout.Directory);
            held.guard = OperatingSystem.IsWindows() ? null : OpenOnPosix(layout.Guard);
            await PollAsync(held.TryGuard, "another copy of the .NET client", layout, wait, watch, logger, cancellationToken).ConfigureAwait(false);
            held.shared = OperatingSystem.IsWindows()
                ? File.OpenHandle(layout.Lock, FileMode.OpenOrCreate, FileAccess.ReadWrite, FileShare.ReadWrite | FileShare.Delete)
                : OpenOnPosix(layout.Lock);
            await PollAsync(held.TrySharedLock, "another process", layout, wait, watch, logger, cancellationToken).ConfigureAwait(false);
        }
        catch
        {
            held.Release();
            throw;
        }

        LogLocked(logger, layout.Lock);
        return held;
    }

    /// <summary>Releases the lock; nothing happens after the first time.</summary>
    public void Dispose() => Release();

    /// <summary>Releases the lock; nothing happens after the first time.</summary>
    /// <returns>A completed task.</returns>
    public ValueTask DisposeAsync()
    {
        Release();
        return ValueTask.CompletedTask;
    }

    private static async Task PollAsync(Func<bool> attempt, string holder, CacheLayout layout, TimeSpan wait, Stopwatch watch, ILogger logger, CancellationToken cancellationToken)
    {
        var backoff = new Backoff();
        var warned = false;
        while (!attempt())
        {
            var remaining = wait - watch.Elapsed;
            if (remaining <= TimeSpan.Zero)
            {
                throw Busy(layout, wait, holder);
            }

            if (!warned && watch.Elapsed >= PatienceBeforeWarning)
            {
                LogWaiting(logger, layout.Lock, holder);
                warned = true;
            }

            var pause = backoff.Next();
            await Task.Delay(pause < remaining ? pause : remaining, cancellationToken).ConfigureAwait(false);
        }
    }

    // The file is made by a .NET open with O_EXCL, which fails before any implicit flock when the file is
    // there; the descriptor that is kept is then libc's own.
    [UnsupportedOSPlatform("windows")]
    private static SafeFileHandle OpenOnPosix(string path)
    {
        try
        {
            using var created = new FileStream(path, new FileStreamOptions
            {
                Mode = FileMode.CreateNew,
                Access = FileAccess.Write,
                Share = FileShare.ReadWrite | FileShare.Delete,
                UnixCreateMode = UnixFileMode.UserRead | UnixFileMode.UserWrite,
            });
        }
        catch (IOException)
        {
            // It exists, or another opener holds it while making it; open(2) reports whatever else this was.
        }

        return CacheNative.OpenDescriptor(path);
    }

    private static ServiceStartException Busy(CacheLayout layout, TimeSpan wait, string holder) => new(
        $"The shared sysml-grpc cache at {layout.Directory} is locked by {holder}, which held {layout.Lock} for all of the "
        + $"{wait.TotalSeconds.ToString("0.###", CultureInfo.InvariantCulture)} s waited.\n"
        + "  fix: let the other client finish installing a release there, or end the process holding the lock; a holder that ends releases it, however it ends.");

    private bool TryGuard()
    {
        if (guard is not null)
        {
            return CacheNative.TryFlock(guard, Layout.Guard);
        }

        try
        {
            guard = File.OpenHandle(Layout.Guard, FileMode.OpenOrCreate, FileAccess.ReadWrite, FileShare.None);
            return true;
        }
        catch (IOException error) when (error.HResult == unchecked((int)0x80070020))
        {
            return false;
        }
    }

    private bool TrySharedLock() =>
        OperatingSystem.IsWindows() ? CacheNative.TryLockFileEx(shared!, Layout.Lock) : CacheNative.TryLockf(shared!, Layout.Lock);

    private void Release()
    {
        if (Interlocked.Exchange(ref released, 1) != 0)
        {
            return;
        }

        if (shared is not null)
        {
            if (OperatingSystem.IsWindows())
            {
                CacheNative.UnlockFileEx(shared);
            }
            else
            {
                CacheNative.UnlockLockf(shared);
            }

            shared.Dispose();
        }

        if (guard is not null)
        {
            if (!OperatingSystem.IsWindows())
            {
                CacheNative.UnlockFlock(guard);
            }

            guard.Dispose();
        }

        threads.Release();
        LogReleased(logger, Layout.Lock);
    }

    [LoggerMessage(Level = LogLevel.Debug, Message = "Locked the shared sysml-grpc cache: {Lock}")]
    private static partial void LogLocked(ILogger logger, string @lock);

    [LoggerMessage(Level = LogLevel.Debug, Message = "Released the shared sysml-grpc cache: {Lock}")]
    private static partial void LogReleased(ILogger logger, string @lock);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Waiting for {Lock}, which {Holder} holds while it changes the shared sysml-grpc cache")]
    private static partial void LogWaiting(ILogger logger, string @lock, string holder);

    /// <summary>The pauses between attempts: doubling from 10 ms to at most 250 ms.</summary>
    internal struct Backoff
    {
        /// <summary>The first pause.</summary>
        internal static readonly TimeSpan First = TimeSpan.FromMilliseconds(10);

        /// <summary>The longest pause, which bounds how late a waiter sees a release.</summary>
        internal static readonly TimeSpan Longest = TimeSpan.FromMilliseconds(250);

        private TimeSpan next;

        /// <summary>The next pause.</summary>
        /// <returns>How long to wait before the next attempt.</returns>
        internal TimeSpan Next()
        {
            var pause = next == TimeSpan.Zero ? First : next;
            next = pause * 2 < Longest ? pause * 2 : Longest;
            return pause;
        }
    }
}
