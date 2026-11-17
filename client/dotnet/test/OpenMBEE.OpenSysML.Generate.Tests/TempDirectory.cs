using System;
using System.IO;
using System.Text;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>A directory of the test's own, removed with everything in it when the test ends.</summary>
internal sealed class TempDirectory : IDisposable
{
    internal TempDirectory() => Path = Directory.CreateTempSubdirectory("opensysml-generate-").FullName;

    /// <summary>The directory.</summary>
    internal string Path { get; }

    /// <summary>Writes a file under the directory, creating the directories it names.</summary>
    internal string Write(string name, string text) => Write(name, Encoding.UTF8.GetBytes(text));

    /// <summary>Writes a file under the directory, creating the directories it names.</summary>
    internal string Write(string name, byte[] content)
    {
        var path = System.IO.Path.Combine(Path, name);
        Directory.CreateDirectory(System.IO.Path.GetDirectoryName(path)!);
        File.WriteAllBytes(path, content);
        return path;
    }

    public void Dispose() => Directory.Delete(Path, recursive: true);
}
