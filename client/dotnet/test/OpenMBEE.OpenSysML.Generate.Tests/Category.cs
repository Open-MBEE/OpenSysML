namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// The Category trait every test declares, so a filtered run selects by it and
/// <see cref="TestCategoryTests"/> can prove that no test escapes every filter.
/// </summary>
internal static class Category
{
    /// <summary>The trait name.</summary>
    internal const string Name = "Category";

    /// <summary>Needs nothing but the checkout and the runtime.</summary>
    internal const string Unit = "Unit";

    /// <summary>Needs a sysml-grpc binary; see ServiceGate.</summary>
    internal const string Integration = "Integration";
}
