using System;
using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// One use of the shared cache with its <see cref="CacheLock"/> held throughout: what is there is read, a
/// verified release is installed and recorded, and the executable is linked under its own digest, all without
/// another client changing the cache in between. A transaction is used by one flow at a time and disposed to
/// release the lock.
/// </summary>
/// <remarks>
/// Every file is staged under a <see cref="CacheLayout.Temporary"/> name in the cache directory and published
/// by an atomic rename, so the executable, the record and a snapshot are each whole, old or new. The executable
/// is published before its record, and a record vouches for an executable only while its digest is the
/// executable's, so an interruption between the two leaves an executable no record vouches for rather than a
/// torn pair. Staged files a crashed transaction left are removed by the next one, which holds the lock that
/// every transaction creating them held.
/// </remarks>
internal sealed partial class CacheTransaction : IAsyncDisposable
{
    private readonly CacheLock held;
    private readonly ILogger logger;
    private readonly CacheSeams seams;
    private readonly List<string> staged = [];
    private int ended;

    private CacheTransaction(CacheLock held, ILogger logger, CacheSeams seams)
    {
        this.held = held;
        this.logger = logger;
        this.seams = seams;
    }

    /// <summary>The cache.</summary>
    internal CacheLayout Layout => held.Layout;

    /// <summary>Locks the cache and removes what an interrupted transaction left staged.</summary>
    /// <param name="layout">The cache.</param>
    /// <param name="logger">Where the transaction logs.</param>
    /// <param name="cancellationToken">Cancels waiting for the lock.</param>
    /// <param name="wait">How long to wait for another holder; <see cref="CacheLock.DefaultWait"/> when null.</param>
    /// <param name="seams">Where tests interpose; none when null.</param>
    /// <returns>The transaction, which the caller disposes.</returns>
    /// <exception cref="ServiceStartException">Another holder kept the lock for all of the wait.</exception>
    /// <exception cref="IOException">The cache or its lock files could not be made or opened.</exception>
    /// <exception cref="UnauthorizedAccessException">The cache or its lock files may not be made or opened.</exception>
    internal static async Task<CacheTransaction> BeginAsync(
        CacheLayout layout,
        ILogger logger,
        CancellationToken cancellationToken,
        TimeSpan? wait = null,
        CacheSeams? seams = null)
    {
        var held = await CacheLock.AcquireAsync(layout, wait ?? CacheLock.DefaultWait, logger, cancellationToken).ConfigureAwait(false);
        var transaction = new CacheTransaction(held, logger, seams ?? CacheSeams.None);
        try
        {
            transaction.RemoveLeftovers();
            await transaction.ReachAsync(CacheStep.Locked, cancellationToken).ConfigureAwait(false);
        }
        catch
        {
            await held.DisposeAsync().ConfigureAwait(false);
            throw;
        }

        return transaction;
    }

    /// <summary>What the cache holds: the executable's digest and the record beside it.</summary>
    /// <param name="cancellationToken">Cancels the hashing.</param>
    /// <returns>The contents.</returns>
    /// <exception cref="IOException">The executable could not be read.</exception>
    internal async Task<CacheContents> InspectAsync(CancellationToken cancellationToken)
    {
        ThrowIfEnded();
        var digest = File.Exists(Layout.Executable) ? await BinaryResolver.Sha256Async(Layout.Executable, cancellationToken).ConfigureAwait(false) : null;
        var record = CacheMetadata.Read(Layout.Metadata);
        return new CacheContents(digest, record.Metadata, record.Problem);
    }

