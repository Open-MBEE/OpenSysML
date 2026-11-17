using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>The connection's own RPC: what the service said of itself when the connection opened.</summary>
internal static class ConnectionArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ServerInfoRequest>("GetServerInfo", [], ServerInfo),
    ];

    // Opening a connection asked the service once; the answer is what the connection kept. The API
    // holds capabilities as a set, so they are rendered in code-point order.
    private static Task<ScenarioAnswer> ServerInfo(Connection connection, Wire.ServerInfoRequest request, CancellationToken cancellationToken)
    {
        var info = connection.ServerInfo;
        if (!info.Answered)
        {
            return Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Refused(StatusCode.Unimplemented, "the service did not answer GetServerInfo when the connection opened"));
        }

        var response = new Wire.ServerInfoResponse { Version = info.Version, Capabilities = { info.Capabilities.Names.Order(Spelling.CodePointOrder) } };
        return Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Answered(response));
    }
}
