namespace OpenMBEE.OpenSysML;

/// <summary>How serious a <see cref="Diagnostic"/> is.</summary>
public enum DiagnosticSeverity
{
    /// <summary>A severity this release of the client does not know; <see cref="Diagnostic.SeverityName"/> keeps it.</summary>
    Unknown,

    /// <summary>The model is wrong: parsing or validation failed.</summary>
    Error,

    /// <summary>The model is questionable but was accepted.</summary>
    Warning,

    /// <summary>Informational only.</summary>
    Info,
}