    /// <summary>
    /// Writes a release to a staged file, verifies its digest, replaces the executable with it and records it.
    /// Bytes that do not have the digest required are never published, and the cache is left as it was.
    /// </summary>
    /// <param name="write">Writes the release to the stream it is given.</param>
    /// <param name="expectedSha256">The digest the release must have.</param>
    /// <param name="version">The release tag to record.</param>
    /// <param name="repo">The repository, <c>owner/repo</c>, to record.</param>
    /// <param name="cancellationToken">Cancels until the executable is published; the record then follows regardless.</param>
    /// <returns>What was installed.</returns>
    /// <exception cref="ChecksumMismatchException">What was written does not have <paramref name="expectedSha256"/>.</exception>
    /// <exception cref="ServiceStartException">The verified release could not replace the executable.</exception>
    internal async Task<CacheInstall> InstallAsync(
        Func<Stream, CancellationToken, Task> write,
        string expectedSha256,
        string version,
        string repo,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(write);
        ArgumentException.ThrowIfNullOrEmpty(version);
        ArgumentException.ThrowIfNullOrEmpty(repo);
        if (expectedSha256.Length != 64 || !CacheLayout.IsHex(expectedSha256))
        {
            throw new ArgumentException($"{expectedSha256} is not a SHA-256 digest.", nameof(expectedSha256));
        }

        ThrowIfEnded();
        var path = Stage(Layout.Executable);
        string actual;
        try
        {
            var file = new FileStream(path, Created(UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute));
            await using (file.ConfigureAwait(false))
            {
                await write(file, cancellationToken).ConfigureAwait(false);
                await file.FlushAsync(cancellationToken).ConfigureAwait(false);
                file.Flush(flushToDisk: true);
            }

            actual = await BinaryResolver.Sha256Async(path, cancellationToken).ConfigureAwait(false);
            if (!string.Equals(actual, expectedSha256, StringComparison.OrdinalIgnoreCase))
            {
                throw new ChecksumMismatchException(
                    Layout.Executable,
                    expectedSha256.ToLowerInvariant(),
                    actual,
                    $"the bytes received for {version} were discarded and the cache was left as it was; fetch the release again from a source you trust");
            }

            await ReachAsync(CacheStep.Staged, cancellationToken).ConfigureAwait(false);
            cancellationToken.ThrowIfCancellationRequested();
            try
            {
                File.Move(path, Layout.Executable, overwrite: true);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw new ServiceStartException(
                    $"sysml-grpc {version}, verified, could not replace {Layout.Executable}: {error.Message}\n"
                    + "  fix: stop whatever runs that file, which on Windows cannot be replaced while it runs, and try again; the cache was left as it was.",
                    error);
            }

            staged.Remove(path);
        }
        finally
        {
            RemoveStaged(path);
        }

        LogInstalled(logger, version, repo, Layout.Executable, actual);
        await ReachAsync(CacheStep.ExecutablePublished, CancellationToken.None).ConfigureAwait(false);
        var recorded = Record(new CacheMetadata(version, actual, repo));
        await ReachAsync(CacheStep.MetadataPublished, CancellationToken.None).ConfigureAwait(false);
        return new CacheInstall(actual, recorded);
    }

    /// <summary>
    /// Links the executable under its own digest, <see cref="CacheLayout.Snapshot"/>, where no install over the
    /// cache can change it, and returns that path to start. Symbolic links in the executable's path are followed;
    /// a snapshot already there is used only when it is not a symbolic link and has its digest.
    /// Where the file system cannot link, the executable is copied, with a warning; where it cannot
    /// be copied either, the executable itself is returned, with a warning that what starts may be a release
    /// installed over it afterwards.
    /// </summary>
    /// <param name="cancellationToken">Cancels the hashing.</param>
    /// <returns>The file to start and its digest, or null when the cache has no executable.</returns>
    /// <exception cref="IOException">The executable could not be read.</exception>
    internal async Task<CacheSnapshot?> SnapshotAsync(CancellationToken cancellationToken)
    {
        ThrowIfEnded();
        if (!File.Exists(Layout.Executable))
        {
            return null;
        }

        var source = BinaryResolver.Canonical(Layout.Executable);
        var digest = await BinaryResolver.Sha256Async(source, cancellationToken).ConfigureAwait(false);
        var target = Layout.Snapshot(digest);
        if (new FileInfo(target).LinkTarget is not null)
        {
            LogReplacingSnapshot(logger, target, "is a symbolic link");
        }
        else if (File.Exists(target))
        {
            var existing = await DigestOrNullAsync(target, cancellationToken).ConfigureAwait(false);
            if (existing == digest && Executable(target))
            {
                return new CacheSnapshot(target, digest, SnapshotKind.Reused);
            }

            LogReplacingSnapshot(logger, target, existing is null ? "could not be read" : existing == digest ? "could not be made executable" : $"has SHA-256 {existing}");
        }

        var path = Stage(target);
        try
        {
            SnapshotKind kind;
            try
            {
                (seams.HardLink ?? CacheNative.HardLink)(source, path);
                kind = SnapshotKind.Linked;
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException or PlatformNotSupportedException or DllNotFoundException or EntryPointNotFoundException)
            {
                LogCopying(logger, Layout.Executable, target, error.Message);
                try
                {
                    File.Copy(source, path);
                }
                catch (Exception copyError) when (copyError is IOException or UnauthorizedAccessException)
                {
                    return Unsnapshotted(digest, target, copyError.Message);
                }

                kind = SnapshotKind.Copied;
            }

            if (!Executable(path))
            {
                return Unsnapshotted(digest, target, "it could not be made executable");
            }

            if (await DigestOrNullAsync(path, cancellationToken).ConfigureAwait(false) != digest)
            {
                return Unsnapshotted(digest, target, "what was linked or copied does not have the executable's digest");
            }

            await ReachAsync(CacheStep.SnapshotStaged, cancellationToken).ConfigureAwait(false);
            try
            {
                File.Move(path, target, overwrite: true);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                return Unsnapshotted(digest, target, error.Message);
            }

            staged.Remove(path);
            return new CacheSnapshot(target, digest, kind);
        }
        finally
        {
            RemoveStaged(path);
        }
    }

