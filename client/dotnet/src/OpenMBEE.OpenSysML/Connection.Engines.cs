using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Connection
{
    internal const string ListEnginesMethod = "sysml.SysMLService/ListEngines";

    /// <summary>
    /// Lists the analysis engines the service registers, in name order, as <c>sysml -engines</c> does: each with the
    /// questions it answers, the strongest evidence it can produce and whether it can run here.
    /// </summary>
    /// <param name="options">Whether to probe each external engine; null for <see cref="ListEnginesOptions.Default"/>, which starts no process.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One listing per engine, in name order.</returns>
    /// <exception cref="CapabilityException">The service lacks <see cref="OpenSysML.Capabilities.Engines"/>; nothing was sent.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<ValueList<EngineInfo>> ListEnginesAsync(ListEnginesOptions? options = null, CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        options ??= ListEnginesOptions.Default;
        Capabilities.Require(OpenSysML.Capabilities.Engines);
        var response = await UnaryAsync<Wire.ListEnginesRequest, Wire.ListEnginesResponse>(
            ListEnginesMethod, new Wire.ListEnginesRequest { Probe = options.Probe }, cancellationToken).ConfigureAwait(false);
        return new ValueList<EngineInfo>(response.Engines.Select(VerificationWire.Engine));
    }
}
