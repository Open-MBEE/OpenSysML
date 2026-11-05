using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string GetSymbolMethod = "sysml.SysMLService/GetSymbol";

    private const string PropertyId = "@id";
    private const string PropertyName = "name";
    private const string PropertyOwner = "owner";
    private const int MaxSuggestions = 3;

    /// <summary>
    /// A symbol by qualified name, as the service resolves it: through imports and aliases, as the
    /// notation does, so the symbol answered may carry another <see cref="Symbol.Id"/>.
    /// </summary>
    /// <param name="symbolId">A qualified name, such as <c>Demo::Vehicle</c>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The symbol.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="SymbolNotFoundException">The model declares no such symbol, as the service worded it; no suggestion is sought.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Symbol> SymbolAsync(string symbolId, CancellationToken cancellationToken = default)
    {
        var (symbol, error) = await ReadSymbolAsync(symbolId, cancellationToken).ConfigureAwait(false);
        return symbol ?? throw new SymbolNotFoundException(symbolId, error, GetSymbolMethod);
    }

    /// <summary>A symbol by qualified name, as the service resolves it, or null when the model declares no such symbol.</summary>
    /// <param name="symbolId">A qualified name, such as <c>Demo::Vehicle</c>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The symbol, or null.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="symbolId"/> is null.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Symbol?> FindSymbolAsync(string symbolId, CancellationToken cancellationToken = default) =>
        (await ReadSymbolAsync(symbolId, cancellationToken).ConfigureAwait(false)).Symbol;

    /// <summary>
    /// A symbol by short name (<c>Vehicle</c>) or by its own <see cref="Symbol.Id"/> (<c>Demo::Vehicle</c>),
    /// or null when the model declares none. Several symbols may share a short name: the outermost wins,
    /// and among those the one declared first. A name the model declares wins over a library symbol
    /// whose id it is.
    /// </summary>
    /// <remarks>
    /// A service advertising <see cref="Capabilities.Query"/> answers which elements carry a name in one
    /// or two calls whatever the size of the model; without it the model is walked from its roots, one
    /// call per symbol, and a model named by hash alone, which has no roots to walk, needs the capability.
    /// </remarks>
    /// <param name="name">A short or qualified name.</param>
    /// <param name="cancellationToken">Cancels the calls.</param>
    /// <returns>The symbol, or null.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="name"/> is empty.</exception>
    /// <exception cref="CapabilityException">The model has no roots to walk and the service lacks <see cref="Capabilities.Query"/>; nothing was sent.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused a call.</exception>
    /// <exception cref="TransportException">A call failed in transport, or an answer was malformed.</exception>
    public async Task<Symbol?> FindAsync(string name, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(name);
        Connection.ThrowIfDisposed();
        foreach (var root in Roots)
        {
            if (root.Name == name || root.Id == name)
            {
                return root;
            }
        }

        if (name.Contains("::", StringComparison.Ordinal))
        {
            return await SymbolWithIdAsync(name, cancellationToken).ConfigureAwait(false)
                ?? await SymbolNamedAsync(name, cancellationToken).ConfigureAwait(false);
        }

        return await SymbolNamedAsync(name, cancellationToken).ConfigureAwait(false)
            ?? await SymbolWithIdAsync(name, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// The raising counterpart of <see cref="FindAsync"/>: a symbol by short or qualified name, failing
    /// with the declared names closest to it when there is none.
    /// </summary>
    /// <param name="name">A short or qualified name.</param>
    /// <param name="cancellationToken">Cancels the calls.</param>
    /// <returns>The symbol.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="name"/> is empty.</exception>
    /// <exception cref="SymbolNotFoundException">The model declares no such symbol; its <see cref="SymbolNotFoundException.Suggestions"/> name at most three declared names it may be a typo of.</exception>
    /// <exception cref="CapabilityException">The model has no roots to walk and the service lacks <see cref="Capabilities.Query"/>; nothing was sent.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused a call.</exception>
    /// <exception cref="TransportException">A call failed in transport, or an answer was malformed.</exception>
    public async Task<Symbol> LookupAsync(string name, CancellationToken cancellationToken = default) =>
        await FindAsync(name, cancellationToken).ConfigureAwait(false)
            ?? throw new SymbolNotFoundException(name, await NearNamesAsync(name, cancellationToken).ConfigureAwait(false), GetSymbolMethod);

    // The symbol, or the one failure the service reports in band: a name that resolves to no symbol.
    private async Task<(Symbol? Symbol, string Error)> ReadSymbolAsync(string symbolId, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(symbolId);
        Connection.ThrowIfDisposed();
        var request = new Wire.GetSymbolRequest { ModelHash = Hash, SymbolId = symbolId };
        var response = await Connection.ModelCallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(GetSymbolMethod, request, cancellationToken).ConfigureAwait(false);
        if (response.Error.Length > 0)
        {
            return (null, response.Error);
        }

        return response.Symbol is { } symbol
            ? (ModelWire.Symbol(symbol, GetSymbolMethod, "symbol"), string.Empty)
            : throw new TransportException(StatusCode.Internal, $"{GetSymbolMethod} answered neither a symbol nor a failure for {symbolId}.", GetSymbolMethod);
    }

    private static Query Selecting(string select, string? property = null, IEnumerable<string>? values = null) =>
        new() { Select = [select], Where = property is null ? null : QueryConstraint.Equal(property, values!) };

    // The symbol whose own id is this one: the service also resolves a name through imports and aliases.
    private async Task<Symbol?> SymbolWithIdAsync(string id, CancellationToken cancellationToken)
    {
        var symbol = await FindSymbolAsync(id, cancellationToken).ConfigureAwait(false);
        return symbol is not null && symbol.Id == id ? symbol : null;
    }

    // The outermost, first-declared symbol of a short name. A model with errors may hold a name the
    // query cannot see at any depth, so there the tree is also walked, as deep as the query's best answer.
    private async Task<Symbol?> SymbolNamedAsync(string name, CancellationToken cancellationToken)
    {
        if (!Connection.Capabilities.Has(Capabilities.Query))
        {
            return await WalkAsync(symbol => symbol.Name == name, maxDepth: null, cancellationToken).ConfigureAwait(false);
        }

        var named = (await QueryAsync(Selecting(PropertyOwner, PropertyName, [name]), cancellationToken).ConfigureAwait(false)).Elements;
        var ids = named.Select(element => element.Id).ToList();
        Dictionary<string, int>? depth = null;
        if (ids.Count > 1 || (ids.Count > 0 && HasErrors))
        {
            var depths = await DepthsAsync(named, cancellationToken).ConfigureAwait(false);
            ids = ids.OrderBy(id => depths[id]).ToList();
            depth = depths;
        }

        if (HasErrors)
        {
            var walked = await WalkAsync(symbol => symbol.Name == name, ids.Count > 0 ? depth![ids[0]] : null, cancellationToken).ConfigureAwait(false);
            if (walked is not null)
            {
                return walked;
            }
        }

        foreach (var id in ids)
        {
            if (await SymbolWithIdAsync(id, cancellationToken).ConfigureAwait(false) is { } symbol)
            {
                return symbol;
            }
        }

        return null;
    }

    // Each element's depth below its document's root, counted up its owner chain rather than from its
    // id's segments, which a quoted name may contain. A chain ending short of a root ends one level below one.
    private async Task<Dictionary<string, int>> DepthsAsync(ValueList<QueryElement> elements, CancellationToken cancellationToken)
    {
        var owner = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var root in Roots)
        {
            owner[root.Id] = string.Empty;
        }

        foreach (var element in elements)
        {
            owner[element.Id] = element.Properties.GetValueOrDefault(PropertyOwner, string.Empty);
        }

        var unknown = UnknownOwners(owner);
        while (unknown.Count > 0)
        {
            var asked = Selecting(PropertyOwner, PropertyId, unknown.Order(StringComparer.Ordinal));
            foreach (var element in (await QueryAsync(asked, cancellationToken).ConfigureAwait(false)).Elements)
            {
                owner[element.Id] = element.Properties.GetValueOrDefault(PropertyOwner, string.Empty);
            }

            foreach (var id in unknown)
            {
                owner.TryAdd(id, string.Empty);
            }

            unknown = UnknownOwners(owner);
        }

        var roots = Roots.Select(root => root.Id).ToHashSet(StringComparer.Ordinal);
        var depths = new Dictionary<string, int>(StringComparer.Ordinal);
        foreach (var element in elements)
        {
            var id = element.Id;
            var hops = 0;
            while (owner.GetValueOrDefault(id, string.Empty) is { Length: > 0 } next)
            {
                if (++hops > owner.Count)
                {
                    throw new TransportException(StatusCode.Internal, $"{QueryMethod} answered owners that form a cycle through {element.Id}.", QueryMethod);
                }

                id = next;
            }

            depths[element.Id] = roots.Contains(id) ? hops : hops + 1;
        }

        return depths;
    }

    private static HashSet<string> UnknownOwners(Dictionary<string, string> owner) =>
        owner.Values.Where(id => id.Length > 0 && !owner.ContainsKey(id)).ToHashSet(StringComparer.Ordinal);

    // Breadth first from the roots, one call per symbol, the first symbol stop accepts; maxDepth
    // bounds the walk to the symbols that many levels below a root.
    private async Task<Symbol?> WalkAsync(Func<Symbol, bool> stop, int? maxDepth, CancellationToken cancellationToken)
    {
        if (Roots.IsEmpty)
        {
            Connection.Capabilities.Require(Capabilities.Query);
        }

        var queue = new Queue<(Symbol Symbol, int Level)>(Roots.Select(root => (root, 0)));
        var seen = new HashSet<string>(StringComparer.Ordinal);
        while (queue.TryDequeue(out var current))
        {
            if (maxDepth is { } limit && current.Level >= limit)
            {
                continue;
            }

            foreach (var childId in current.Symbol.ChildIds)
            {
                if (!seen.Add(childId) || await FindSymbolAsync(childId, cancellationToken).ConfigureAwait(false) is not { } child)
                {
                    continue;
                }

                if (stop(child))
                {
                    return child;
                }

                queue.Enqueue((child, current.Level + 1));
            }
        }

        return null;
    }

    // Short names and ids alike are candidates, since either is accepted by a lookup and either may be mistyped.
    private async Task<ValueList<string>> NearNamesAsync(string name, CancellationToken cancellationToken)
    {
        var declared = new List<(string Id, string Name)>();
        if (Connection.Capabilities.Has(Capabilities.Query))
        {
            foreach (var element in (await QueryAsync(Selecting(PropertyName), cancellationToken).ConfigureAwait(false)).Elements)
            {
                declared.Add((element.Id, element.Properties.GetValueOrDefault(PropertyName, string.Empty)));
            }
        }
        else
        {
            await WalkAsync(
                symbol =>
                {
                    declared.Add((symbol.Id, symbol.Name));
                    return false;
                },
                maxDepth: null,
                cancellationToken).ConfigureAwait(false);
        }

        var candidates = new List<string>();
        foreach (var (id, shortName) in declared)
        {
            if (shortName.Length > 0)
            {
                candidates.Add(shortName);
            }

            if (id.Length > 0 && id != shortName)
            {
                candidates.Add(id);
            }
        }

        return NearNames.Closest(name, candidates, MaxSuggestions);
    }
}
