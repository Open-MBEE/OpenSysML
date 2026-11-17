namespace OpenMBEE.OpenSysML;

/// <summary>What <see cref="Connection.ListEnginesAsync"/> does beside listing the engines.</summary>
public sealed class ListEnginesOptions
{
    /// <summary>Listing alone, starting no process.</summary>
    public static ListEnginesOptions Default { get; } = new();

    /// <summary>
    /// Whether the service starts each external engine's process once and checks its handshake against its manifest,
    /// reporting the outcome as the engine's readiness (<c>sysml -engines -probe</c>); without it no process is started.
    /// </summary>
    public bool Probe { get; init; }
}