    /// <summary>Removes whatever this transaction staged and did not publish, then releases the lock.</summary>
    /// <returns>When the lock is released.</returns>
    public async ValueTask DisposeAsync()
    {
        if (Interlocked.Exchange(ref ended, 1) != 0)
        {
            return;
        }

        foreach (var path in staged.ToArray())
        {
            RemoveStaged(path);
        }

        await held.DisposeAsync().ConfigureAwait(false);
    }

    private static FileStreamOptions Created(UnixFileMode mode)
    {
        var options = new FileStreamOptions { Mode = FileMode.CreateNew, Access = FileAccess.Write, Share = FileShare.None };
        if (!OperatingSystem.IsWindows())
        {
            options.UnixCreateMode = mode;
        }

        return options;
    }

    private static bool Executable(string path)
    {
        if (OperatingSystem.IsWindows())
        {
            return true;
        }

        try
        {
            var mode = File.GetUnixFileMode(path);
            if ((mode & UnixFileMode.UserExecute) == 0)
            {
                File.SetUnixFileMode(path, mode | UnixFileMode.UserExecute);
            }

            return true;
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            return false;
        }
    }

    private static async Task<string?> DigestOrNullAsync(string path, CancellationToken cancellationToken)
    {
        try
        {
            return await BinaryResolver.Sha256Async(path, cancellationToken).ConfigureAwait(false);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            return null;
        }
    }

    private CacheSnapshot Unsnapshotted(string digest, string target, string reason)
    {
        LogUnsnapshotted(logger, Layout.Executable, target, reason);
        return new CacheSnapshot(Layout.Executable, digest, SnapshotKind.Unsnapshotted);
    }

    private bool Record(CacheMetadata metadata)
    {
        var path = Stage(Layout.Metadata);
        try
        {
            using (var file = new FileStream(path, Created(UnixFileMode.UserRead | UnixFileMode.UserWrite)))
            {
                file.Write(metadata.Format());
                file.Flush(flushToDisk: true);
            }

            File.Move(path, Layout.Metadata, overwrite: true);
            staged.Remove(path);
            return true;
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            LogUnrecorded(logger, Layout.Executable, Layout.Metadata, error.Message);
            return false;
        }
        finally
        {
            RemoveStaged(path);
        }
    }

    private string Stage(string final)
    {
        var path = CacheLayout.Temporary(final);
        staged.Add(path);
        return path;
    }

    private void RemoveStaged(string path)
    {
        if (!staged.Remove(path))
        {
            return;
        }

        try
        {
            File.Delete(path);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            LogLeftBehind(logger, path, error.Message);
        }
    }

    private void RemoveLeftovers()
    {
        IEnumerable<string> files;
        try
        {
            files = Directory.EnumerateFiles(Layout.Directory);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            LogLeftBehind(logger, Layout.Directory, error.Message);
            return;
        }

        foreach (var file in files)
        {
            if (!CacheLayout.IsTemporary(Path.GetFileName(file)))
            {
                continue;
            }

            try
            {
                File.Delete(file);
                LogRemovedLeftover(logger, file);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                LogLeftBehind(logger, file, error.Message);
            }
        }
    }

    private Task ReachAsync(CacheStep step, CancellationToken cancellationToken) =>
        seams.Reached is { } reached ? reached(step, cancellationToken) : Task.CompletedTask;

    private void ThrowIfEnded() => ObjectDisposedException.ThrowIf(Volatile.Read(ref ended) != 0, this);

