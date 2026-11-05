using System;

namespace OpenMBEE.OpenSysML;

/// <summary>The verdict a migration gave one SysML v1 element.</summary>
public sealed record MigrationEntry
{
    /// <summary>Creates an entry.</summary>
    /// <param name="id">The element's <c>xmi:id</c>.</param>
    /// <param name="kind">Its stereotype when one classifies it, else its UML metaclass.</param>
    /// <param name="name">Its qualified name in v1.</param>
    /// <param name="target">The v2 declaration written for it; empty when nothing was.</param>
    /// <param name="verdict"><c>mapped</c>, <c>approximated</c>, <c>unmapped</c> or <c>skipped</c>.</param>
    /// <param name="note">Why it was approximated, left unmapped or skipped.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public MigrationEntry(string id, string kind, string name, string target, string verdict, string note)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(target);
        ArgumentNullException.ThrowIfNull(verdict);
        ArgumentNullException.ThrowIfNull(note);
        Id = id;
        Kind = kind;
        Name = name;
        Target = target;
        Verdict = verdict;
        Note = note;
    }

    /// <summary>The element's <c>xmi:id</c>, the handle a v1 tool addresses it by.</summary>
    public string Id { get; }

    /// <summary>The element as modeled: its stereotype when one classifies it (<c>«Block»</c>, <c>«Requirement»</c>), else its UML metaclass.</summary>
    public string Kind { get; }

    /// <summary>Its qualified name in v1.</summary>
    public string Name { get; }

    /// <summary>The v2 declaration written for it, a qualified name with its keyword; empty when nothing was.</summary>
    public string Target { get; }

    /// <summary><c>mapped</c>, <c>approximated</c>, <c>unmapped</c> or <c>skipped</c>, as the service spells it.</summary>
    public string Verdict { get; }

    /// <summary>Why it was approximated, left unmapped or skipped.</summary>
    public string Note { get; }
}
