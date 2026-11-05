using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>A <c>sysml-grpc</c> binary to start: its canonical path and the SHA-256 digest of its contents.</summary>
/// <param name="Path">The full path, every link in it followed.</param>
/// <param name="Sha256">The digest of its contents, in lower-case hexadecimal.</param>
internal sealed record ResolvedBinary(string Path, string Sha256);

/// <summary>
/// Finds the <c>sysml-grpc</c> binary a private service is started from: the one
/// <see cref="ConnectionOptions.BinaryPath"/> names, else the one <see cref="ConnectionOptions.BinaryVariable"/>
/// names, else the release asked for (<see cref="ReleaseRequest"/>) in the shared cache every client installs
/// into, <c>~/.opensysml/bin</c>, downloaded and verified there when the cache does not hold it, else, when the
/// release this client was built with cannot be had, the first <c>sysml-grpc</c> on <c>PATH</c>. A binary named
/// explicitly that is not there fails rather than another being used, and is never downloaded. The binary is
/// hashed whichever way it was found, and checked against <see cref="ConnectionOptions.ExpectedBinarySha256"/>
/// before anything executes it.
/// </summary>
/// <remarks>
/// The whole decision about the cache is made with its lock held, and what is started from it is its executable
/// linked under its own digest, which no later install replaces. The cache is used as it is when it is the
/// release asked for by its record, without contradicting a committed pin, or, when no release was asked for, when nothing records what it is, as for a
/// build installed by hand. Otherwise the release is downloaded over it, with a warning, and replaces it only once
/// it has the digest required of it. A release that cannot be had leaves a working cache in place with a warning
/// when no release was asked for, and fails when one was; a download that contradicts a pin, or the digest the
/// caller requires, always fails, however good the cache.
/// </remarks>
internal static partial class BinaryResolver
{
    private const int MaxLinkHops = 40;

    /// <summary>The binary's file name on this platform.</summary>
    internal static string ExecutableName => OperatingSystem.IsWindows() ? "sysml-grpc.exe" : "sysml-grpc";

    /// <summary>The variable holding the home directory on this platform.</summary>
    internal static string HomeVariable => OperatingSystem.IsWindows() ? "USERPROFILE" : "HOME";

    /// <summary>The platform this process runs on when it cannot start processes; null when it can.</summary>
    internal static string? ProcessLessPlatform =>
        OperatingSystem.IsBrowser() ? "browser"
        : OperatingSystem.IsWasi() ? "wasi"
        : OperatingSystem.IsIOS() ? "ios"
        : OperatingSystem.IsTvOS() ? "tvos"
        : null;

    /// <summary>Finds, provides, hashes and checks the binary to start.</summary>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <param name="releases">Where a release is downloaded from.</param>
    /// <param name="cancellationToken">Cancels the hashing, the download, and waiting for the shared cache's lock.</param>
    /// <returns>The binary.</returns>
    /// <exception cref="UnsupportedPlatformException">This platform cannot start a process, or no release is built for it and one was asked for.</exception>
    /// <exception cref="BinaryNotFoundException">No executable binary was found where it was looked for, and none could be downloaded.</exception>
    /// <exception cref="ChecksumMismatchException">The binary, or a release, does not have the digest required of it.</exception>
    /// <exception cref="UnpinnedReleaseException">The release asked for is one nothing this client trusts vouches for.</exception>
    /// <exception cref="InvalidRequestException">A variable names no repository or release.</exception>
    /// <exception cref="ServiceStartException">
    /// The binary could not be read, the shared cache stayed locked, or the release asked for could not be downloaded.
    /// </exception>
    internal static async Task<ResolvedBinary> ResolveAsync(
        ConnectionOptions options,
        Func<string, string?> environment,
        ReleaseSource releases,
        CancellationToken cancellationToken)
    {
        RequireProcesses(ProcessLessPlatform);
        ResolvedBinary resolved;
        if (FindNamed(options, environment) is { } named)
        {
            if (options.DownloadVersion is not null || options.GithubRepo is not null)
            {
                LogDownloadUnused(options.Logger, named);
            }

            resolved = await HashAsync(named, cancellationToken).ConfigureAwait(false);
        }
        else
        {
            resolved = await ProvideAsync(options, environment, releases, cancellationToken).ConfigureAwait(false);
        }

        if (options.ExpectedBinarySha256 is { } expected && !string.Equals(expected, resolved.Sha256, StringComparison.OrdinalIgnoreCase))
        {
            throw new ChecksumMismatchException(
                resolved.Path,
                expected.ToLowerInvariant(),
                resolved.Sha256,
                "name the binary that digest was taken of, or set ConnectionOptions.ExpectedBinarySha256 to this one's after checking where it came from");
        }

        LogResolved(options.Logger, resolved.Path, resolved.Sha256);
        return resolved;
    }

