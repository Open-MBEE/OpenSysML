using System;
using System.Diagnostics;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.LoadContextProbe;

/// <summary>
/// One copy of a probe, and so of the client, loaded into a <see cref="CopyContext"/> of its own, with the probe's
/// agent reached by name: naming the type would make the default context load what its fields refer to. Both
/// lifecycle probes load, call and unload their copies through it, each keeping its copies in frames of its own.
/// </summary>
internal sealed class ClientCopy
{
    private readonly Type agent;

    private ClientCopy(CopyContext context, Type agent)
    {
        Context = context;
        this.agent = agent;
    }

    /// <summary>The context the copy is loaded into.</summary>
    internal CopyContext Context { get; }

    /// <summary>Loads a copy of a probe's main assembly into a new collectible context.</summary>
    /// <param name="name">The context's name.</param>
    /// <param name="probe">The probe's main assembly, whose dependency manifest the copy resolves by.</param>
    /// <param name="agentType">The full name of the probe's agent, the static class the copy is called through.</param>
    /// <returns>The copy.</returns>
    internal static ClientCopy Load(string name, Assembly probe, string agentType)
    {
        var context = new CopyContext(name, probe.Location);
        return new ClientCopy(context, context.LoadFromAssemblyPath(probe.Location).GetType(agentType, throwOnError: true)!);
    }

    /// <summary>Calls one of the agent's static methods, which take and answer strings, and waits for its answer.</summary>
    /// <param name="method">The method's name.</param>
    /// <param name="arguments">Its arguments.</param>
    /// <returns>What it answered.</returns>
    internal string Call(string method, params string[] arguments)
    {
        var target = agent.GetMethod(method, BindingFlags.NonPublic | BindingFlags.Static)
            ?? throw new MissingMethodException(agent.FullName ?? agent.Name, method);
        return ((Task<string>)target.Invoke(null, arguments)!).GetAwaiter().GetResult();
    }

    /// <summary>Unloads the copy.</summary>
    /// <returns>A weak reference to its context, alive until the context is collected.</returns>
    internal WeakReference Unload()
    {
        Context.Unload();
        return new WeakReference(Context);
    }

    /// <summary>Collects garbage until every copy's context is collected or the wait is over.</summary>
    /// <param name="copies">Weak references to the copies' contexts.</param>
    /// <param name="wait">How long to keep collecting.</param>
    /// <returns>How long it took.</returns>
    internal static TimeSpan AwaitCollected(WeakReference[] copies, TimeSpan wait)
    {
        var watch = Stopwatch.StartNew();
        while (copies.Any(copy => copy.IsAlive) && watch.Elapsed < wait)
        {
            GC.Collect();
            GC.WaitForPendingFinalizers();
            GC.Collect();
            Thread.Sleep(50);
        }

        return watch.Elapsed;
    }
}
