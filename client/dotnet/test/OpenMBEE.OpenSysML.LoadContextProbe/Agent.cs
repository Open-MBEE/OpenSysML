using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Runtime.Loader;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.LoadContextProbe;

/// <summary>
/// The probe's code inside one copy of the client: it runs only in a <see cref="CopyContext"/>, called
/// by reflection with strings, and answers with JSON, so no client type crosses into the default context.
/// </summary>
internal static class Agent
{
    private static readonly List<Connection> Connections = [];

    /// <summary>Opens two connections that share this copy's private service.</summary>
    internal static async Task<string> OpenAsync(string binary, string wire)
    {
        var options = Options(binary, wire);
        Connections.Add(await Connection.OpenAsync(options).ConfigureAwait(false));
        Connections.Add(await Connection.OpenAsync(options).ConfigureAwait(false));
        var entry = ServiceRegistry.Shared.Entries().Single();
        var context = AssemblyLoadContext.GetLoadContext(typeof(Connection).Assembly)!;
        var shared = Connections.Select(connection => connection.Address).Distinct().Count() == 1;
        return string.Create(
            CultureInfo.InvariantCulture,
            $"\"service\":{entry.ProcessId},\"holders\":{entry.Holders},\"sameAddress\":{Json(shared)},\"context\":\"{context.Name}\",\"collectible\":{Json(context.IsCollectible)}");
    }

    /// <summary>Starts opening a connection, without waiting for it; it never completes before the copy is unloaded.</summary>
    internal static Task<string> StartOpeningAsync(string binary, string wire)
    {
        _ = Connection.OpenAsync(Options(binary, wire));
        return Task.FromResult(string.Empty);
    }

    /// <summary>Closes every connection this copy opened.</summary>
    internal static async Task<string> CloseAsync()
    {
        foreach (var connection in Connections)
        {
            await connection.DisposeAsync().ConfigureAwait(false);
        }

        Connections.Clear();
        return string.Create(CultureInfo.InvariantCulture, $"\"entries\":{ServiceRegistry.Shared.Entries().Count}");
    }

    private static ConnectionOptions Options(string binary, string wire) => wire switch
    {
        "connect-proto" => new ConnectionOptions { BinaryPath = binary, Protocol = Protocol.Connect },
        "connect-json" => new ConnectionOptions { BinaryPath = binary, Protocol = Protocol.Connect, Encoding = MessageEncoding.Json },
        _ => new ConnectionOptions { BinaryPath = binary },
    };

    private static string Json(bool value) => value ? "true" : "false";
}