    /// <summary>Refuses to go on where processes cannot be started.</summary>
    /// <param name="platform">The platform, when it cannot start processes; null when it can.</param>
    /// <exception cref="UnsupportedPlatformException"><paramref name="platform"/> is not null.</exception>
    internal static void RequireProcesses(string? platform)
    {
        if (platform is not null)
        {
            throw new UnsupportedPlatformException(
                platform,
                $"A private sysml-grpc cannot be started on {platform}, which cannot start processes: "
                + $"name a running service with ConnectionOptions.Address or ${ConnectionOptions.ServiceVariable}.");
        }
    }

    /// <summary>The binary named explicitly, by <see cref="ConnectionOptions.BinaryPath"/> or <see cref="ConnectionOptions.BinaryVariable"/>.</summary>
    /// <param name="options">The connection's options.</param>
    /// <param name="environment">Reads an environment variable.</param>
    /// <returns>The path of an executable file, or null when none is named.</returns>
    /// <exception cref="BinaryNotFoundException">The one named is not an executable file.</exception>
    internal static string? FindNamed(ConnectionOptions options, Func<string, string?> environment)
    {
        if (options.BinaryPath is { } option)
        {
            return Named(option, "ConnectionOptions.BinaryPath");
        }

        var variable = environment(ConnectionOptions.BinaryVariable);
        return string.IsNullOrWhiteSpace(variable) ? null : Named(variable, "$" + ConnectionOptions.BinaryVariable);
    }

    /// <summary>The first executable <c>sysml-grpc</c> on <c>PATH</c>.</summary>
    /// <param name="environment">Reads an environment variable.</param>
    /// <param name="searched">What <c>PATH</c> held, when it held none.</param>
    /// <returns>The path, or null when there is none.</returns>
    internal static string? FindOnPath(Func<string, string?> environment, out string searched)
    {
        var directories = (environment("PATH") ?? string.Empty).Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        foreach (var directory in directories)
        {
            var candidate = Path.Combine(directory, ExecutableName);
            if (IsExecutable(candidate, out _))
            {
                searched = string.Empty;
                return candidate;
            }
        }

        searched = directories.Length == 0
            ? "$PATH: unset or empty"
            : $"$PATH: no executable {ExecutableName} in its {directories.Length} director{(directories.Length == 1 ? "y" : "ies")}";
        return null;
    }

    private static async Task<ResolvedBinary> ProvideAsync(
        ConnectionOptions options,
        Func<string, string?> environment,
        ReleaseSource releases,
        CancellationToken cancellationToken)
    {
        var request = ReleaseRequest.From(options, environment);
        var searched = new List<string> { $"${ConnectionOptions.BinaryVariable}: unset" };
        ServiceStartException? unavailable = null;
        if (CacheLayout.InHome(environment) is { } cache)
        {
            CacheOutcome outcome;
            try
            {
                outcome = await FromCacheAsync(cache, request, options, releases, cancellationToken).ConfigureAwait(false);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw new ServiceStartException($"sysml-grpc at {cache.Executable} could not be read: {error.Message}", error);
            }

            if (outcome.Binary is { } binary)
            {
                return binary;
            }

            searched.AddRange(outcome.Searched);
            unavailable = outcome.Unavailable;
        }
        else
        {
            if (request.Explicit)
            {
                throw new ServiceStartException(
                    $"sysml-grpc {request.Version} of {request.Repo} was asked for, but ${HomeVariable} is unset, so there is no ~/.opensysml/bin to keep it in.\n"
                    + $"  fix: set ${HomeVariable}, or name a binary with ConnectionOptions.BinaryPath or ${ConnectionOptions.BinaryVariable}.");
            }

            searched.Add($"~/.opensysml/bin/{ExecutableName}: no home directory, since ${HomeVariable} is unset, so no release can be kept there");
        }

        if (FindOnPath(environment, out var onPath) is { } path)
        {
            if (unavailable is not null)
            {
                LogFromPath(options.Logger, path, request.Version, request.Repo, Reason(unavailable));
            }

            return await HashAsync(path, cancellationToken).ConfigureAwait(false);
        }

        searched.Add(onPath);
        throw new BinaryNotFoundException(
            $"No sysml-grpc binary was found to start as a private service. Looked at: {string.Join("; ", searched)}.\n"
            + $"  fix: build one (make build-grpc) and set ConnectionOptions.BinaryPath or ${ConnectionOptions.BinaryVariable} to it, "
            + $"ask for a release this client pins with ConnectionOptions.DownloadVersion or ${ConnectionOptions.VersionVariable}, "
            + $"install one on $PATH, or name a running service with ConnectionOptions.Address or ${ConnectionOptions.ServiceVariable}.",
            searched,
            unavailable);
    }

