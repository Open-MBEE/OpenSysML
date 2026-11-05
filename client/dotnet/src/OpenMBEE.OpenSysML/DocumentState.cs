using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One row a <c>States</c> document query answered: an active leaf state of the state machine an
/// object exhibits.
/// </summary>
public sealed record DocumentState
{
    /// <summary>Creates a state row, copying its enclosing states.</summary>
    /// <param name="subject">The object in the state; null when the service reported none.</param>
    /// <param name="machine">The state machine, by the name the object exhibits it under, else its declared name.</param>
    /// <param name="name">The leaf state's name.</param>
    /// <param name="statePath">The leaf state's path in its machine, composite states first (<c>on.dim</c>).</param>
    /// <param name="state">The leaf state's declaration; null when the service reported none.</param>
    /// <param name="region">The orthogonal region the leaf is declared in; empty outside one.</param>
    /// <param name="enclosing">The composite states enclosing the leaf, outermost first.</param>
    /// <exception cref="ArgumentNullException">A string or the list is null.</exception>
    /// <exception cref="ArgumentException">An enclosing state is null.</exception>
    public DocumentState(
        DocumentObject? subject,
        string machine,
        string name,
        string statePath,
        DocumentValue.ElementValue? state,
        string region,
        IEnumerable<string> enclosing)
    {
        ArgumentNullException.ThrowIfNull(machine);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(statePath);
        ArgumentNullException.ThrowIfNull(region);
        Subject = subject;
        Machine = machine;
        Name = name;
        StatePath = statePath;
        State = state;
        Region = region;
        Enclosing = new ValueList<string>(enclosing);
    }

    /// <summary>The object in the state, with the path the session reaches it by; null when the service reported none.</summary>
    public DocumentObject? Subject { get; }

    /// <summary>The state machine, by the name the object exhibits it under (<c>lp</c>), else its declared name.</summary>
    public string Machine { get; }

    /// <summary>The leaf state's name.</summary>
    public string Name { get; }

    /// <summary>The leaf state's path in its machine, composite states first (<c>on.dim</c>).</summary>
    public string StatePath { get; }

    /// <summary>
    /// The leaf state's declaration, with no <see cref="DocumentValue.ElementValue.Id"/> when the machine
    /// declares no element for it; null when the service reported none.
    /// </summary>
    public DocumentValue.ElementValue? State { get; }

    /// <summary>The orthogonal region the leaf is declared in; empty outside one.</summary>
    public string Region { get; }

    /// <summary>The composite states enclosing the leaf, outermost first; each is active.</summary>
    public ValueList<string> Enclosing { get; }

    /// <summary>The row as <c>Garage::lamp.lp in on.dim</c>.</summary>
    /// <returns>The row's text.</returns>
    public override string ToString() => $"{Subject?.ToString() ?? "(no object)"}.{Machine} in {StatePath}";
}
