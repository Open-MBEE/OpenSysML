using System;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.CacheProbe;

/// <summary>
/// The probe's code inside one copy of the client: it runs only in a <see cref="OpenMBEE.OpenSysML.LoadContextProbe.CopyContext"/>,
/// called by reflection with strings, so no client type crosses into the default context.
/// </summary>
internal static class Agent
{
    private static CacheLock? held;

    /// <summary>Takes the cache lock and keeps it.</summary>
    internal static async Task<string> AcquireAsync(string directory)
    {
        held = await CacheLock.AcquireAsync(new CacheLayout(directory), TimeSpan.FromSeconds(30), NullLogger.Instance, CancellationToken.None).ConfigureAwait(false);
        return string.Empty;
    }

    /// <summary>Tries the cache lock once, releasing it if it was taken; "true" when it was.</summary>
    internal static async Task<string> TryAsync(string directory)
    {
        try
        {
            var once = await CacheLock.AcquireAsync(new CacheLayout(directory), TimeSpan.Zero, NullLogger.Instance, CancellationToken.None).ConfigureAwait(false);
            await once.DisposeAsync().ConfigureAwait(false);
            return "true";
        }
        catch (ServiceStartException)
        {
            return "false";
        }
    }

    /// <summary>Releases the lock <see cref="AcquireAsync"/> took.</summary>
    internal static async Task<string> ReleaseAsync()
    {
        await held!.DisposeAsync().ConfigureAwait(false);
        held = null;
        return string.Empty;
    }

    /// <summary>The name of the load context this copy of the client is in.</summary>
    internal static Task<string> ContextAsync() =>
        Task.FromResult(System.Runtime.Loader.AssemblyLoadContext.GetLoadContext(typeof(CacheLock).Assembly)!.Name ?? string.Empty);
}