    // Everything is decided with the lock held, so two opens asking for different releases cannot each find the
    // cache stale and install over each other, and what is linked under its digest is what was decided on.
    private static async Task<CacheOutcome> FromCacheAsync(
        CacheLayout cache,
        ReleaseRequest request,
        ConnectionOptions options,
        ReleaseSource releases,
        CancellationToken cancellationToken)
    {
        var logger = options.Logger;
        CacheTransaction transaction;
        try
        {
            transaction = await CacheTransaction.BeginAsync(cache, logger, cancellationToken).ConfigureAwait(false);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            return await UnlockedAsync(cache, request, releases, logger, error, cancellationToken).ConfigureAwait(false);
        }

        await using (transaction.ConfigureAwait(false))
        {
            BinaryDownloader? downloader = null;
            try
            {
                var contents = await transaction.InspectAsync(cancellationToken).ConfigureAwait(false);
                var usable = IsExecutable(cache.Executable, out var why) && contents.Sha256 is not null;
                if (usable)
                {
                    VerifyCachedPin(cache, contents, releases);
                }

                if (!request.Explicit && usable && options.ExpectedBinarySha256 is { } required && string.Equals(required, contents.Sha256, StringComparison.OrdinalIgnoreCase))
                {
                    return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                }

                var tag = request.IsLatest ? null : request.Version;
                ServiceStartException? unresolved = null;
                if (request.IsLatest)
                {
                    downloader = new BinaryDownloader(releases, logger);
                    try
                    {
                        tag = await downloader.LatestAsync(request.Repo, cancellationToken).ConfigureAwait(false);
                    }
                    catch (ServiceStartException error)
                    {
                        unresolved = error;
                    }
                }

                if (usable)
                {
                    if (tag is not null && Holds(contents, request.Repo, tag))
                    {
                        return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                    }

                    if (!request.Explicit && contents.Unrecorded)
                    {
                        return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                    }

                    if (tag is null)
                    {
                        // Unreachable releases are no reason to discard a release of the repository asked for.
                        if (!contents.Vouched || contents.Metadata!.Repo != request.Repo)
                        {
                            throw unresolved!;
                        }

                        LogLatestUnresolved(logger, contents.Metadata.Version, request.Repo, cache.Executable, Reason(unresolved!));
                        return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                    }

                    LogReplacing(logger, StaleReason(cache, contents, request.Repo, tag), tag, request.Repo);
                }
                else if (tag is null)
                {
                    throw unresolved!;
                }

                var failure = await InstallAsync(transaction, downloader ??= new BinaryDownloader(releases, logger), request, tag, options, cancellationToken).ConfigureAwait(false);
                if (failure is null)
                {
                    return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                }

                if (request.Explicit)
                {
                    throw failure;
                }

                if (usable)
                {
                    LogKeeping(logger, cache.Executable, tag, request.Repo, Reason(failure));
                    return await StartAsync(transaction, logger, cancellationToken).ConfigureAwait(false);
                }

                return new CacheOutcome(null, [$"{cache.Executable}: {(contents.Sha256 is null ? "absent" : why)}", $"release {tag} of {request.Repo}: {Reason(failure)}"], failure);
            }
            finally
            {
                downloader?.Dispose();
            }
        }
    }

