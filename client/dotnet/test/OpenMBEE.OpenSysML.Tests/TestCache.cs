using System;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A shared cache of the test's own, in a temporary directory removed when the test ends, and the cache probe
/// (test/OpenMBEE.OpenSysML.CacheProbe) to hold or try its lock from a process of its own.
/// </summary>
internal sealed class TestCache : IDisposable
{
    internal TestCache()
    {
        Root = Directory.CreateTempSubdirectory("opensysml-cache-").FullName;
        Layout = new CacheLayout(Path.Combine(Root, ".opensysml", "bin"));
    }

    /// <summary>The home directory the cache is made in, as <c>~/.opensysml/bin</c>.</summary>
    internal string Root { get; }

    /// <summary>The cache, which nothing has made yet.</summary>
    internal CacheLayout Layout { get; }

    /// <summary>The digest of some bytes, in lower-case hexadecimal.</summary>
    internal static string Sha256(byte[] bytes) => Convert.ToHexStringLower(SHA256.HashData(bytes));

    /// <summary>The names in the cache directory, in order.</summary>
    internal string[] Files() =>
        Directory.Exists(Layout.Directory)
            ? [.. Directory.EnumerateFileSystemEntries(Layout.Directory).Select(path => Path.GetFileName(path)).Order(StringComparer.Ordinal)]
            : [];

    /// <summary>The names a cache holding a recorded release has, and nothing staged.</summary>
    internal string[] SettledFiles(params string[] snapshots) =>
        [.. new[] { Name(Layout.Executable), Name(Layout.Metadata), Name(Layout.Lock), Name(Layout.Guard) }.Concat(snapshots.Select(Name)).Order(StringComparer.Ordinal)];

    /// <summary>Installs bytes as a release in a transaction of their own.</summary>
    internal async Task<CacheInstall> InstallAsync(byte[] bytes, string version = "v1.0.0", string repo = "Open-MBEE/OpenSysML", ILogger? logger = null)
    {
        await using var transaction = await CacheTransaction.BeginAsync(Layout, logger ?? NullLogger.Instance, TestContext.Current.CancellationToken);
        return await transaction.InstallAsync(Writing(bytes), Sha256(bytes), version, repo, TestContext.Current.CancellationToken);
    }

    /// <summary>Writes bytes to a stream.</summary>
    internal static Func<Stream, CancellationToken, Task> Writing(byte[] bytes) => (stream, token) => stream.WriteAsync(bytes, token).AsTask();

    /// <summary>Makes the lock file, as any client would, before any lock is held in this process.</summary>
    internal void MakeLockFile()
    {
        Directory.CreateDirectory(Layout.Directory);
        if (!File.Exists(Layout.Lock))
        {
            File.WriteAllBytes(Layout.Lock, []);
        }
    }

    /// <summary>Whether another process using this client could take the lock now.</summary>
    internal async Task<bool> IsFreeToAnotherProcessAsync() => Free(await Run("try", Layout.Directory));

    /// <summary>Whether another process could take the cross-client lock alone now, by its own system calls.</summary>
    internal async Task<bool> IsSharedLockFreeAsync() => Free(await Run("try-shared", Layout.Lock));

    /// <summary>Starts the probe holding the lock through this client, once it holds it.</summary>
    internal Task<ProbeRun> HeldByAnotherProcessAsync() => HoldAsync("hold", Layout.Directory);

    /// <summary>Starts the probe holding the cross-client lock alone, as another client would, once it holds it.</summary>
    internal Task<ProbeRun> HeldByAnotherClientAsync()
    {
        MakeLockFile();
        return HoldAsync("hold-shared", Layout.Lock);
    }

    /// <summary>Has a holding probe release the lock and exit.</summary>
    internal static async Task ReleaseAsync(ProbeRun holder)
    {
        await holder.SendLineAsync();
        var events = await holder.ReadToExitAsync();
        Assert.Equal("released", events[^1].GetProperty("event").GetString());
    }

    /// <summary>
    /// Whether this process is refused a file in a directory it may not write, which a privileged user is not;
    /// tests of what such a directory refuses skip without it.
    /// </summary>
    internal bool PermissionsAreEnforced()
    {
        if (OperatingSystem.IsWindows())
        {
            return false;
        }

        var directory = Path.Combine(Root, "sealed-check");
        Directory.CreateDirectory(directory);
        File.SetUnixFileMode(directory, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        try
        {
            File.WriteAllBytes(Path.Combine(directory, "written"), []);
            return false;
        }
        catch (UnauthorizedAccessException)
        {
            return true;
        }
        finally
        {
            File.SetUnixFileMode(directory, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }
    }

    public void Dispose()
    {
        try
        {
            if (!OperatingSystem.IsWindows())
            {
                foreach (var directory in Directory.EnumerateDirectories(Root, "*", SearchOption.AllDirectories).Prepend(Root))
                {
                    File.SetUnixFileMode(directory, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
                }
            }

            Directory.Delete(Root, recursive: true);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
        }
    }

    private static string Name(string path) => Path.GetFileName(path);

    private static bool Free(JsonElement answer)
    {
        Assert.Equal("tried", answer.GetProperty("event").GetString());
        return answer.GetProperty("free").GetBoolean();
    }

    private static async Task<JsonElement> Run(params string[] arguments)
    {
        await using var probe = ProbeRun.Start(ProbeRun.Cache, arguments);
        return Assert.Single(await probe.ReadToExitAsync());
    }

    private static async Task<ProbeRun> HoldAsync(params string[] arguments)
    {
        var probe = ProbeRun.Start(ProbeRun.Cache, arguments);
        try
        {
            var held = await probe.ReadAsync();
            Assert.Equal("held", held.GetProperty("event").GetString());
            return probe;
        }
        catch
        {
            await probe.DisposeAsync();
            throw;
        }
    }
}
