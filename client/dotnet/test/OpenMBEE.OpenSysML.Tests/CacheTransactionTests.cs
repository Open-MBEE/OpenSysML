using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A use of the shared cache with its lock held: what it holds is read, a verified release is installed and
/// recorded, and the executable is linked under its digest, with every interruption, in this process or by a
/// crash of another, leaving either the cache as it was or an executable no record vouches for.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class CacheTransactionTests : IDisposable
{
    private static readonly byte[] First = Encoding.ASCII.GetBytes("#!/bin/sh\necho first release\n");
    private static readonly byte[] Second = Encoding.ASCII.GetBytes("#!/bin/sh\necho second release, longer\n");

    private readonly TestCache cache = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private CacheLayout Layout => cache.Layout;

    [Fact]
    public async Task AReleaseIsVerifiedPublishedAndRecorded()
    {
        var logger = new ListLogger();

        var installed = await cache.InstallAsync(First, "v1.2.3", "Open-MBEE/OpenSysML", logger);

        Assert.Equal(new CacheInstall(TestCache.Sha256(First), Recorded: true), installed);
        Assert.Equal(First, File.ReadAllBytes(Layout.Executable));
        Assert.Equal(new CacheMetadata("v1.2.3", TestCache.Sha256(First), "Open-MBEE/OpenSysML").Format(), File.ReadAllBytes(Layout.Metadata));
        if (!OperatingSystem.IsWindows())
        {
            Assert.Equal(UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute, File.GetUnixFileMode(Layout.Executable));
        }

        Assert.Equal(cache.SettledFiles(), cache.Files());
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Information
            && entry.Message == $"Installed sysml-grpc v1.2.3 of Open-MBEE/OpenSysML at {Layout.Executable}, SHA-256 {TestCache.Sha256(First)}");
        var contents = await Inspect();
        Assert.True(contents.Vouched);
        Assert.Equal(new CacheContents(TestCache.Sha256(First), new CacheMetadata("v1.2.3", TestCache.Sha256(First), "Open-MBEE/OpenSysML"), null), contents);
    }

    [Fact]
    public async Task AnotherReleaseReplacesTheExecutableAndItsRecord()
    {
        await cache.InstallAsync(First, "v1.0.0");
        await cache.InstallAsync(Second, "v2.0.0", "fork/OpenSysML");

        Assert.Equal(Second, File.ReadAllBytes(Layout.Executable));
        Assert.Equal(new CacheMetadata("v2.0.0", TestCache.Sha256(Second), "fork/OpenSysML"), (await Inspect()).Metadata);
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task BytesWithoutTheDigestRequiredAreNeverPublished()
    {
        await cache.InstallAsync(First);
        var before = State();

        await using var transaction = await Begin();
        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            transaction.InstallAsync(TestCache.Writing(Second), TestCache.Sha256(First).ToUpperInvariant(), "v2.0.0", "Open-MBEE/OpenSysML", Token));

        Assert.Equal((Layout.Executable, TestCache.Sha256(First), TestCache.Sha256(Second)), (error.BinaryPath, error.ExpectedSha256, error.ActualSha256));
        Assert.Contains("the bytes received for v2.0.0 were discarded and the cache was left as it was", error.Message, StringComparison.Ordinal);
        Assert.Equal(before, State());
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task AWriteThatFailsOrIsCancelledLeavesTheCacheAsItWas()
    {
        await cache.InstallAsync(First);
        var before = State();
        await using var transaction = await Begin();

        await Assert.ThrowsAsync<IOException>(() => transaction.InstallAsync(
            async (stream, token) =>
            {
                await stream.WriteAsync(Second.AsMemory(0, 10), token);
                throw new IOException("the connection dropped");
            },
            TestCache.Sha256(Second),
            "v2.0.0",
            "Open-MBEE/OpenSysML",
            Token));
        Assert.Equal(before, State());
        Assert.Equal(cache.SettledFiles(), cache.Files());

        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => transaction.InstallAsync(
            async (stream, token) =>
            {
                await stream.WriteAsync(Second.AsMemory(0, 10), token);
                await cancel.CancelAsync();
                await stream.WriteAsync(Second.AsMemory(10), token);
            },
            TestCache.Sha256(Second),
            "v2.0.0",
            "Open-MBEE/OpenSysML",
            cancel.Token));
        Assert.Equal(before, State());
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Theory]
    [InlineData("abc", "v1", "o/r")]
    [InlineData("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg", "v1", "o/r")]
    [InlineData("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "", "o/r")]
    [InlineData("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "v1", "")]
    public async Task WhatCannotBeRecordedIsRefusedBeforeAnythingIsWritten(string sha256, string version, string repo)
    {
        await using var transaction = await Begin();
        var written = false;

        await Assert.ThrowsAnyAsync<ArgumentException>(() => transaction.InstallAsync((_, _) => Task.FromResult(written = true), sha256, version, repo, Token));

        Assert.False(written);
        Assert.Equal([Path.GetFileName(Layout.Lock), Path.GetFileName(Layout.Guard)], cache.Files());
    }

    [Fact]
    public async Task AnInterruptionBetweenTheExecutableAndItsRecordLeavesAnExecutableNoRecordVouchesFor()
    {
        await cache.InstallAsync(First, "v1.0.0");
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.ExecutablePublished ? throw new InvalidOperationException("interrupted") : Task.CompletedTask };

        await using (var transaction = await Begin(seams))
        {
            var error = await Assert.ThrowsAsync<InvalidOperationException>(() =>
                transaction.InstallAsync(TestCache.Writing(Second), TestCache.Sha256(Second), "v2.0.0", "Open-MBEE/OpenSysML", Token));
            Assert.Equal("interrupted", error.Message);
        }

        var torn = await Inspect();
        Assert.Equal(TestCache.Sha256(Second), torn.Sha256);
        Assert.Equal(new CacheMetadata("v1.0.0", TestCache.Sha256(First), "Open-MBEE/OpenSysML"), torn.Metadata);
        Assert.False(torn.Vouched, "a record of the first release vouched for the second's executable");
        Assert.Equal(cache.SettledFiles(), cache.Files());

        await cache.InstallAsync(Second, "v2.0.0");
        Assert.True((await Inspect()).Vouched);
    }

    [Fact]
    public async Task AnInterruptionBeforeAnyRecordLeavesAnExecutableWithoutOne()
    {
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.ExecutablePublished ? throw new InvalidOperationException("interrupted") : Task.CompletedTask };

        await using (var transaction = await Begin(seams))
        {
            await Assert.ThrowsAsync<InvalidOperationException>(() =>
                transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1.0.0", "Open-MBEE/OpenSysML", Token));
        }

        Assert.Equal(new CacheContents(TestCache.Sha256(First), null, "absent"), await Inspect());
    }

    [Fact]
    public async Task AReleaseCancelledAfterVerificationIsNotPublished()
    {
        await cache.InstallAsync(First);
        var before = State();
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.Staged ? cancel.CancelAsync() : Task.CompletedTask };
        await using var transaction = await Begin(seams);

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            transaction.InstallAsync(TestCache.Writing(Second), TestCache.Sha256(Second), "v2.0.0", "Open-MBEE/OpenSysML", cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.Equal(before, State());
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task AFirstReleaseCancelledAfterVerificationLeavesNoExecutableOrRecord()
    {
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.Staged ? cancel.CancelAsync() : Task.CompletedTask };
        await using var transaction = await Begin(seams);

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1.0.0", "Open-MBEE/OpenSysML", cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.Equal([Path.GetFileName(Layout.Lock), Path.GetFileName(Layout.Guard)], cache.Files());
    }

    [Fact]
    public async Task OncePublishedTheExecutableIsRecordedEvenIfTheInstallIsCancelled()
    {
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.ExecutablePublished ? cancel.CancelAsync() : Task.CompletedTask };
        await using var transaction = await Begin(seams);

        var installed = await transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1.0.0", "Open-MBEE/OpenSysML", cancel.Token);

        Assert.True(installed.Recorded);
        Assert.True(cancel.IsCancellationRequested);
        Assert.True((await transaction.InspectAsync(Token)).Vouched);
    }

    [Fact]
    public async Task ARecordThatCannotBeWrittenLeavesTheReleaseUnvouchedWithAWarning()
    {
        Directory.CreateDirectory(Layout.Metadata);
        var logger = new ListLogger();

        var installed = await cache.InstallAsync(First, logger: logger);

        Assert.Equal(new CacheInstall(TestCache.Sha256(First), Recorded: false), installed);
        Assert.Equal(First, File.ReadAllBytes(Layout.Executable));
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.StartsWith($"Installed {Layout.Executable}, but could not record which release it is in {Layout.Metadata}: ", warning.Message, StringComparison.Ordinal);
        Assert.Equal(new CacheContents(TestCache.Sha256(First), null, "a directory"), await Inspect());
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task WhatAnInterruptedTransactionStagedIsRemovedByTheNextAndNothingElse()
    {
        await cache.InstallAsync(First);
        string[] ours =
        [
            Path.GetFileName(CacheLayout.Temporary(Layout.Executable)),
            Path.GetFileName(CacheLayout.Temporary(Layout.Metadata)),
            Path.GetFileName(CacheLayout.Temporary(Layout.Snapshot(TestCache.Sha256(First)))),
        ];
        string[] theirs = ["sysml-grpc.tmp", "sysml-grpc.4815162342.tmp", "sysml-grpc.4815162342.json", "sysml-grpc.4815162342.link", "notes.dotnet-0123456789abcdef.tmp"];
        foreach (var name in ours.Concat(theirs))
        {
            File.WriteAllText(Path.Combine(Layout.Directory, name), "left behind");
        }

        var logger = new ListLogger();
        await using (await CacheTransaction.BeginAsync(Layout, logger, Token))
        {
        }

        Assert.Equal([.. cache.SettledFiles().Concat(theirs).Order(StringComparer.Ordinal)], cache.Files());
        Assert.Equal(
            ours.Select(name => $"Removed {Path.Combine(Layout.Directory, name)}, left by a .NET client that did not finish changing the shared sysml-grpc cache").Order(StringComparer.Ordinal),
            logger.Entries.Where(entry => entry.Level == LogLevel.Information).Select(entry => entry.Message).Order(StringComparer.Ordinal));
    }

    [Fact]
    public async Task TheCrossClientLockIsHeldAtEveryStep()
    {
        var checkedSteps = new List<CacheStep>();
        var seams = new CacheSeams
        {
            Reached = async (step, _) =>
            {
                Assert.False(await cache.IsSharedLockFreeAsync(), $"another client could take the cache's lock at {step}");
                checkedSteps.Add(step);
            },
        };

        await using (var transaction = await Begin(seams))
        {
            await transaction.InspectAsync(Token);
            await transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1.0.0", "Open-MBEE/OpenSysML", Token);
            await transaction.SnapshotAsync(Token);
            Assert.False(await cache.IsSharedLockFreeAsync(), "another client could take the cache's lock at the end of the transaction");
        }

        Assert.Equal(Enum.GetValues<CacheStep>(), checkedSteps);
        Assert.True(await cache.IsSharedLockFreeAsync());
    }

    [Fact]
    public async Task AnEndedTransactionIsRefusedAndHasReleasedTheLock()
    {
        var transaction = await Begin();
        await transaction.DisposeAsync();
        await transaction.DisposeAsync();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => transaction.InspectAsync(Token));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => transaction.SnapshotAsync(Token));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1", "o/r", Token));
        Assert.True(await cache.IsFreeToAnotherProcessAsync());
    }

    [Fact]
    public async Task ACacheWithoutAnExecutableHasNoSnapshot()
    {
        await using var transaction = await Begin();

        Assert.Null(await transaction.SnapshotAsync(Token));
        Assert.Equal(new CacheContents(null, null, "absent"), await transaction.InspectAsync(Token));
    }

    [Fact]
    public async Task TheExecutableIsHardLinkedUnderItsDigestAndThenReused()
    {
        await cache.InstallAsync(First);
        var logger = new ListLogger();
        var digest = TestCache.Sha256(First);

        var linked = await SnapshotAsync(logger: logger);
        var reused = await SnapshotAsync(logger: logger);

        Assert.Equal(new CacheSnapshot(Layout.Snapshot(digest), digest, SnapshotKind.Linked), linked);
        Assert.Equal(new CacheSnapshot(Layout.Snapshot(digest), digest, SnapshotKind.Reused), reused);
        Assert.Equal(First, File.ReadAllBytes(linked!.Path));
        Assert.True(BinaryResolver.IsExecutable(linked.Path, out _));
        Assert.DoesNotContain(logger.Entries, entry => entry.Level >= LogLevel.Warning);
        Assert.Equal(cache.SettledFiles(Layout.Snapshot(digest)), cache.Files());

        // One file under two names: what changes the executable in place changes the snapshot.
        using (var executable = new FileStream(Layout.Executable, FileMode.Append))
        {
            executable.WriteByte((byte)'#');
        }

        Assert.Equal(First.Length + 1, new FileInfo(linked.Path).Length);
    }

    [Fact]
    public async Task ASymbolicLinkSnapshotIsReplacedEvenWhenItsDigestMatches()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var target = Layout.Snapshot(digest);
        File.CreateSymbolicLink(target, Path.GetFileName(Layout.Executable));
        var logger = new ListLogger();

        var snapshot = await SnapshotAsync(logger: logger);

        Assert.Equal(new CacheSnapshot(target, digest, SnapshotKind.Linked), snapshot);
        Assert.Null(new FileInfo(target).LinkTarget);
        Assert.Equal($"Replacing {target}, which is a symbolic link", Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning).Message);
        await cache.InstallAsync(Second);
        Assert.Equal(First, File.ReadAllBytes(target));
        Assert.Equal(digest, await BinaryResolver.Sha256Async(target, Token));
    }

    [Fact]
    public async Task ADanglingSymbolicLinkSnapshotIsReplacedWithAWarning()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var target = Layout.Snapshot(digest);
        File.CreateSymbolicLink(target, "missing");
        var logger = new ListLogger();

        var snapshot = await SnapshotAsync(logger: logger);

        Assert.Equal(new CacheSnapshot(target, digest, SnapshotKind.Linked), snapshot);
        Assert.Null(new FileInfo(target).LinkTarget);
        Assert.Equal(First, File.ReadAllBytes(target));
        Assert.Equal($"Replacing {target}, which is a symbolic link", Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning).Message);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task ASymbolicLinkExecutableIsSnapshottedAsItsFileNotItsLink(bool copy)
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        await cache.InstallAsync(First);
        var source = Path.Combine(cache.Root, "hand-installed");
        File.Move(Layout.Executable, source);
        File.CreateSymbolicLink(Layout.Executable, Path.GetRelativePath(Layout.Directory, source));
        var seams = copy ? new CacheSeams { HardLink = (_, _) => throw new IOException("this file system has no hard links") } : null;

        var snapshot = await SnapshotAsync(seams);

        Assert.Equal(new CacheSnapshot(Layout.Snapshot(TestCache.Sha256(First)), TestCache.Sha256(First), copy ? SnapshotKind.Copied : SnapshotKind.Linked), snapshot);
        Assert.Null(new FileInfo(snapshot!.Path).LinkTarget);
        var replacement = Path.Combine(cache.Root, "replacement");
        File.WriteAllBytes(replacement, Second);
        File.Move(replacement, source, overwrite: true);
        Assert.Equal(Second, File.ReadAllBytes(Layout.Executable));
        Assert.Equal(First, File.ReadAllBytes(snapshot.Path));
        Assert.Equal(snapshot.Sha256, await BinaryResolver.Sha256Async(snapshot.Path, Token));
    }

    [Fact]
    public async Task ASnapshotWithoutItsDigestIsReplaced()
    {
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        File.WriteAllBytes(Layout.Snapshot(digest), Second);
        var logger = new ListLogger();

        var snapshot = await SnapshotAsync(logger: logger);

        Assert.Equal(new CacheSnapshot(Layout.Snapshot(digest), digest, SnapshotKind.Linked), snapshot);
        Assert.Equal(First, File.ReadAllBytes(snapshot!.Path));
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.Equal($"Replacing {Layout.Snapshot(digest)}, which has SHA-256 {TestCache.Sha256(Second)}", warning.Message);
        Assert.Equal(cache.SettledFiles(Layout.Snapshot(digest)), cache.Files());
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task ASnapshotThatLostItsExecuteBitIsGivenItBackAndReused()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Windows has no execute bit.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        await SnapshotAsync();
        File.SetUnixFileMode(Layout.Snapshot(digest), UnixFileMode.UserRead | UnixFileMode.UserWrite);

        var snapshot = await SnapshotAsync();

        Assert.Equal(SnapshotKind.Reused, snapshot!.Kind);
        Assert.True((File.GetUnixFileMode(snapshot.Path) & UnixFileMode.UserExecute) != 0);
    }

    [Fact]
    public async Task WhereTheFileSystemCannotLinkTheExecutableIsCopiedWithAWarning()
    {
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var logger = new ListLogger();
        var seams = new CacheSeams { HardLink = (_, _) => throw new IOException("this file system has no hard links") };

        var copied = await SnapshotAsync(seams, logger);

        Assert.Equal(new CacheSnapshot(Layout.Snapshot(digest), digest, SnapshotKind.Copied), copied);
        Assert.Equal(First, File.ReadAllBytes(copied!.Path));
        Assert.True(BinaryResolver.IsExecutable(copied.Path, out _));
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.Equal(
            $"Could not hard link {Layout.Executable} to {Layout.Snapshot(digest)} (this file system has no hard links); copying it there instead, which takes the space of another binary",
            warning.Message);
        Assert.Equal(cache.SettledFiles(Layout.Snapshot(digest)), cache.Files());

        // A copy of its own: what changes the executable in place leaves it alone.
        using (var executable = new FileStream(Layout.Executable, FileMode.Append))
        {
            executable.WriteByte((byte)'#');
        }

        Assert.Equal(First, File.ReadAllBytes(copied.Path));
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task WhereTheExecutableCanBeNeitherLinkedNorCopiedItIsStartedItselfWithAWarning()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user writes where file permissions forbid it, or this platform has no such permissions.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var logger = new ListLogger();
        await using var transaction = await CacheTransaction.BeginAsync(Layout, logger, Token);
        File.SetUnixFileMode(Layout.Directory, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        try
        {
            var snapshot = await transaction.SnapshotAsync(Token);

            Assert.Equal(new CacheSnapshot(Layout.Executable, digest, SnapshotKind.Unsnapshotted), snapshot);
            var warnings = logger.Entries.Where(entry => entry.Level == LogLevel.Warning).Select(entry => entry.Message).ToList();
            Assert.Equal(2, warnings.Count);
            Assert.StartsWith($"Could not hard link {Layout.Executable} to {Layout.Snapshot(digest)} (", warnings[0], StringComparison.Ordinal);
            Assert.StartsWith($"Could not keep {Layout.Executable} under its digest at {Layout.Snapshot(digest)} (", warnings[1], StringComparison.Ordinal);
            Assert.EndsWith("); starting it from the shared cache, where a release another client installs over it may be started instead", warnings[1], StringComparison.Ordinal);
        }
        finally
        {
            File.SetUnixFileMode(Layout.Directory, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }

        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task AFailureAsATransactionBeginsReleasesTheLock()
    {
        var seams = new CacheSeams { Reached = (step, _) => step == CacheStep.Locked ? throw new InvalidOperationException("refused") : Task.CompletedTask };

        await Assert.ThrowsAsync<InvalidOperationException>(() => Begin(seams));

        Assert.True(await cache.IsFreeToAnotherProcessAsync());
        await using (await Begin())
        {
        }
    }

    [Fact]
    public async Task AReleaseThatCannotReplaceTheExecutableFailsAndLeavesNothingStaged()
    {
        Directory.CreateDirectory(Path.Combine(Layout.Executable, "occupied"));
        await using var transaction = await Begin();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() =>
            transaction.InstallAsync(TestCache.Writing(First), TestCache.Sha256(First), "v1.0.0", "Open-MBEE/OpenSysML", Token));

        Assert.StartsWith($"sysml-grpc v1.0.0, verified, could not replace {Layout.Executable}: ", error.Message, StringComparison.Ordinal);
        Assert.True(error.InnerException is IOException or UnauthorizedAccessException, $"the cause is {error.InnerException}");
        Assert.Equal([Path.GetFileName(Layout.Executable), Path.GetFileName(Layout.Lock), Path.GetFileName(Layout.Guard)], cache.Files());
    }

    [Fact]
    public async Task WhatWasLinkedWithoutTheExecutablesDigestIsNotTakenForItsSnapshot()
    {
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var logger = new ListLogger();
        var seams = new CacheSeams { HardLink = (_, link) => File.WriteAllBytes(link, Second) };

        var snapshot = await SnapshotAsync(seams, logger);

        Assert.Equal(new CacheSnapshot(Layout.Executable, digest, SnapshotKind.Unsnapshotted), snapshot);
        Assert.Equal(
            $"Could not keep {Layout.Executable} under its digest at {Layout.Snapshot(digest)} (what was linked or copied does not have the executable's digest); starting it from the shared cache, where a release another client installs over it may be started instead",
            Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning).Message);
        Assert.Equal(cache.SettledFiles(), cache.Files());
    }

    [Fact]
    public async Task ASnapshotThatCannotBePublishedLeavesTheExecutableItselfToStart()
    {
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        Directory.CreateDirectory(Path.Combine(Layout.Snapshot(digest), "occupied"));
        var logger = new ListLogger();

        var snapshot = await SnapshotAsync(logger: logger);

        Assert.Equal(new CacheSnapshot(Layout.Executable, digest, SnapshotKind.Unsnapshotted), snapshot);
        Assert.StartsWith($"Could not keep {Layout.Executable} under its digest at {Layout.Snapshot(digest)} (", Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning).Message, StringComparison.Ordinal);
        Assert.Equal(cache.SettledFiles(Layout.Snapshot(digest)), cache.Files());
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task ARecordOrSnapshotThatCannotBeReadIsNone()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user reads what file permissions forbid, or this platform has no such permissions.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        File.WriteAllBytes(Layout.Snapshot(digest), First);
        File.SetUnixFileMode(Layout.Metadata, UnixFileMode.None);
        File.SetUnixFileMode(Layout.Snapshot(digest), UnixFileMode.UserWrite);
        var logger = new ListLogger();

        var contents = await Inspect();
        var snapshot = await SnapshotAsync(logger: logger);

        Assert.Null(contents.Metadata);
        Assert.StartsWith("unreadable (", contents.MetadataProblem, StringComparison.Ordinal);
        Assert.Equal(new CacheSnapshot(Layout.Snapshot(digest), digest, SnapshotKind.Linked), snapshot);
        Assert.Equal($"Replacing {Layout.Snapshot(digest)}, which could not be read", Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning).Message);
        File.SetUnixFileMode(Layout.Metadata, UnixFileMode.UserRead | UnixFileMode.UserWrite);
    }

    [Fact]
    public async Task ASnapshotKeepsTheBytesThatWereVerifiedWhenAnotherReleaseIsInstalled()
    {
        await cache.InstallAsync(First, "v1.0.0");
        var first = await SnapshotAsync();
        await cache.InstallAsync(Second, "v2.0.0");
        var second = await SnapshotAsync();

        Assert.Equal(First, File.ReadAllBytes(first!.Path));
        Assert.Equal(Second, File.ReadAllBytes(second!.Path));
        Assert.NotEqual(first.Path, second.Path);
        Assert.Equal(cache.SettledFiles(first.Path, second.Path), cache.Files());
    }

    [Theory]
    [InlineData(nameof(CacheStep.Locked))]
    [InlineData(nameof(CacheStep.Staged))]
    [InlineData(nameof(CacheStep.ExecutablePublished))]
    [InlineData(nameof(CacheStep.MetadataPublished))]
    [InlineData(nameof(CacheStep.SnapshotStaged))]
    public async Task AProcessKilledMidTransactionReleasesTheLockAndLeavesNothingTrustedThatIsTorn(string step)
    {
        await cache.InstallAsync(First, "v1.0.0");
        var payload = Path.Combine(cache.Root, "payload");
        File.WriteAllBytes(payload, Second);

        var probe = ProbeRun.Start(ProbeRun.Cache, "crash", Layout.Directory, step, payload, "v2.0.0", "Open-MBEE/OpenSysML");
        await using (probe)
        {
            var reached = await probe.ReadAsync();
            Assert.Equal(("reached", step), (reached.GetProperty("event").GetString(), reached.GetProperty("step").GetString()));
            Assert.False(await cache.IsSharedLockFreeAsync());
            probe.KillProbe();
        }

        Assert.True(await cache.IsSharedLockFreeAsync(), $"the lock outlived a holder killed at {step}");
        var contents = await Inspect();
        var installed = Enum.Parse<CacheStep>(step) >= CacheStep.ExecutablePublished;
        Assert.Equal(TestCache.Sha256(installed ? Second : First), contents.Sha256);
        Assert.Equal(Enum.Parse<CacheStep>(step) != CacheStep.ExecutablePublished, contents.Vouched);
        Assert.Equal(installed && step != nameof(CacheStep.ExecutablePublished) ? "v2.0.0" : "v1.0.0", contents.Metadata!.Version);
        Assert.Equal(cache.SettledFiles(), cache.Files());

        await cache.InstallAsync(Second, "v2.0.0");
        Assert.True((await Inspect()).Vouched);
        Assert.Equal(TestCache.Sha256(Second), (await SnapshotAsync())!.Sha256);
    }

    public void Dispose() => cache.Dispose();

    private Task<CacheTransaction> Begin(CacheSeams? seams = null) => CacheTransaction.BeginAsync(Layout, NullLogger.Instance, Token, seams: seams);

    private async Task<CacheContents> Inspect()
    {
        await using var transaction = await Begin();
        return await transaction.InspectAsync(Token);
    }

    private async Task<CacheSnapshot?> SnapshotAsync(CacheSeams? seams = null, ILogger? logger = null)
    {
        await using var transaction = await CacheTransaction.BeginAsync(Layout, logger ?? NullLogger.Instance, Token, seams: seams);
        return await transaction.SnapshotAsync(Token);
    }

    // The executable's and the record's bytes, comparable as a whole.
    private string State() => Convert.ToHexString(File.ReadAllBytes(Layout.Executable)) + "|" + Convert.ToHexString(File.ReadAllBytes(Layout.Metadata));
}
