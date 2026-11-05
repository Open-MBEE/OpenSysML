using System;
using System.IO;
using System.Text;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// The fixtures directory of a suite, conformance/fixtures: the models scenarios parse and the
/// sources they carry inline, named relative to it. A name reaching outside it is refused.
/// </summary>
internal sealed class FixtureDirectory
{
    private readonly string root;

    /// <summary>Opens a fixtures directory.</summary>
    /// <param name="path">The directory; it is read only when a fixture is.</param>
    internal FixtureDirectory(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        root = Path.TrimEndingDirectorySeparator(Path.GetFullPath(path));
    }

    /// <summary>A fixture's source, decoded as UTF-8.</summary>
    /// <param name="name">The fixture's name, relative to the directory.</param>
    /// <returns>Its text.</returns>
    /// <exception cref="InvalidDataException">The name reaches outside the directory, or the fixture cannot be read.</exception>
    internal string Text(string name) => Encoding.UTF8.GetString(Bytes(name));

    /// <summary>A fixture's bytes.</summary>
    /// <param name="name">The fixture's name, relative to the directory.</param>
    /// <returns>Its content.</returns>
    /// <exception cref="InvalidDataException">The name reaches outside the directory, or the fixture cannot be read.</exception>
    internal byte[] Bytes(string name)
    {
        ArgumentNullException.ThrowIfNull(name);
        try
        {
            var path = Path.GetFullPath(Path.Combine(root, name));
            if (!path.StartsWith(root + Path.DirectorySeparatorChar, StringComparison.Ordinal))
            {
                throw new InvalidDataException($"fixture {Spelling.Quote(name)} is outside {root}");
            }

            return File.ReadAllBytes(path);
        }
        catch (Exception e) when (e is IOException or UnauthorizedAccessException or ArgumentException or NotSupportedException)
        {
            throw new InvalidDataException($"reading fixture {Spelling.Quote(name)}: {e.Message}", e);
        }
    }
}