    // Integrity failures are thrown; anything else that leaves the release unavailable is returned.
    private static async Task<ServiceStartException?> InstallAsync(
        CacheTransaction transaction,
        BinaryDownloader downloader,
        ReleaseRequest request,
        string tag,
        ConnectionOptions options,
        CancellationToken cancellationToken)
    {
        try
        {
            var asset = await downloader.VouchAsync(request, tag, options.ExpectedBinarySha256, cancellationToken).ConfigureAwait(false);
            await downloader.InstallAsync(transaction, asset, cancellationToken).ConfigureAwait(false);
            return null;
        }
        catch (ChecksumMismatchException)
        {
            throw;
        }
        catch (ServiceStartException error)
        {
            return error;
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            return new ServiceStartException($"sysml-grpc {tag} of {request.Repo} could not be installed in {transaction.Layout.Directory}: {error.Message}", error);
        }
    }

    // A cache that cannot be locked, being where this user may not write, is used as the other clients use it:
    // unlocked, with a warning, as itself; but nothing can be installed there, so a release asked for fails.
    private static async Task<CacheOutcome> UnlockedAsync(CacheLayout cache, ReleaseRequest request, ReleaseSource releases, ILogger logger, Exception error, CancellationToken cancellationToken)
    {
        if (request.Explicit)
        {
            throw new ServiceStartException(
                $"sysml-grpc {request.Version} of {request.Repo} was asked for, but {cache.Lock} could not be locked ({error.Message}), "
                + "so the cache can neither be read as that release nor have it installed.\n"
                + $"  fix: let this user write to {cache.Directory}, or name a binary with ConnectionOptions.BinaryPath or ${ConnectionOptions.BinaryVariable}.",
                error);
        }

        if (IsExecutable(cache.Executable, out var why))
        {
            var digest = await Sha256Async(cache.Executable, cancellationToken).ConfigureAwait(false);
            var record = CacheMetadata.Read(cache.Metadata);
            VerifyCachedPin(cache, new CacheContents(digest, record.Metadata, record.Problem), releases);
            LogUnlocked(logger, cache.Lock, cache.Executable, error.Message);
            return new CacheOutcome(new ResolvedBinary(Canonical(cache.Executable), digest), [], null);
        }

        var unavailable = new ServiceStartException($"{cache.Lock} could not be locked ({error.Message}), so no release can be installed in {cache.Directory}.", error);
        return new CacheOutcome(null, [$"{cache.Executable}: {why}", $"release {request.Version} of {request.Repo}: {Reason(unavailable)}"], unavailable);
    }

    private static void VerifyCachedPin(CacheLayout cache, CacheContents contents, ReleaseSource releases)
    {
        if (!contents.Vouched || contents.Metadata!.Repo is not { } repo
            || (releases.Asset is null && !ReleasePlatform.Published.Contains($"{ReleasePlatform.HostGoos}-{ReleasePlatform.HostGoarch}")))
        {
            return;
        }

        var asset = releases.Asset ?? ReleasePlatform.HostAsset();
        var pinned = (releases.Pins ?? ReleaseDigests.Embedded).Pinned(repo, contents.Metadata.Version, asset);
        if (pinned is not null && !string.Equals(pinned, contents.Sha256, StringComparison.OrdinalIgnoreCase))
        {
            throw new ChecksumMismatchException(
                cache.Executable,
                pinned,
                contents.Sha256!,
                $"the cached {asset} of {contents.Metadata.Version} of {repo} contradicts this client's committed pin; "
                + "replace it from a source you trust, or name a verified build with ConnectionOptions.BinaryPath");
        }
    }

    private static async Task<CacheOutcome> StartAsync(CacheTransaction transaction, ILogger logger, CancellationToken cancellationToken)
    {
        var snapshot = await transaction.SnapshotAsync(cancellationToken).ConfigureAwait(false)
            ?? throw new ServiceStartException($"{transaction.Layout.Executable} was removed while the shared cache was locked.");
        var path = Canonical(snapshot.Path);
        LogFromCache(logger, transaction.Layout.Executable, path, snapshot.Kind);
        return new CacheOutcome(new ResolvedBinary(path, snapshot.Sha256), [], null);
    }

