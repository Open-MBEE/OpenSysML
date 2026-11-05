using System;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string RenderViewMethod = "sysml.SysMLService/RenderView";

    /// <summary>
    /// Renders a view the model declares, or a targeted pseudo-view, as machine-readable diagram data: its nodes and
    /// edges, a tabular rendering's columns and rows, and the canvas, layout, styles, notes and notices the view
    /// states, each absent where the view states none.
    /// </summary>
    /// <param name="view">A view's qualified name, or a <c>#&lt;kind&gt;:&lt;qualified name&gt;</c> pseudo-view.</param>
    /// <param name="options">The ports drawn, the optional notation shown, the depth drawn and the compartments folded; null for <see cref="RenderViewOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>Every field the view renderer produced.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="view"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="view"/> is empty.</exception>
    /// <exception cref="InvalidRequestException">
    /// <see cref="RenderViewOptions.Ports"/> is not a <see cref="RenderViewPorts"/>, <see cref="RenderViewOptions.Compartments"/>
    /// is not a <see cref="RenderViewCompartments"/>, or <see cref="RenderViewOptions.Depth"/> is not positive; nothing was sent.
    /// </exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.RenderView"/>; nothing was sent.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.NotFound"/> for a view the model does not declare,
    /// <see cref="StatusCode.InvalidArgument"/> for a name that selects no view or a notation element it does not know.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<RenderedView> RenderViewAsync(string view, RenderViewOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(view);
        Connection.ThrowIfDisposed();
        options ??= RenderViewOptions.Default;
        if (!Enum.IsDefined(options.Ports))
        {
            throw new InvalidRequestException($"RenderViewOptions.Ports is {options.Ports}, which is not a RenderViewPorts.");
        }

        if (!Enum.IsDefined(options.Compartments))
        {
            throw new InvalidRequestException($"RenderViewOptions.Compartments is {options.Compartments}, which is not a RenderViewCompartments.");
        }

        if (options.Depth is <= 0 and var depth)
        {
            throw new InvalidRequestException($"RenderViewOptions.Depth is {depth}, which draws no level; leave it null for the view's own depth.");
        }

        Connection.Capabilities.Require(Capabilities.RenderView);

        // Minimal ports and positioned compartments are what empty selections name, which every service draws.
        var request = new Wire.RenderViewRequest
        {
            ModelHash = Hash,
            View = view,
            Ports = options.Ports == RenderViewPorts.Full ? "full" : string.Empty,
            Show = { options.Show },
            Depth = options.Depth ?? 0,
            Compartments = options.Compartments switch
            {
                RenderViewCompartments.Most => "most",
                RenderViewCompartments.Ports => "ports",
                _ => string.Empty,
            },
        };
        var response = await DeclarationCallAsync<Wire.RenderViewRequest, Wire.RenderViewResponse>(RenderViewMethod, request, modelFaults: false, cancellationToken)
            .ConfigureAwait(false);
        return ViewWire.Rendered(response, RenderViewMethod);
    }
}
