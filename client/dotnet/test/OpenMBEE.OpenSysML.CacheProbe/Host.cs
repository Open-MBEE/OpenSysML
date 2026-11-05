using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.Loader;
using System.Security.Cryptography;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using OpenMBEE.OpenSysML.LoadContextProbe;

namespace OpenMBEE.OpenSysML.CacheProbe;

/// <summary>The probe's modes; see Program.cs.</summary>
internal static class Host
{
    private const string LibraryName = "OpenMBEE.OpenSysML";
    private static readonly TimeSpan CollectWait = TimeSpan.FromSeconds(30);

    internal static async Task<int> RunAsync(string[] args)
    {
        try
        {
            switch (args)
            {
                case ["hold", var directory]:
                    await Direct.HoldAsync(directory);
                    return 0;
                case ["try", var directory]:
                    await Direct.TryAsync(directory);
                    return 0;
                case ["hold-shared", var lockFile]:
                    SharedLock.Hold(lockFile, () =>
                    {
                        Emit("held", ("probe", Environment.ProcessId));
                        Console.In.ReadLine();
                    });
                    Emit("released");
                    return 0;
                case ["try-shared", var lockFile]:
                    Emit("tried", ("free", SharedLock.IsFree(lockFile)));
                    return 0;
                case ["hold-and-start", var directory]:
                    await Direct.HoldAndStartAsync(directory);
                    return 0;
                case ["sleep"]:
                    Thread.Sleep(TimeSpan.FromMinutes(2));
                    return 0;
                case ["copies", var directory]:
                    Copies(directory);
                    return 0;
                case ["crash", var directory, var step, var payload, var version, var repo]:
                    await Direct.CrashAsync(directory, step, payload, version, repo);
                    return 0;
                default:
                    Console.Error.WriteLine("usage: OpenMBEE.OpenSysML.CacheProbe hold|try <cache-dir> | hold-shared|try-shared <lock-file> | hold-and-start <cache-dir> | sleep | copies <cache-dir> | crash <cache-dir> <step> <payload-file> <version> <repo>");
                    return 2;
            }
        }
#pragma warning disable CA1031 // Whatever failed is the probe's answer; the test reads it from stderr.
        catch (Exception error)
#pragma warning restore CA1031
        {
            Console.Error.WriteLine($"probe failed: {error}");
            return 1;
        }
    }

    internal static void Emit(string step, params (string Name, object Value)[] fields)
    {
        var line = new Dictionary<string, object> { ["event"] = step };
        foreach (var (name, value) in fields)
        {
            line[name] = value;
        }

        Console.Out.WriteLine(JsonSerializer.Serialize(line));
        Console.Out.Flush();
    }

    // Two copies of the client, which share nothing but the process, contend for one cache.
    private static void Copies(string directory)
    {
        var copies = new WeakReference[2];
        Contend(directory, copies);
        var watch = Stopwatch.StartNew();
        while (copies.Any(copy => copy.IsAlive) && watch.Elapsed < CollectWait)
        {
            GC.Collect();
            GC.WaitForPendingFinalizers();
            GC.Collect();
            Thread.Sleep(50);
        }

        Emit(
            "collected",
            ("copies", copies.Select(copy => !copy.IsAlive).ToArray()),
            ("libraryInDefault", AssemblyLoadContext.Default.Assemblies.Any(assembly => assembly.GetName().Name == LibraryName)));
    }

    // Every reference to a copy lives in this frame, which the collection loop does not share.
    [MethodImpl(MethodImplOptions.NoInlining)]
    private static void Contend(string directory, WeakReference[] unloaded)
    {
        var first = Copy.Load(0);
        var second = Copy.Load(1);
        first.Call("AcquireAsync", directory);
        Emit("held", ("copy", 0), ("context", first.Call("ContextAsync")));
        Console.In.ReadLine();
        Emit("tried", ("copy", 1), ("free", bool.Parse(second.Call("TryAsync", directory))), ("context", second.Call("ContextAsync")));
        first.Call("ReleaseAsync");
        Emit("released", ("copy", 0));
        Emit("tried", ("copy", 1), ("free", bool.Parse(second.Call("TryAsync", directory))));
        second.Call("AcquireAsync", directory);
        Emit("tried", ("copy", 0), ("free", bool.Parse(first.Call("TryAsync", directory))));
        second.Call("ReleaseAsync");
        Emit("released", ("copy", 1));
        unloaded[0] = first.Unload();
        unloaded[1] = second.Unload();
    }