    private static bool Holds(CacheContents contents, string repo, string tag) =>
        contents.Vouched && contents.Metadata!.Version == tag && contents.Metadata.Repo == repo;

    private static string StaleReason(CacheLayout cache, CacheContents contents, string repo, string tag)
    {
        var cached = $"the binary cached at {cache.Executable}";
        return contents.Metadata switch
        {
            null => $"{cached} has no usable record ({contents.MetadataProblem}), so which release it is cannot be told, and {tag} of {repo} was asked for",
            { } recorded when !contents.Vouched => $"{cached} is not the {recorded.Version} its record names, whose digest is another, and {tag} of {repo} was asked for",
            { Repo: null } recorded => $"{cached} is {recorded.Version} of a repository its record does not name, and {tag} of {repo} was asked for",
            { } recorded when recorded.Repo != repo => $"{cached} was downloaded from {recorded.Repo}, but {tag} of {repo} was asked for",
            { } recorded => $"{cached} is {recorded.Version}, but {tag} was asked for",
        };
    }

    private static string Reason(Exception error)
    {
        var message = error.Message;
        var end = message.IndexOf('\n', StringComparison.Ordinal);
        return (end < 0 ? message : message[..end]).TrimEnd('.');
    }

    private static async Task<ResolvedBinary> HashAsync(string path, CancellationToken cancellationToken)
    {
        try
        {
            var canonical = Canonical(path);
            return new ResolvedBinary(canonical, await Sha256Async(canonical, cancellationToken).ConfigureAwait(false));
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            throw new ServiceStartException($"sysml-grpc at {path} could not be read: {error.Message}", error);
        }
    }

    /// <summary>
    /// The full path of a file with every link in it followed, as the kernel would follow them, so
    /// that one file reached by different paths is one binary.
    /// </summary>
    /// <param name="path">The path.</param>
    /// <returns>The canonical path.</returns>
    /// <exception cref="IOException">The links do not end within the kernel's own limit.</exception>
    /// <exception cref="DirectoryNotFoundException">A parent component is not an accessible directory.</exception>
    internal static string Canonical(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        var current = Absolute(path, Environment.CurrentDirectory);
        for (var hops = 0; ; hops++)
        {
            var resolved = FollowFirstLink(current, out var canonical);
            if (resolved is null)
            {
                return canonical;
            }

            if (hops == MaxLinkHops)
            {
                throw new IOException($"{path} has more than {MaxLinkHops} links to follow.");
            }

            current = resolved;
        }
    }

    /// <summary>Whether a path is a file this process could execute, and why not when it is not.</summary>
    /// <param name="path">The path.</param>
    /// <param name="why">What it is instead, when it is not.</param>
    /// <returns>Whether it is.</returns>
    internal static bool IsExecutable(string path, out string why)
    {
        try
        {
            // File.Exists is true of a link to nothing, so what the links lead to is what is examined.
            string resolved;
            try
            {
                resolved = Canonical(path);
            }
            catch (DirectoryNotFoundException)
            {
                why = "absent";
                return false;
            }
            catch (IOException)
            {
                why = "a loop of links";
                return false;
            }

            if (Directory.Exists(resolved))
            {
                why = "a directory";
                return false;
            }

            if (!File.Exists(resolved))
            {
                why = new FileInfo(path).LinkTarget is null ? "absent" : "a link to nothing";
                return false;
            }

            if (!OperatingSystem.IsWindows()
                && (File.GetUnixFileMode(resolved) & (UnixFileMode.UserExecute | UnixFileMode.GroupExecute | UnixFileMode.OtherExecute)) == 0)
            {
                why = "not executable";
                return false;
            }
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException or ArgumentException or NotSupportedException)
        {
            why = $"unreadable ({error.Message})";
            return false;
        }

        why = string.Empty;
        return true;
    }

    private static string Named(string path, string source)
    {
        if (IsExecutable(path, out var why))
        {
            return path;
        }

        throw new BinaryNotFoundException(
            $"{source} names {path}, which is {why}, not an executable sysml-grpc; it is used as named or not at all.\n"
            + $"  fix: point {source} at a sysml-grpc build (make build-grpc writes bin/{ExecutableName}), or leave it unset to look for one.",
            [$"{source}: {path}: {why}"]);
    }

