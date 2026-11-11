using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal.Mapping;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string GetElementPropertiesMethod = "sysml.SysMLService/GetElementProperties";

    // The in-band failure of an element the model's element form does not know, as the service words it.
    private const string ElementNotFound = "element not found";

    /// <summary>
    /// Reads the metamodel properties of one element by their specification names (KerML 1.1 and SysML 2.1 abstract
    /// syntax: <c>inheritedFeature</c>, <c>connectorEnd</c>, <c>directedFeature</c>, <c>payloadType</c>, …), as the
    /// model's API element form (<see cref="ConversionFormat.ApiJson"/>) spells them, without converting the whole model.
    /// </summary>
    /// <remarks>
    /// The element is its qualified name, the positional id of an unnamed member (<c>Demo::Pair::@2</c>), or the
    /// element form's <c>@id</c>, which every <see cref="ElementRef"/> in an answer carries, so the element graph is
    /// walked with one vocabulary. A standard-library element the model references is read too.
    /// </remarks>
    /// <param name="element">The element to read.</param>
    /// <param name="properties">The specification names to read, each answered in this order; null or empty reads every property the element form carries for the element, in its order.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The element and one entry per property, each with its values or why it has none.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="element"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="element"/> is empty or white space, or a property name is null, empty or white space; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.ElementProperties"/>; nothing was sent.</exception>
    /// <exception cref="SymbolNotFoundException">The model's element form knows no such element, as the service worded it.</exception>
    /// <exception cref="ModelException">The model has no element form: a document of it could not be read whole.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<ElementProperties> ElementPropertiesAsync(string element, IEnumerable<string>? properties = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(element);
        var names = properties?.ToList() ?? [];
        if (names.Any(string.IsNullOrWhiteSpace))
        {
            throw new ArgumentException("A property name is null, empty or white space; name each property by its specification name.", nameof(properties));
        }

        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.ElementProperties);
        var request = new Wire.GetElementPropertiesRequest { ModelHash = Hash, ElementId = element, Properties = { names } };
        var response = await Connection.ModelCallAsync<Wire.GetElementPropertiesRequest, Wire.GetElementPropertiesResponse>(GetElementPropertiesMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            throw response.Error.StartsWith(ElementNotFound, StringComparison.Ordinal)
                ? new SymbolNotFoundException(element, response.Error, GetElementPropertiesMethod)
                : new ModelException(response.Error, GetElementPropertiesMethod, []);
        }

        return ModelWire.ElementProperties(response, GetElementPropertiesMethod);
    }
}
