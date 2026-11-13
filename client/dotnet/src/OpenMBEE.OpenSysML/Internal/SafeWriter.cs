using System;
using System.Buffers;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Writes a file, and the files a migration refers to beside it, without ever leaving a partial file or touching what
/// it refused. Every destination is judged before anything is written; each file is then written whole to a temporary
/// file in its own directory, flushed to disk, and moved into place, the files a model refers to before the model. A
/// failure or cancellation removes the temporary files, the files it created and the directories it made, and leaves
/// every file that was there before as it was, except one an overwrite already replaced: several files are not
/// published atomically.
/// </summary>
internal static class SafeWriter
{
    private const int MaxLinkHops = 40;

    private static readonly UTF8Encoding Utf8NoBom = new(encoderShouldEmitUTF8Identifier: false);

    private static readonly char[] Separators = [Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar];

    private static readonly SearchValues<char> InvalidFileNameCharacters = SearchValues.Create("<>\":/\\|?*");

    private static readonly HashSet<string> DeviceNames = new(
        ["CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "CONIN$", "CONOUT$", "COM\u00b9", "COM\u00b2", "COM\u00b3", "LPT\u00b9", "LPT\u00b2", "LPT\u00b3"],
        StringComparer.OrdinalIgnoreCase);

    /// <summary>How this platform's default file system compares names: ignoring case on Windows and macOS.</summary>
    internal static StringComparison PathComparison =>
        OperatingSystem.IsWindows() || OperatingSystem.IsMacOS() ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal;

    /// <summary>Text as the bytes written for it: UTF-8 without a byte-order mark, every line ending as given.</summary>
    internal static ReadOnlyMemory<byte> Utf8(string text) => Utf8NoBom.GetBytes(text);

    /// <summary>Writes <paramref name="content"/> at <paramref name="path"/> and each asset at its relative name beside it.</summary>
    /// <param name="path">The file to write.</param>
    /// <param name="content">Its bytes.</param>
    /// <param name="assets">Files to write beside it, each named by a <c>/</c>-separated path relative to its directory.</param>
    /// <param name="overwrite">Whether an existing file is replaced.</param>
    /// <param name="guarded">A file never replaced, whatever <paramref name="overwrite"/> says; null for none.</param>
    /// <param name="beforePublish">Called with each destination just before its file is moved into place; null for none.</param>
    /// <param name="cancellationToken">Cancels the write, which is then undone.</param>
    /// <returns>The full path of the file written.</returns>
    /// <exception cref="UnwritableException">A destination was refused; nothing was written.</exception>
    internal static async Task<string> WriteAsync(
        string path,
        ReadOnlyMemory<byte> content,
        IReadOnlyList<(string Name, ReadOnlyMemory<byte> Content)> assets,
        bool overwrite,
        string? guarded,
        Action<string>? beforePublish,
        CancellationToken cancellationToken)
    {
        var (full, files, directories) = Plan(path, content, assets, overwrite, guarded);
        cancellationToken.ThrowIfCancellationRequested();
        var made = new List<string>();
        var staged = new List<(string Temporary, string Target)>();
        var published = new List<string>();
        var moved = 0;
        try
        {
            foreach (var directory in directories.Where(directory => !Directory.Exists(directory)))
            {
                Directory.CreateDirectory(directory);
                made.Add(directory);
            }

            foreach (var (target, bytes) in files)
            {
                var temporary = Path.Combine(Path.GetDirectoryName(target)!, $".opensysml-{Guid.NewGuid():N}.tmp");
                var options = new FileStreamOptions { Mode = FileMode.CreateNew, Access = FileAccess.Write, Share = FileShare.None, Options = FileOptions.Asynchronous };
                var stream = new FileStream(temporary, options);
                staged.Add((temporary, target));
                await using (stream.ConfigureAwait(false))
                {
                    await stream.WriteAsync(bytes, cancellationToken).ConfigureAwait(false);
                    await stream.FlushAsync(cancellationToken).ConfigureAwait(false);
                    stream.Flush(flushToDisk: true);
                }
            }

            foreach (var (temporary, target) in staged)
            {
                beforePublish?.Invoke(target);
                cancellationToken.ThrowIfCancellationRequested();
                var existed = overwrite && (File.Exists(target) || IsLink(target));
                File.Move(temporary, target, overwrite);
                moved++;
                if (!existed)
                {
                    published.Add(target);
                }
            }

            return full;
        }
        catch
        {
            foreach (var (temporary, _) in staged.Skip(moved))
            {
                TryDelete(temporary);
            }

            foreach (var target in Enumerable.Reverse(published))
            {
                TryDelete(target);
            }

            foreach (var directory in Enumerable.Reverse(made))
            {
                TryDeleteDirectory(directory);
            }

            throw;
        }
    }

    /// <summary>
    /// Where a path lands once every symbolic link or junction on it is followed, existing or dangling: each existing
    /// component resolved in turn, with the part that does not exist yet appended below the last that does.
    /// </summary>
    /// <param name="path">The path, which is made full first.</param>
    /// <returns>The full path it lands on.</returns>
    /// <exception cref="UnwritableException">The path holds a cycle of links.</exception>
    internal static string Landing(string path)
    {
        var hops = 0;
        return Landing(Path.GetFullPath(path), path, ref hops);
    }

    /// <summary>Whether two paths name one place, including Unicode normalization equivalence on macOS.</summary>
    internal static bool Same(string left, string right)
    {
        left = Path.TrimEndingDirectorySeparator(left);
        right = Path.TrimEndingDirectorySeparator(right);
        if (OperatingSystem.IsMacOS())
        {
            left = left.Normalize(NormalizationForm.FormD);
            right = right.Normalize(NormalizationForm.FormD);
        }

        return string.Equals(left, right, PathComparison);
    }

    private static (string Full, List<(string Target, ReadOnlyMemory<byte> Content)> Files, List<string> Directories) Plan(
        string path,
        ReadOnlyMemory<byte> content,
        IReadOnlyList<(string Name, ReadOnlyMemory<byte> Content)> assets,
        bool overwrite,
        string? guarded)
    {
        string full;
        try
        {
            full = Path.GetFullPath(path);
        }
        catch (Exception error) when (error is ArgumentException or NotSupportedException or PathTooLongException)
        {
            throw new UnwritableException($"{path} is not a path this file system can hold: {error.Message}", path);
        }

        if (Path.EndsInDirectorySeparator(path) || Path.GetFileName(full).Length == 0)
        {
            throw new UnwritableException($"{full} names a directory, not a file.", full);
        }

        var directory = Path.GetDirectoryName(full)!;
        if (!Directory.Exists(directory))
        {
            throw new UnwritableException($"{full} cannot be written: its directory {directory} does not exist.", full);
        }

        var source = guarded is null ? null : Landing(guarded);
        Judge(full, full, overwrite, source);
        var files = new List<(string Target, ReadOnlyMemory<byte> Content)>();
        var directories = new List<string>();
        var landings = new List<string> { Landing(full) };
        var parents = new List<string>();
        foreach (var (name, bytes) in assets)
        {
            var segments = Segments(name, directory);
            var target = Path.Combine([directory, .. segments]);
            var parent = directory;
            foreach (var segment in segments.SkipLast(1))
            {
                parent = Path.Combine(parent, segment);
                if (File.Exists(parent) || IsLink(parent))
                {
                    throw new UnwritableException($"The migration's file {name} needs the directory {parent}, which is a file or a symbolic link.", target);
                }

                if (!directories.Any(listed => Same(listed, parent)))
                {
                    directories.Add(parent);
                }

                parents.Add(Landing(parent));
            }

            // No component below the directory is a link and none climbs out, so the file lands below the directory.
            Judge(target, name, overwrite, source);
            var landing = Landing(target);
            if (landings.Any(taken => Same(taken, landing)))
            {
                throw new UnwritableException($"The migration's file {name} would land on {landing}, which another of its files is written to.", target);
            }

            landings.Add(landing);
            files.Add((target, bytes));
        }

        if (landings.FirstOrDefault(file => parents.Any(parent => Same(parent, file))) is { } clash)
        {
            throw new UnwritableException($"{clash} would be written both as a file and as a directory another of the migration's files is in.", clash);
        }

        // The model last, so it appears only once every file it refers to is in place.
        files.Add((full, content));
        return (full, files, directories);
    }

    // Refuses a destination: a link or a directory, the guarded file, or an existing file without an overwrite.
    private static void Judge(string target, string name, bool overwrite, string? guarded)
    {
        if (IsLink(target))
        {
            throw new UnwritableException($"{name} is a symbolic link at {target}; write to the file it links to instead.", target);
        }

        if (Directory.Exists(target))
        {
            throw new UnwritableException($"{name} is the directory {target}, not a file.", target);
        }

        if (guarded is not null && Same(Landing(target), guarded))
        {
            throw new UnwritableException($"{target} is the SysML v1 model being migrated; a migration never replaces it.", target);
        }

        if (!overwrite && File.Exists(target))
        {
            throw new UnwritableException($"{target} exists; set SaveOptions.Overwrite to replace it.", target);
        }
    }

    // The components of a relative name the schema separates with '/', each one a single portable file name.
    private static string[] Segments(string name, string directory)
    {
        var why = name.Length == 0 ? "it is empty"
            : name[0] == '/' ? "it is absolute"
            : null;
        var segments = name.Split('/');
        foreach (var segment in segments)
        {
            why ??= segment switch
            {
                "" or "." or ".." => $"it has the component \"{segment}\"",
                _ when segment.Contains('\\', StringComparison.Ordinal) || segment.Contains(':', StringComparison.Ordinal) =>
                    "it names a drive, a share or a backslash-separated path",
                _ when segment.AsSpan().IndexOfAny(InvalidFileNameCharacters) >= 0 || segment.Any(char.IsControl) => "it holds a character no portable file name may",
                _ when segment.EndsWith('.') || segment.EndsWith(' ') => "a component ends in a dot or a space, which Windows drops",
                _ when DeviceNames.Contains(segment.Split('.')[0].TrimEnd()) => $"\"{segment}\" names a Windows device",
                _ => null,
            };
        }

        return why is null
            ? segments
            : throw new UnwritableException($"The migration's file \"{name}\" is not a relative path below {directory}: {why}.", directory);
    }

    private static string Landing(string full, string asked, ref int hops)
    {
        var (root, parts) = Components(full);
        var current = root;
        for (var i = 0; i < parts.Length; i++)
        {
            var next = Path.Combine(current, parts[i]);
            if (LinkTarget(next) is { } link)
            {
                if (++hops > MaxLinkHops)
                {
                    throw new UnwritableException($"{asked} cannot be resolved: too many levels of symbolic links.", full);
                }

                current = Landing(Path.GetFullPath(link, current), asked, ref hops);
                continue;
            }

            if (!File.Exists(next) && !Directory.Exists(next))
            {
                return Path.Combine([current, .. parts[i..]]);
            }

            current = next;
        }

        return current;
    }

    private static (string Root, string[] Parts) Components(string path)
    {
        var full = Path.GetFullPath(path);
        var root = Path.GetPathRoot(full) ?? string.Empty;
        return (root, full[root.Length..].Split(Separators, StringSplitOptions.RemoveEmptyEntries));
    }

    private static bool IsLink(string path) => LinkTarget(path) is not null;

    private static string? LinkTarget(string path)
    {
        try
        {
            return new FileInfo(path).LinkTarget;
        }
        catch (IOException)
        {
            return null;
        }
        catch (UnauthorizedAccessException)
        {
            return null;
        }
    }

    private static void TryDelete(string path)
    {
        try
        {
            File.Delete(path);
        }
        catch (IOException)
        {
        }
        catch (UnauthorizedAccessException)
        {
        }
    }

    private static void TryDeleteDirectory(string path)
    {
        try
        {
            Directory.Delete(path, recursive: false);
        }
        catch (IOException)
        {
        }
        catch (UnauthorizedAccessException)
        {
        }
    }
}
