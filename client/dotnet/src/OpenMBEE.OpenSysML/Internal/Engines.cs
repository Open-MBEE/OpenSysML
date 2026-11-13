namespace OpenMBEE.OpenSysML.Internal;

/// <summary>How the service reads an option naming an engine.</summary>
internal static class Engines
{
    /// <summary>Whether an engine's name selects the explore engine, which the service reads with surrounding white space trimmed.</summary>
    internal static bool Explores(string? engine) => engine is { } name && name.Trim() == RunOptions.Explore;

    /// <summary>
    /// Requires the capabilities naming an engine needs, in the order the service checks them: <c>engines</c>, then
    /// <c>schedule_explore</c> for the explore engine. Naming none needs neither.
    /// </summary>
    internal static void Require(string? engine, Capabilities capabilities)
    {
        if (engine is null)
        {
            return;
        }

        capabilities.Require(Capabilities.Engines);
        if (Explores(engine))
        {
            capabilities.Require(Capabilities.ScheduleExplore);
        }
    }
}
