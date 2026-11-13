namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The Category trait every test declares, so a filtered run selects by it and
/// <see cref="TestCategoryTests"/> can prove that no test escapes every filter.
/// </summary>
internal static class Category
{
    /// <summary>The trait name.</summary>
    internal const string Name = "Category";

    /// <summary>Needs nothing but the checkout and the runtime: every test here.</summary>
    internal const string Unit = "Unit";
}
