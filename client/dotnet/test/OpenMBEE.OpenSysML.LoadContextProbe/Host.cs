using System;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.Loader;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.LoadContextProbe;

/// <summary>
/// The probe in the default load context. It reaches each copy of the client only through
/// <see cref="Copy"/>, by reflection, and keeps no reference to a copy once it is unloaded, so that
/// whatever keeps a copy alive afterwards is the client's own doing.
/// </summary>
internal static class Host
{
    private const int CopyCount = 3;
    private const string LibraryName = "OpenMBEE.OpenSysML";
    private static readonly TimeSpan ExitWait = TimeSpan.FromSeconds(20);
    private static readonly TimeSpan CollectWait = TimeSpan.FromSeconds(30);
    private static readonly Copy?[] Copies = new Copy?[CopyCount];

    internal static int Run(string[] args)
    {
        try
        {
            switch (args)
            {
                case ["dispose", var wire, var binary]:
                    Dispose(wire, binary);
                    return 0;
                case ["unload", var wire, var binary]:
                    Unload(wire, [binary, binary, binary], pidFiles: null);
                    return 0;
                case ["unload-starting", var wire, var b0, var p0, var b1, var p1, var b2, var p2]:
                    Unload(wire, [b0, b1, b2], [p0, p1, p2]);
                    return 0;
                default:
                    Console.Error.WriteLine("usage: OpenMBEE.OpenSysML.LoadContextProbe (dispose|unload) <wire> <binary> | unload-starting <wire> (<binary> <pid-file>){3}");
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

    // Each copy opens and closes its connections, and only then is unloaded.
    private static void Dispose(string wire, string binary)
    {
        var services = new int[CopyCount];
        for (var i = 0; i < CopyCount; i++)
        {
            services[i] = Open(i, wire, binary);
        }

        for (var i = 0; i < CopyCount; i++)
        {
            var answer = Call(i, "CloseAsync");
            Emit(i, "closed", $"{answer},\"running\":{Running(services)}");
        }

        var copies = new WeakReference[CopyCount];
        for (var i = 0; i < CopyCount; i++)
        {
            copies[i] = Unload(i);
            Emit(i, "unloaded", $"\"running\":{Running(services)}");
        }

        Collect(copies);
    }

    // Each copy is unloaded while it holds its service, or while its service starts.
    private static void Unload(string wire, string[] binaries, string[]? pidFiles)
    {
        var services = new int[CopyCount];
        for (var i = 0; i < CopyCount; i++)
        {
            if (pidFiles is null)
            {
                services[i] = Open(i, wire, binaries[i]);
            }
            else
            {
                services[i] = StartOpening(i, wire, binaries[i], pidFiles[i]);
            }
        }

        var copies = new WeakReference[CopyCount];
        for (var i = 0; i < CopyCount; i++)
        {
            copies[i] = Unload(i);
            var exited = AwaitExit(services[i]);
            Emit(i, "unloaded", $"\"exited\":{Json(exited)},\"running\":{Running(services)}");
        }

        Collect(copies);
    }

    // Every reference to a copy lives in a method of its own, which no caller's frame outlives.
    [MethodImpl(MethodImplOptions.NoInlining)]
    private static int Open(int index, string wire, string binary)
    {
        Copies[index] = Copy.Load(index);
        var answer = Call(index, "OpenAsync", binary, wire);
        Emit(index, "opened", answer);
        using var document = JsonDocument.Parse("{" + answer + "}");
        return document.RootElement.GetProperty("service").GetInt32();
    }

    [MethodImpl(MethodImplOptions.NoInlining)]
    private static int StartOpening(int index, string wire, string binary, string pidFile)
    {
        Copies[index] = Copy.Load(index);
        Call(index, "StartOpeningAsync", binary, wire);
        var service = ReadProcessId(pidFile);
        Emit(index, "starting", string.Create(CultureInfo.InvariantCulture, $"\"service\":{service}"));
        return service;
    }

    [MethodImpl(MethodImplOptions.NoInlining)]
    private static string Call(int index, string method, params string[] arguments) => Copies[index]!.Call(method, arguments);

    [MethodImpl(MethodImplOptions.NoInlining)]
    private static WeakReference Unload(int index)
    {
        var context = Copies[index]!.Context;
        Copies[index] = null;
        context.Unload();
        return new WeakReference(context);
    }

    private static void Collect(WeakReference[] copies)
    {
        var watch = Stopwatch.StartNew();
        while (copies.Any(copy => copy.IsAlive) && watch.Elapsed < CollectWait)
        {
            GC.Collect();
            GC.WaitForPendingFinalizers();
            GC.Collect();
            Thread.Sleep(50);
        }

        var collected = string.Join(',', copies.Select(copy => Json(!copy.IsAlive)));
        var inDefault = AssemblyLoadContext.Default.Assemblies.Any(assembly => assembly.GetName().Name == LibraryName);
        Console.Out.WriteLine(string.Create(
            CultureInfo.InvariantCulture,
            $"{{\"event\":\"collected\",\"copies\":[{collected}],\"libraryInDefault\":{Json(inDefault)},\"seconds\":{watch.Elapsed.TotalSeconds:F1}}}"));
        Console.Out.Flush();
    }

    private static int ReadProcessId(string file)
    {
        var watch = Stopwatch.StartNew();
        while (watch.Elapsed < ExitWait)
        {
            if (File.Exists(file) && File.ReadAllText(file) is { Length: > 0 } text)
            {
                return int.Parse(text, CultureInfo.InvariantCulture);
            }

            Thread.Sleep(20);
        }

        throw new TimeoutException($"{file} was not written within {ExitWait}.");
    }

    private static bool AwaitExit(int processId)
    {
        var watch = Stopwatch.StartNew();
        while (IsRunning(processId))
        {
            if (watch.Elapsed > ExitWait)
            {
                return false;
            }

            Thread.Sleep(20);
        }

        return true;
    }

    private static string Running(int[] services) => "[" + string.Join(',', services.Select(service => Json(IsRunning(service)))) + "]";

    private static bool IsRunning(int processId)
    {
        try
        {
            using var process = Process.GetProcessById(processId);
            return !process.HasExited;
        }
        catch (ArgumentException)
        {
            return false;
        }
        catch (InvalidOperationException)
        {
            return false;
        }
    }

    private static void Emit(int index, string step, string fields)
    {
        Console.Out.WriteLine(string.Create(CultureInfo.InvariantCulture, $"{{\"event\":\"{step}\",\"copy\":{index}{(fields.Length > 0 ? "," : string.Empty)}{fields}}}"));
        Console.Out.Flush();
    }

    private static string Json(bool value) => value ? "true" : "false";

    /// <summary>One loaded copy: its context, and its <see cref="Agent"/> type reached by name.</summary>
    private sealed class Copy(CopyContext context, Type agent)
    {
        // By name: naming the type would make the default context load what its fields refer to.
        private const string AgentType = "OpenMBEE.OpenSysML.LoadContextProbe.Agent";

        internal CopyContext Context => context;

        internal static Copy Load(int index)
        {
            var main = typeof(Host).Assembly.Location;
            var context = new CopyContext($"copy-{index}", main);
            return new Copy(context, context.LoadFromAssemblyPath(main).GetType(AgentType, throwOnError: true)!);
        }

        internal string Call(string method, string[] arguments)
        {
            var target = agent.GetMethod(method, BindingFlags.NonPublic | BindingFlags.Static)
                ?? throw new MissingMethodException(AgentType, method);
            return ((Task<string>)target.Invoke(null, arguments)!).GetAwaiter().GetResult();
        }
    }
}
