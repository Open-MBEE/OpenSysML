using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>What one data file set: how many values, on how many elements, and each assignment it made.</summary>
public sealed record ImportedFile
{
    /// <summary>Creates a file's report, copying its assignments.</summary>
    /// <param name="name">The file's name, as the import named it.</param>
    /// <param name="values">How many values it set.</param>
    /// <param name="elements">How many elements they were set on.</param>
    /// <param name="assignments">Each assignment, in data order.</param>
    /// <exception cref="ArgumentNullException"><paramref name="name"/> or <paramref name="assignments"/> is null.</exception>
    /// <exception cref="ArgumentException">An assignment is null.</exception>
    public ImportedFile(string name, long values, long elements, IEnumerable<ImportedValue> assignments)
    {
        ArgumentNullException.ThrowIfNull(name);
        Name = name;
        Values = values;
        Elements = elements;
        Assignments = new ValueList<ImportedValue>(assignments);
    }

    /// <summary>The file's name, as <see cref="DataImport.Name"/> named it.</summary>
    public string Name { get; }

    /// <summary>How many values it set; a redefinition made only to reach a nested inherited feature sets none.</summary>
    public long Values { get; }

    /// <summary>How many elements the values were set on.</summary>
    public long Elements { get; }

    /// <summary>Each assignment, in data order.</summary>
    public ValueList<ImportedValue> Assignments { get; }
}
