using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The file-writing contract of saving a conversion and writing a migration: nothing is written over an existing file
/// unless asked, a refused write changes nothing on disk, a failed or cancelled one leaves no partial or temporary file
/// and removes what it created, and the files a migration refers to are written only below the model's directory —
/// never through <c>..</c>, a drive, a share, a symbolic link or onto one another, the v1 model or a case variant of it.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SafeWriteTests : IDisposable
{
    private static readonly byte[] Old = "old\n"u8.ToArray();

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-write-").FullName;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Fact]
    public async Task ASaveWritesTheContentWholeAndRefusesAnExistingFileUnlessOverwriting()
    {
        var path = Path.Combine(directory, "model.sysml");
        var conversion = Converted("package P {\r\n    // \u00e9\r\n}\n");

        var written = await conversion.SaveAsync(path, cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<UnwritableException>(() => Converted("other").SaveAsync(path, cancellationToken: Token));
        var kept = await File.ReadAllBytesAsync(path, Token);
        await Converted("replaced").SaveAsync(path, new SaveOptions { Overwrite = true }, Token);

        Assert.Equal(Path.GetFullPath(path), written);
        Assert.Equal("package P {\r\n    // \u00e9\r\n}\n"u8.ToArray(), kept);
        Assert.Equal((path, true), (refused.Path, refused.Message.Contains("SaveOptions.Overwrite", StringComparison.Ordinal)));
        Assert.Equal("replaced", await File.ReadAllTextAsync(path, Token));
        Assert.Equal(["model.sysml"], Tree());
    }

    [Fact]
    public async Task ASaveRefusesADirectoryAMissingDirectoryAndANameEndingInASeparator()
    {
        Directory.CreateDirectory(Path.Combine(directory, "taken"));
        var conversion = Converted("c");
        var overwrite = new SaveOptions { Overwrite = true };

        var folder = await Assert.ThrowsAsync<UnwritableException>(() => conversion.SaveAsync(Path.Combine(directory, "taken"), overwrite, Token));
        var missing = await Assert.ThrowsAsync<UnwritableException>(() => conversion.SaveAsync(Path.Combine(directory, "no", "model.sysml"), overwrite, Token));
        var separator = await Assert.ThrowsAsync<UnwritableException>(() => conversion.SaveAsync(Path.Combine(directory, "taken") + Path.DirectorySeparatorChar, overwrite, Token));

        Assert.Contains("not a file", folder.Message, StringComparison.Ordinal);
        Assert.Contains("does not exist", missing.Message, StringComparison.Ordinal);
        Assert.Contains("names a directory", separator.Message, StringComparison.Ordinal);
        var unholdable = await Assert.ThrowsAsync<UnwritableException>(() => conversion.SaveAsync(Path.Combine(directory, "a\0b.sysml"), overwrite, Token));
        Assert.Contains("not a path this file system can hold", unholdable.Message, StringComparison.Ordinal);
        Assert.Equal(["taken/"], Tree());
    }

    [Fact]
    public async Task ASaveIsRefusedAtASymbolicLinkAndTheFileItLinksToIsLeftAlone()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var target = Path.Combine(directory, "target.sysml");
        await File.WriteAllBytesAsync(target, Old, Token);
        var link = Path.Combine(directory, "link.sysml");
        File.CreateSymbolicLink(link, target);
        File.CreateSymbolicLink(Path.Combine(directory, "dangling.sysml"), Path.Combine(directory, "nowhere.sysml"));

        var linked = await Assert.ThrowsAsync<UnwritableException>(() => Converted("c").SaveAsync(link, new SaveOptions { Overwrite = true }, Token));
        var dangling = await Assert.ThrowsAsync<UnwritableException>(() => Converted("c").SaveAsync(Path.Combine(directory, "dangling.sysml"), cancellationToken: Token));

        Assert.Contains("symbolic link", linked.Message, StringComparison.Ordinal);
        Assert.Contains("symbolic link", dangling.Message, StringComparison.Ordinal);
        Assert.Equal(Old, await File.ReadAllBytesAsync(target, Token));
        Assert.False(File.Exists(Path.Combine(directory, "nowhere.sysml")));
    }

    [Fact]
    public async Task ACancelledOrFailedSaveLeavesNoPartialFileAndTheExistingOneAsItWas()
    {
        var path = Path.Combine(directory, "model.sysml");
        await File.WriteAllBytesAsync(path, Old, Token);
        var fresh = Path.Combine(directory, "fresh.sysml");
        using var cancel = new CancellationTokenSource();
        var conversion = Converted(new string('x', 1 << 20));

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            conversion.PublishAsync(path, new SaveOptions { Overwrite = true }, _ => cancel.Cancel(), cancel.Token));
        await Assert.ThrowsAsync<IOException>(() => conversion.PublishAsync(path, new SaveOptions { Overwrite = true }, _ => throw new IOException("disk full"), Token));
        await Assert.ThrowsAsync<IOException>(() => conversion.PublishAsync(fresh, null, _ => throw new IOException("disk full"), Token));
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => conversion.SaveAsync(fresh, cancellationToken: new CancellationToken(canceled: true)));

        Assert.Equal(Old, await File.ReadAllBytesAsync(path, Token));
        Assert.Equal(["model.sysml"], Tree());
    }

    [Fact]
    public async Task AFileCreatedAtTheDestinationWhileTheWriteIsUnderWayIsNotReplaced()
    {
        var path = Path.Combine(directory, "model.sysml");

        await Assert.ThrowsAsync<IOException>(() => Converted("mine").PublishAsync(path, null, target => File.WriteAllBytes(target, Old), Token));

        Assert.Equal(Old, await File.ReadAllBytesAsync(path, Token));
        Assert.Equal(["model.sysml"], Tree());
    }

    [Fact]
    public async Task AMigrationWritesItsFilesBesideTheModelCreatingTheirDirectories()
    {
        var path = Path.Combine(directory, "Vehicle.sysml");
        var migration = Migrated(null, ("images/deep/a.png", [1, 2]), ("b.png", []), ("images/c d.png", [3]));

        var written = await migration.WriteAsync(path, cancellationToken: Token);
        var again = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("b.png", [9])).WriteAsync(Path.Combine(directory, "Other.sysml"), cancellationToken: Token));
        await Migrated(null, ("b.png", [9])).WriteAsync(path, new SaveOptions { Overwrite = true }, Token);

        Assert.Equal(Path.GetFullPath(path), written);
        Assert.Equal(["Vehicle.sysml", "b.png", "images/", "images/c d.png", "images/deep/", "images/deep/a.png"], Tree());
        Assert.Equal([1, 2], await File.ReadAllBytesAsync(Path.Combine(directory, "images", "deep", "a.png"), Token));
        Assert.Equal([9], await File.ReadAllBytesAsync(Path.Combine(directory, "b.png"), Token));
        Assert.Contains("b.png", again.Message, StringComparison.Ordinal);
        Assert.Equal("package Vehicle;\n", await File.ReadAllTextAsync(path, Token));
    }

    [Fact]
    public async Task AMigrationNeverReplacesItsV1ModelEvenWhenOverwriting()
    {
        var source = Path.Combine(directory, "Vehicle.xmi");
        await File.WriteAllBytesAsync(source, Old, Token);
        var overwrite = new SaveOptions { Overwrite = true };

        var itself = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source).WriteAsync(source, overwrite, Token));
        var file = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source, ("Vehicle.xmi", [1])).WriteAsync(Path.Combine(directory, "Vehicle.sysml"), overwrite, Token));
        if (OperatingSystem.IsWindows() || OperatingSystem.IsMacOS())
        {
            await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source).WriteAsync(Path.Combine(directory, "VEHICLE.XMI"), overwrite, Token));
            await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source, ("vehicle.XMI", [1])).WriteAsync(Path.Combine(directory, "Vehicle.sysml"), overwrite, Token));
        }

        Assert.Contains("never replaces it", itself.Message, StringComparison.Ordinal);
        Assert.Contains("never replaces it", file.Message, StringComparison.Ordinal);
        Assert.Equal(Old, await File.ReadAllBytesAsync(source, Token));
        Assert.Equal(["Vehicle.xmi"], Tree());
    }

    [Fact]
    public async Task UnicodeEquivalentSourceNamesAreGuardedOnMacOSAndDistinctElsewhere()
    {
        var source = Path.Combine(directory, "caf\u00e9.xmi");
        var alias = Path.Combine(directory, "cafe\u0301.xmi");
        var model = Path.Combine(directory, "Vehicle.sysml");
        await File.WriteAllBytesAsync(source, Old, Token);
        var overwrite = new SaveOptions { Overwrite = true };

        Assert.Equal(OperatingSystem.IsMacOS(), Internal.SafeWriter.Same(source, alias));
        if (OperatingSystem.IsMacOS())
        {
            await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source).WriteAsync(alias, overwrite, Token));
            await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source, ("cafe\u0301.xmi", [1])).WriteAsync(model, overwrite, Token));
            Assert.Single(Tree());
        }
        else
        {
            await Migrated(source).WriteAsync(alias, overwrite, Token);
            await Migrated(source, ("cafe\u0301.xmi", [1])).WriteAsync(model, overwrite, Token);
            Assert.Equal([1], await File.ReadAllBytesAsync(alias, Token));
            Assert.Equal(3, Tree().Count);
        }

        Assert.Equal(Old, await File.ReadAllBytesAsync(source, Token));
    }

    [Theory]
    [InlineData("Vehicle.sysml", "caf\u00e9.png", "cafe\u0301.png")]
    [InlineData("caf\u00e9.sysml", "cafe\u0301.sysml", null)]
    [InlineData("Vehicle.sysml", "caf\u00e9", "cafe\u0301/a.png")]
    public async Task UnicodeEquivalentDestinationsAreRefusedOnMacOSAndDistinctElsewhere(string model, string first, string? second)
    {
        var files = new List<(string Name, byte[] Content)> { (first, [1]) };
        if (second is not null)
        {
            files.Add((second, [2]));
        }

        var migration = Migrated(null, [.. files]);
        var path = Path.Combine(directory, model);
        var overwrite = new SaveOptions { Overwrite = true };
        if (OperatingSystem.IsMacOS())
        {
            await Assert.ThrowsAsync<UnwritableException>(() => migration.WriteAsync(path, overwrite, Token));
            Assert.Empty(Tree());
        }
        else
        {
            await migration.WriteAsync(path, overwrite, Token);
            Assert.Equal(migration.Content, await File.ReadAllTextAsync(path, Token));
            foreach (var (name, bytes) in files)
            {
                Assert.Equal(bytes, await File.ReadAllBytesAsync(Path.Combine(directory, name), Token));
            }
        }
    }

    [Fact]
    public async Task AMigrationNeverReplacesItsV1ModelReachedThroughALink()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var models = Directory.CreateDirectory(Path.Combine(directory, "models")).FullName;
        var source = Path.Combine(models, "Vehicle.xmi");
        await File.WriteAllBytesAsync(source, Old, Token);
        var alias = Path.Combine(directory, "alias");
        Directory.CreateSymbolicLink(alias, models);

        var loop = Path.Combine(directory, "loop");
        File.CreateSymbolicLink(loop, Path.Combine(directory, "pool"));
        File.CreateSymbolicLink(Path.Combine(directory, "pool"), loop);

        await Assert.ThrowsAsync<UnwritableException>(() => Migrated(source).WriteAsync(Path.Combine(alias, "Vehicle.xmi"), new SaveOptions { Overwrite = true }, Token));
        var cycle = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(loop).WriteAsync(Path.Combine(directory, "Vehicle.sysml"), cancellationToken: Token));

        Assert.Equal(Old, await File.ReadAllBytesAsync(source, Token));
        Assert.Contains("too many levels of symbolic links", cycle.Message, StringComparison.Ordinal);
        Assert.False(File.Exists(Path.Combine(directory, "Vehicle.sysml")));
    }

    [Theory]
    [InlineData("")]
    [InlineData("/etc/passwd")]
    [InlineData("../escape.png")]
    [InlineData("images/../../escape.png")]
    [InlineData("images/./a.png")]
    [InlineData("images//a.png")]
    [InlineData("images/")]
    [InlineData("C:/Windows/a.png")]
    [InlineData("C:a.png")]
    [InlineData(@"\\server\share\a.png")]
    [InlineData("//server/share/a.png")]
    [InlineData(@"images\a.png")]
    [InlineData("a.png.")]
    [InlineData("a.png ")]
    [InlineData("con.png")]
    [InlineData("images/LPT1")]
    [InlineData("CONOUT$")]
    [InlineData("com\u00b9.png")]
    [InlineData("a\u0001.png")]
    [InlineData("a\0.png")]
    [InlineData("a?.png")]
    [InlineData("images/a*.png")]
    [InlineData("a|b.png")]
    [InlineData("a\"b.png")]
    [InlineData("a<b.png")]
    [InlineData("a>b.png")]
    public async Task AMigrationFileThatIsNotAPortableRelativePathIsRefusedAndNothingIsWritten(string name)
    {
        var refused = await Assert.ThrowsAsync<UnwritableException>(() =>
            Migrated(null, ("fine.png", [1]), (name, [2])).WriteAsync(Path.Combine(directory, "Vehicle.sysml"), new SaveOptions { Overwrite = true }, Token));

        Assert.Contains("not a relative path below", refused.Message, StringComparison.Ordinal);
        Assert.Empty(Tree());
    }

    [Fact]
    public async Task FilesThatWouldLandOnOneAnotherOrOnTheModelAreRefused()
    {
        var path = Path.Combine(directory, "Vehicle.sysml");
        var overwrite = new SaveOptions { Overwrite = true };

        var twice = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("a.png", [1]), ("a.png", [2])).WriteAsync(path, overwrite, Token));
        var model = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("Vehicle.sysml", [1])).WriteAsync(path, overwrite, Token));
        var clash = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("a", [1]), ("a/b.png", [2])).WriteAsync(path, overwrite, Token));
        var under = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("Vehicle.sysml/b.png", [2])).WriteAsync(path, overwrite, Token));
        if (OperatingSystem.IsWindows() || OperatingSystem.IsMacOS())
        {
            await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("img/a.png", [1]), ("IMG/A.png", [2])).WriteAsync(path, overwrite, Token));
        }

        Assert.Contains("another of its files", twice.Message, StringComparison.Ordinal);
        Assert.Contains("another of its files", model.Message, StringComparison.Ordinal);
        Assert.Contains("both as a file and as a directory", clash.Message, StringComparison.Ordinal);
        Assert.Contains("both as a file and as a directory", under.Message, StringComparison.Ordinal);
        Assert.Empty(Tree());
    }

    [Fact]
    public async Task AMigrationFileIsNeverWrittenThroughASymbolicLink()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        var model = Directory.CreateDirectory(Path.Combine(directory, "model")).FullName;
        var outside = Directory.CreateDirectory(Path.Combine(directory, "outside")).FullName;
        var inside = Directory.CreateDirectory(Path.Combine(model, "real")).FullName;
        Directory.CreateSymbolicLink(Path.Combine(model, "images"), outside);
        Directory.CreateSymbolicLink(Path.Combine(model, "alias"), inside);
        File.CreateSymbolicLink(Path.Combine(model, "a.png"), Path.Combine(outside, "a.png"));
        var path = Path.Combine(model, "Vehicle.sysml");
        var overwrite = new SaveOptions { Overwrite = true };
        var before = Tree();

        var escape = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("images/x.png", [1])).WriteAsync(path, overwrite, Token));
        var within = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("alias/x.png", [1])).WriteAsync(path, overwrite, Token));
        var linked = await Assert.ThrowsAsync<UnwritableException>(() => Migrated(null, ("a.png", [1])).WriteAsync(path, overwrite, Token));

        Assert.Contains("a file or a symbolic link", escape.Message, StringComparison.Ordinal);
        Assert.Contains("a file or a symbolic link", within.Message, StringComparison.Ordinal);
        Assert.Contains("symbolic link", linked.Message, StringComparison.Ordinal);
        Assert.Equal(before, Tree());
        Assert.Empty(Directory.GetFileSystemEntries(outside));
    }

    [Fact]
    public async Task AFailedMigrationWriteRemovesWhatItCreatedAndKeepsWhatWasThere()
    {
        var path = Path.Combine(directory, "Vehicle.sysml");
        var kept = Path.Combine(directory, "kept.png");
        await File.WriteAllBytesAsync(kept, Old, Token);
        await File.WriteAllBytesAsync(path, Old, Token);
        var migration = Migrated(null, ("new/deep/a.png", [1]), ("kept.png", [2]));
        using var cancel = new CancellationTokenSource();

        await Assert.ThrowsAsync<IOException>(() =>
            migration.PublishAsync(path, new SaveOptions { Overwrite = true }, target => { if (target == path) { throw new IOException("disk full"); } }, Token));
        var treeAfterFailure = Tree();
        var keptAfterFailure = await File.ReadAllBytesAsync(kept, Token);
        var modelAfterFailure = await File.ReadAllBytesAsync(path, Token);
        await File.WriteAllBytesAsync(kept, Old, Token);
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            migration.PublishAsync(path, new SaveOptions { Overwrite = true }, target => { if (target == path) { cancel.Cancel(); } }, cancel.Token));
        var notOverwriting = await Assert.ThrowsAsync<UnwritableException>(() => migration.WriteAsync(path, cancellationToken: Token));

        // Several files are not published atomically: the one an overwrite replaced stays replaced.
        Assert.Equal(["Vehicle.sysml", "kept.png"], treeAfterFailure);
        Assert.Equal([2], keptAfterFailure);
        Assert.Equal(Old, modelAfterFailure);
        Assert.Equal(["Vehicle.sysml", "kept.png"], Tree());
        Assert.Equal(Old, await File.ReadAllBytesAsync(path, Token));
        Assert.Contains("exists", notOverwriting.Message, StringComparison.Ordinal);
    }

    private static Conversion Converted(string content) => new(content, "sysml", "sysml", [], false, string.Empty, []);

    private static Migration Migrated(string? source, params (string Name, byte[] Content)[] files) =>
        new(
            "package Vehicle;\n",
            "xmi",
            "sysml",
            new MigrationReport("<content>", string.Empty, "migrated", 0, 0, 0, 0, [], string.Empty),
            string.Empty,
            files.Select(file => new MigrationFile(file.Name, file.Content)),
            true,
            "experimental",
            source);

    // Every entry below the directory, '/'-separated with directories marked, in ordinal order; a temporary file shows.
    private List<string> Tree() =>
        [.. Directory.EnumerateFileSystemEntries(directory, "*", new EnumerationOptions { RecurseSubdirectories = true, AttributesToSkip = 0 })
            .Select(entry => Path.GetRelativePath(directory, entry).Replace(Path.DirectorySeparatorChar, '/') + (Directory.Exists(entry) && new FileInfo(entry).LinkTarget is null ? "/" : string.Empty))
            .Order(StringComparer.Ordinal)];
}
