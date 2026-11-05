using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>One symbol of a model and the symbols it owns, in the order the service lists them.</summary>
/// <param name="Symbol">The symbol.</param>
/// <param name="Children">The symbols it owns.</param>
internal sealed record SymbolNode(Symbol Symbol, IReadOnlyList<SymbolNode> Children)
{
    /// <summary>Reads a model's symbol trees from its roots, one per document.</summary>
    /// <exception cref="GenerationException">The service lists a child it then reports no symbol for.</exception>
    internal static async Task<IReadOnlyList<SymbolNode>> ReadAsync(Model model, CancellationToken cancellationToken)
    {
        var visited = new HashSet<string>(StringComparer.Ordinal);
        var missing = new List<string>();
        var roots = new List<SymbolNode>();
        foreach (var root in model.Roots)
        {
            roots.Add(await ReadAsync(model, root, visited, missing, cancellationToken).ConfigureAwait(false));
        }

        return missing.Count == 0 ? roots : throw new GenerationException(missing);
    }

    private static async Task<SymbolNode> ReadAsync(Model model, Symbol symbol, HashSet<string> visited, List<string> missing, CancellationToken cancellationToken)
    {
        var children = new List<SymbolNode>();
        foreach (var childId in symbol.ChildIds)
        {
            var child = await model.FindSymbolAsync(childId, cancellationToken).ConfigureAwait(false);

            // An unnamed member is listed under its owner's name and "::", which names nothing the service can look up.
            if (child is null && childId.EndsWith("::", StringComparison.Ordinal))
            {
                continue;
            }

            if (child is null)
            {
                missing.Add($"the service lists {childId} as owned by {Describe(symbol)} but reports no symbol for it");
                continue;
            }

            // A name the service resolves to another element is an alias or import of it, not a member this symbol owns.
            if (child.Id != childId || !visited.Add(child.Id))
            {
                continue;
            }

            children.Add(await ReadAsync(model, child, visited, missing, cancellationToken).ConfigureAwait(false));
        }

        return new SymbolNode(symbol, children);
    }

    private static string Describe(Symbol symbol) => symbol.Id.Length == 0 ? $"the {symbol.Kind}" : symbol.Id;

    /// <summary>This node and every node beneath it, depth first, in order.</summary>
    internal IEnumerable<SymbolNode> DescendantsAndSelf() =>
        Children.SelectMany(child => child.DescendantsAndSelf()).Prepend(this);
}
