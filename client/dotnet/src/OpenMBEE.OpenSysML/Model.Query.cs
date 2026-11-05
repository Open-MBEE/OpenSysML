using System;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string QueryMethod = "sysml.SysMLService/Query";

    /// <summary>
    /// Selects elements of the model by a SysML v2 API &amp; Services query: those in its scope, nested ones included,
    /// that its filter matches, each with the properties it selects, in the order its <see cref="Query.OrderBy"/>
    /// sorts them, or else the order the model declares them.
    /// </summary>
    /// <param name="query">The elements considered, the properties reported, the filter applied and the sort order; <see cref="Query.All"/> for every element.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The elements selected; none when nothing matched.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="query"/> is null.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.Query"/>, or <see cref="Capabilities.QueryOrderBy"/> for a query whose
    /// <see cref="Query.OrderBy"/> is not empty, checked in that order; nothing was sent.
    /// </exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service could not evaluate the query, with <see cref="StatusCode.InvalidArgument"/>: an unknown property, a
    /// scope element the model does not have, an ordered comparison on a property that is not ordered, a value the
    /// property cannot be compared with, or a metaclass the library does not declare.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<QueryResult> QueryAsync(Query query, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(query);
        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.Query);
        if (query.OrderBy.Count > 0)
        {
            Connection.Capabilities.Require(Capabilities.QueryOrderBy);
        }
        var request = new Wire.QueryRequest { ModelHash = Hash, Query = QueryWire.ToWire(query) };
        var response = await Connection.ModelCallAsync<Wire.QueryRequest, Wire.QueryResponse>(QueryMethod, request, cancellationToken).ConfigureAwait(false);
        return QueryWire.Result(response, QueryMethod);
    }

    /// <summary>
    /// Selects elements of the model by OSLC Query 3.0 parameter text, such as
    /// <c>oslc.where=rdf:type="PartUsage"&amp;oslc.select=sysml:name</c>, the text form of <see cref="QueryAsync"/>.
    /// </summary>
    /// <param name="oslcQuery">The query, as OSLC Query Syntax spells it.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The elements selected; none when nothing matched.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="oslcQuery"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="oslcQuery"/> is empty, which states no query.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Query"/> or <see cref="Capabilities.OslcQuery"/>, checked in that order; nothing was sent.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service could not parse or evaluate the text, with <see cref="StatusCode.InvalidArgument"/>.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<QueryResult> QueryOslcAsync(string oslcQuery, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(oslcQuery);
        Connection.ThrowIfDisposed();
        Connection.Capabilities.Require(Capabilities.Query);
        Connection.Capabilities.Require(Capabilities.OslcQuery);
        var request = new Wire.QueryRequest { ModelHash = Hash, OslcQuery = oslcQuery };
        var response = await Connection.ModelCallAsync<Wire.QueryRequest, Wire.QueryResponse>(QueryMethod, request, cancellationToken).ConfigureAwait(false);
        return QueryWire.Result(response, QueryMethod);
    }
}