    /// <summary>One loaded copy: its context, and its <see cref="Agent"/> type reached by name.</summary>
    private sealed class Copy(CopyContext context, Type agent)
    {
        private const string AgentType = "OpenMBEE.OpenSysML.CacheProbe.Agent";

        internal static Copy Load(int index)
        {
            var main = typeof(Host).Assembly.Location;
            var context = new CopyContext($"cache-copy-{index}", main);
            return new Copy(context, context.LoadFromAssemblyPath(main).GetType(AgentType, throwOnError: true)!);
        }

        internal string Call(string method, params string[] arguments)
        {
            var target = agent.GetMethod(method, BindingFlags.NonPublic | BindingFlags.Static)
                ?? throw new MissingMethodException(AgentType, method);
            return ((Task<string>)target.Invoke(null, arguments)!).GetAwaiter().GetResult();
        }

        internal WeakReference Unload()
        {
            context.Unload();
            return new WeakReference(context);
        }
    }

    /// <summary>The modes that use the client in the default context, kept apart from <see cref="Copies"/>.</summary>
    private static class Direct
    {
        internal static async Task HoldAsync(string directory)
        {
            var held = await CacheLock.AcquireAsync(new CacheLayout(directory), CacheLock.DefaultWait, NullLogger.Instance, CancellationToken.None);
            await using (held)
            {
                Emit("held", ("probe", Environment.ProcessId));
                await Console.In.ReadLineAsync();
            }

            Emit("released");
        }

        // The child shares this probe's standard streams and nothing else it was not given: a descriptor
        // to a lock file it inherited would keep the guard held after this probe is killed.
        internal static async Task HoldAndStartAsync(string directory)
        {
            var held = await CacheLock.AcquireAsync(new CacheLayout(directory), CacheLock.DefaultWait, NullLogger.Instance, CancellationToken.None);
            await using (held)
            {
                var start = new ProcessStartInfo(Environment.ProcessPath!) { UseShellExecute = false };
                start.ArgumentList.Add("sleep");
                using var child = Process.Start(start)!;
                Emit("held", ("probe", Environment.ProcessId), ("siblings", new[] { child.Id }));
                await Console.In.ReadLineAsync();
            }
        }

        internal static async Task TryAsync(string directory)
        {
            bool free;
            try
            {
                await using var held = await CacheLock.AcquireAsync(new CacheLayout(directory), TimeSpan.Zero, NullLogger.Instance, CancellationToken.None);
                free = true;
            }
            catch (ServiceStartException)
            {
                free = false;
            }

            Emit("tried", ("free", free));
        }

        internal static async Task CrashAsync(string directory, string step, string payloadFile, string version, string repo)
        {
            var payload = await File.ReadAllBytesAsync(payloadFile);
            var stop = Enum.Parse<CacheStep>(step);
            var seams = new CacheSeams
            {
                Reached = async (reached, _) =>
                {
                    if (reached == stop)
                    {
                        Emit("reached", ("step", step), ("probe", Environment.ProcessId));
                        await Task.Delay(Timeout.Infinite, CancellationToken.None);
                    }
                },
            };
            var transaction = await CacheTransaction.BeginAsync(new CacheLayout(directory), NullLogger.Instance, CancellationToken.None, seams: seams);
            await using (transaction)
            {
                await transaction.InstallAsync((file, token) => file.WriteAsync(payload, token).AsTask(), Convert.ToHexStringLower(SHA256.HashData(payload)), version, repo, CancellationToken.None);
                await transaction.SnapshotAsync(CancellationToken.None);
            }

            Emit("finished");
        }
    }
}
