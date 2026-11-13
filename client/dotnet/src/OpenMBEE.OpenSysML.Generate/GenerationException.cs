using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>
/// The model cannot be generated soundly: every problem found, each naming the SysML elements behind it, in
/// ordinal order so the report is the same on every run.
/// </summary>
internal sealed class GenerationException : Exception
{
    internal GenerationException(IEnumerable<string> problems)
        : this([.. problems.Distinct(StringComparer.Ordinal).Order(StringComparer.Ordinal)])
    {
    }

    private GenerationException(IReadOnlyList<string> problems)
        : base(string.Join(Environment.NewLine, problems)) => Problems = problems;

    /// <summary>Every problem found.</summary>
    internal IReadOnlyList<string> Problems { get; }
}
