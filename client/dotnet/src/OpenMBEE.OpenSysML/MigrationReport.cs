using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a migration did with every element of the SysML v1 model: mapped to a v2 counterpart that states the same
/// thing, approximated by a v2 form that loses or restates part of it, left unmapped and recorded as a comment where it
/// stood, or skipped as a profile, library or notation-only element nothing refers to. The summary and the counts come
/// back with every migration; <see cref="Entries"/> and <see cref="Text"/> when <see cref="MigrateOptions.Report"/> asks.
/// </summary>
public sealed record MigrationReport
{
    /// <summary>Creates a report, copying its entries.</summary>
    /// <param name="source">The v1 document migrated.</param>
    /// <param name="exporter">The tool that exported it, when it said.</param>
    /// <param name="summary">The one-line account <c>sysml -migrate</c> prints.</param>
    /// <param name="mapped">Elements mapped.</param>
    /// <param name="approximated">Elements approximated.</param>
    /// <param name="unmapped">Elements left unmapped.</param>
    /// <param name="skipped">Elements skipped.</param>
    /// <param name="entries">Every element's verdict, when asked for.</param>
    /// <param name="text">The report as <c>-migration-report</c> writes it, when asked for.</param>
    /// <exception cref="ArgumentNullException">A string or <paramref name="entries"/> is null.</exception>
    /// <exception cref="ArgumentException">An entry is null.</exception>
    public MigrationReport(
        string source,
        string exporter,
        string summary,
        int mapped,
        int approximated,
        int unmapped,
        int skipped,
        IEnumerable<MigrationEntry> entries,
        string text)
    {
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(exporter);
        ArgumentNullException.ThrowIfNull(summary);
        ArgumentNullException.ThrowIfNull(text);
        Source = source;
        Exporter = exporter;
        Summary = summary;
        Mapped = mapped;
        Approximated = approximated;
        Unmapped = unmapped;
        Skipped = skipped;
        Entries = new ValueList<MigrationEntry>(entries);
        Text = text;
    }

    /// <summary>The v1 document migrated, as the service named it.</summary>
    public string Source { get; }

    /// <summary>The tool that exported it, when its XMI said; empty otherwise.</summary>
    public string Exporter { get; }

    /// <summary>The one-line account <c>sysml -migrate</c> prints after migrating.</summary>
    public string Summary { get; }

    /// <summary>Elements with a v2 counterpart that states the same thing.</summary>
    public int Mapped { get; }

    /// <summary>Elements written in a v2 form that loses or restates part of what v1 said.</summary>
    public int Approximated { get; }

    /// <summary>Elements with no v2 form, recorded as a comment where they stood.</summary>
    public int Unmapped { get; }

    /// <summary>Profile, library and notation-only elements nothing in the model refers to.</summary>
    public int Skipped { get; }

    /// <summary>Every element accounted for: the four counts together.</summary>
    public long Total => (long)Mapped + Approximated + Unmapped + Skipped;

    /// <summary>Every element's verdict, in the service's order, when <see cref="MigrateOptions.Report"/> asked; empty otherwise, the counts standing in.</summary>
    public ValueList<MigrationEntry> Entries { get; }

    /// <summary>
    /// The report as <c>-migration-report</c> writes it to a text file, when <see cref="MigrateOptions.Report"/> asked:
    /// the entries grouped by verdict, those needing attention first; empty otherwise.
    /// </summary>
    public string Text { get; }

    /// <summary>The entries of one verdict, in the service's order.</summary>
    /// <param name="verdict"><c>mapped</c>, <c>approximated</c>, <c>unmapped</c> or <c>skipped</c>.</param>
    /// <returns>The entries; empty when <see cref="Entries"/> is, or none has that verdict.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="verdict"/> is null.</exception>
    public ValueList<MigrationEntry> ByVerdict(string verdict)
    {
        ArgumentNullException.ThrowIfNull(verdict);
        return new ValueList<MigrationEntry>(Entries.Where(entry => entry.Verdict == verdict));
    }
}