    [LoggerMessage(Level = LogLevel.Information, Message = "Installed sysml-grpc {Version} of {Repo} at {Path}, SHA-256 {Sha256}")]
    private static partial void LogInstalled(ILogger logger, string version, string repo, string path, string sha256);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Installed {Path}, but could not record which release it is in {Metadata}: {Reason}. No record vouches for it, so it is not taken for that release")]
    private static partial void LogUnrecorded(ILogger logger, string path, string metadata, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Replacing {Snapshot}, which {Reason}")]
    private static partial void LogReplacingSnapshot(ILogger logger, string snapshot, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Could not hard link {Executable} to {Snapshot} ({Reason}); copying it there instead, which takes the space of another binary")]
    private static partial void LogCopying(ILogger logger, string executable, string snapshot, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Could not keep {Executable} under its digest at {Snapshot} ({Reason}); starting it from the shared cache, where a release another client installs over it may be started instead")]
    private static partial void LogUnsnapshotted(ILogger logger, string executable, string snapshot, string reason);

    [LoggerMessage(Level = LogLevel.Information, Message = "Removed {Path}, left by a .NET client that did not finish changing the shared sysml-grpc cache")]
    private static partial void LogRemovedLeftover(ILogger logger, string path);

    [LoggerMessage(Level = LogLevel.Debug, Message = "Could not remove {Path}: {Reason}")]
    private static partial void LogLeftBehind(ILogger logger, string path, string reason);
}

/// <summary>A point in a <see cref="CacheTransaction"/> that tests interpose at.</summary>
internal enum CacheStep
{
    /// <summary>The lock is held and leftovers removed.</summary>
    Locked,

    /// <summary>A release is written to its staged file and verified, not yet published.</summary>
    Staged,

    /// <summary>The executable is replaced and its record not yet written.</summary>
    ExecutablePublished,

    /// <summary>The record is written.</summary>
    MetadataPublished,

    /// <summary>A snapshot is staged and verified, not yet published.</summary>
    SnapshotStaged,
}

/// <summary>Where tests interpose in a <see cref="CacheTransaction"/>: each is null in use.</summary>
internal sealed class CacheSeams
{
    /// <summary>No seams.</summary>
    internal static CacheSeams None { get; } = new();

    /// <summary>Called at each step; what it throws, the transaction throws.</summary>
    internal Func<CacheStep, CancellationToken, Task>? Reached { get; init; }

    /// <summary>Hard links its first argument to its second, in place of <see cref="CacheNative.HardLink"/>.</summary>
    internal Action<string, string>? HardLink { get; init; }
}

/// <summary>What the cache holds.</summary>
/// <param name="Sha256">The executable's digest; null when there is none.</param>
/// <param name="Metadata">The record beside it, when there is a readable one.</param>
/// <param name="MetadataProblem">Why there is none, when there is not.</param>
internal sealed record CacheContents(string? Sha256, CacheMetadata? Metadata, string? MetadataProblem)
{
    /// <summary>Whether the record vouches for the executable: there are both, and the digest recorded is the executable's.</summary>
    internal bool Vouched => Sha256 is not null && Metadata is not null && Metadata.Describes(Sha256);

    /// <summary>Whether there is no record at all, as beside an executable installed by hand, rather than one that cannot be used.</summary>
    internal bool Unrecorded => Metadata is null && MetadataProblem == CacheMetadata.Absent;
}

/// <summary>A release <see cref="CacheTransaction.InstallAsync"/> installed.</summary>
/// <param name="Sha256">Its digest.</param>
/// <param name="Recorded">Whether its record was written; when not, no record vouches for it.</param>
internal sealed record CacheInstall(string Sha256, bool Recorded);

/// <summary>The file <see cref="CacheTransaction.SnapshotAsync"/> returns to start.</summary>
/// <param name="Path">The file.</param>
/// <param name="Sha256">Its digest, taken with the lock held.</param>
/// <param name="Kind">How it came to be.</param>
internal sealed record CacheSnapshot(string Path, string Sha256, SnapshotKind Kind);

/// <summary>How a <see cref="CacheSnapshot"/> came to be.</summary>
internal enum SnapshotKind
{
    /// <summary>It was there already, with its digest.</summary>
    Reused,

    /// <summary>It was hard linked to the executable.</summary>
    Linked,

    /// <summary>It was copied from the executable, the file system not linking.</summary>
    Copied,

    /// <summary>It is the executable itself, which could be neither linked nor copied.</summary>
    Unsnapshotted,
}