    private static string? FollowFirstLink(string full, out string canonical)
    {
        var root = Path.GetPathRoot(full) ?? string.Empty;
        var parts = full[root.Length..].Split([Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar], StringSplitOptions.RemoveEmptyEntries);
        canonical = root;
        for (var i = 0; i < parts.Length; i++)
        {
            if (!Directory.Exists(canonical))
            {
                throw new DirectoryNotFoundException($"{canonical} is not an accessible directory in {full}.");
            }

            if (parts[i] == ".")
            {
                continue;
            }

            if (parts[i] == "..")
            {
                canonical = Path.GetDirectoryName(Path.TrimEndingDirectorySeparator(canonical)) ?? root;
                continue;
            }

            var parent = canonical;
            canonical = Path.Combine(canonical, parts[i]);
            if (new FileInfo(canonical).LinkTarget is { } target)
            {
                return Path.Combine([Absolute(target, parent), .. parts[(i + 1)..]]);
            }
        }

        if (Path.EndsInDirectorySeparator(full) && !Directory.Exists(canonical))
        {
            throw new DirectoryNotFoundException($"{canonical} is not an accessible directory in {full}.");
        }

        return null;
    }

    // Keep parent components until their preceding links have been followed.
    private static string Absolute(string path, string parent) =>
        Path.IsPathFullyQualified(path) ? path
        : Path.IsPathRooted(path) ? Path.GetFullPath(path, parent)
        : Path.Combine(parent, path);

    /// <summary>The SHA-256 digest of a file's contents, in lower-case hexadecimal.</summary>
    /// <param name="path">The file.</param>
    /// <param name="cancellationToken">Cancels the reading.</param>
    /// <returns>The digest.</returns>
    internal static async Task<string> Sha256Async(string path, CancellationToken cancellationToken)
    {
        var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read | FileShare.Delete, 1 << 16, FileOptions.Asynchronous | FileOptions.SequentialScan);
        await using (file.ConfigureAwait(false))
        {
            return Convert.ToHexStringLower(await SHA256.HashDataAsync(file, cancellationToken).ConfigureAwait(false));
        }
    }

    [LoggerMessage(Level = LogLevel.Debug, Message = "Resolved the private service's binary to {Path}, SHA-256 {Sha256}")]
    private static partial void LogResolved(ILogger logger, string path, string sha256);

    [LoggerMessage(Level = LogLevel.Debug, Message = "Starting the shared cache's {Executable} as {Snapshot} ({Kind})")]
    private static partial void LogFromCache(ILogger logger, string executable, string snapshot, SnapshotKind kind);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Could not lock {Lock} ({Reason}); using {Executable} unlocked, so a release another client installs over it may be started instead")]
    private static partial void LogUnlocked(ILogger logger, string @lock, string executable, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Replacing the cached sysml-grpc: {Reason}. Downloading {Tag} of {Repo}")]
    private static partial void LogReplacing(ILogger logger, string reason, string tag, string repo);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Keeping the cached sysml-grpc at {Path}: {Tag} of {Repo} could not be had ({Reason}). It may be an older release than the one this client was built with")]
    private static partial void LogKeeping(ILogger logger, string path, string tag, string repo, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Keeping the cached sysml-grpc {Version} of {Repo} at {Path}: the latest release could not be resolved ({Reason}), so it may not be the latest")]
    private static partial void LogLatestUnresolved(ILogger logger, string version, string repo, string path, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Starting {Path} from $PATH: sysml-grpc {Tag} of {Repo} could not be had ({Reason})")]
    private static partial void LogFromPath(ILogger logger, string path, string tag, string repo, string reason);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Starting {Path}, which $" + ConnectionOptions.BinaryVariable + " names: ConnectionOptions.DownloadVersion and GithubRepo describe a release to download, and are not used")]
    private static partial void LogDownloadUnused(ILogger logger, string path);

    /// <summary>What the shared cache provided: a binary to start, or what was found there and why nothing could be had.</summary>
    /// <param name="Binary">The binary, when there is one.</param>
    /// <param name="Searched">What was found there, when there is none.</param>
    /// <param name="Unavailable">Why no release could be had, when there is none.</param>
    private sealed record CacheOutcome(ResolvedBinary? Binary, IReadOnlyList<string> Searched, ServiceStartException? Unavailable);
}
