using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A query over a parsed model, as the SysML v2 API &amp; Services standard's <c>Query</c> resource states one: the
/// elements considered (<see cref="Scope"/>), the properties reported for each match (<see cref="Select"/>), the
/// filter every considered element must satisfy (<see cref="Where"/>) and the properties the matches are sorted by
/// (<see cref="OrderBy"/>). It is immutable; derive one from another with a <c>with</c> expression. Collections are
/// copied when they are set.
/// </summary>
/// <remarks>
/// A query names a property as the service's query property table names it (<c>@type</c>, <c>name</c>,
/// <c>qualifiedName</c>, <c>owner</c>, ...); the service refuses a property it does not know, or a scope element the
/// model does not have, with <see cref="StatusCode.InvalidArgument"/> rather than answering empty.
/// </remarks>
public sealed record Query
{
    private readonly ValueList<string> scope;
    private readonly ValueList<string> select;
    private readonly ValueList<string> orderBy;

    /// <summary>The whole loaded model, every property of each element, unfiltered.</summary>
    public static Query All { get; } = new();

    /// <summary>
    /// The elements the query considers, each by qualified name, together with every element nested inside them;
    /// empty considers the whole loaded model.
    /// </summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Scope
    {
        get => scope;
        init => scope = new ValueList<string>(value);
    }

    /// <summary>The properties to report for each matched element; empty reports all of them.</summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Select
    {
        get => select;
        init => select = new ValueList<string>(value);
    }

    /// <summary>The filter every considered element must satisfy; null matches all of them.</summary>
    public QueryConstraint? Where { get; init; }

    /// <summary>
    /// The properties the matched elements are sorted by, in order of precedence, each ascending, the standard's
    /// <c>orderBy</c>; elements equal under every one keep declaration order. Empty keeps declaration order. A
    /// non-empty list needs <see cref="Capabilities.QueryOrderBy"/>, since a service without it would ignore it.
    /// </summary>
    /// <exception cref="System.ArgumentNullException">The list is null.</exception>
    /// <exception cref="System.ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> OrderBy
    {
        get => orderBy;
        init => orderBy = new ValueList<string>(value);
    }
}
