namespace OpenMBEE.OpenSysML.Tests;

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

    /// <summary>Needs a sysml-grpc binary, see <see cref="ServiceGate"/>, or buf, see <see cref="BufGate"/>.</summary>
    internal const string Integration = "Integration";

    /// <summary>Needs the Python, Java and Rust toolchains, whose clients' cache lock it is run against; see <see cref="InteropGate"/>.</summary>
    internal const string Interop = "Interop";
}
