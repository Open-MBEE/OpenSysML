using System;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string ExportGraphsMethod = "sysml.SysMLService/ExportGraphs";

    /// <summary>
    /// Exports the lowered graph of an action or a state machine, and of every behavior it performs, as the canonical
    /// <c>graphs:&lt;version&gt;</c> JSON an external analysis engine is sent (<c>sysml -graphs</c>): the same text, kept
    /// exactly as the service wrote it.
    /// </summary>
    /// <param name="subject">The qualified name of the action or state machine, definition or usage.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The form, its version and the subject as the service resolved it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="subject"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="subject"/> is empty, which names no behavior.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.ExportGraphs"/>; nothing was sent.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.NotFound"/> for a subject no element is named by,
    /// <see cref="StatusCode.InvalidArgument"/> for one that is no action or state machine or that several elements are
    /// named by, <see cref="StatusCode.FailedPrecondition"/> for a behavior whose graph cannot be lowered.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Graphs> ExportGraphsAsync(string subject, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(subject);
        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.ExportGraphs);
        var request = new Wire.ExportGraphsRequest { ModelHash = Hash, Subject = subject };
        var response = await DeclarationCallAsync<Wire.ExportGraphsRequest, Wire.ExportGraphsResponse>(ExportGraphsMethod, request, modelFaults: false, cancellationToken)
            .ConfigureAwait(false);
        return GraphDataWire.Graphs(response, ExportGraphsMethod);
    }
}
