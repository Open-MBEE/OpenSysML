using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>
/// The generator's command: reads the model's documents, parses them through a service that reports static
/// type facts, and writes, or checks, the generated file. Exits 0 on success, 1 when the model cannot be
/// generated or the checked file is out of date, and 2 when the command was used wrongly.
/// </summary>
internal static class GenerateCommand
{
    /// <summary>Success.</summary>
    internal const int Succeeded = 0;

    /// <summary>The model could not be generated, or the checked file is not what it would generate.</summary>
    internal const int Failed = 1;

    /// <summary>The command was used wrongly, or an input cannot be read.</summary>
    internal const int Misused = 2;

    private static readonly UTF8Encoding Utf8 = new(encoderShouldEmitUTF8Identifier: false);

    /// <summary>Runs the command.</summary>
    /// <param name="arguments">The command's arguments.</param>
    /// <param name="output">Where the generated file goes without <c>-o</c>, and <c>--help</c> its usage.</param>
    /// <param name="error">Where failures are reported.</param>
    /// <param name="cancellationToken">Cancels the run.</param>
    /// <returns>The exit code.</returns>
    internal static async Task<int> RunAsync(string[] arguments, TextWriter output, TextWriter error, CancellationToken cancellationToken)
    {
        CommandLine command;
        IReadOnlyList<SourceFile> sources;
        try
        {
            command = CommandLine.Parse(arguments);
            if (command.Help)
            {
                await output.WriteLineAsync(CommandLine.Usage).ConfigureAwait(false);
                return Succeeded;
            }

            sources = SourceClosure.Read(command.Sources);
            if (command.Output is { } target && Directory.Exists(target))
            {
                throw new UsageException($"cannot {(command.Check ? "read" : "write")} {target}: the path is a directory");
            }

            if (command.Output is { } path && !command.Check && Path.GetDirectoryName(Path.GetFullPath(path)) is { } directory && !Directory.Exists(directory))
            {
                throw new UsageException($"cannot write {path}: the directory {directory} does not exist");
            }
        }
        catch (UsageException exception)
        {
            return await MisuseAsync(error, exception.Message).ConfigureAwait(false);
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            return await MisuseAsync(error, exception.Message).ConfigureAwait(false);
        }

        string generated;
        try
        {
            generated = await GenerateAsync(command, sources, cancellationToken).ConfigureAwait(false);
        }
        catch (GenerationException exception)
        {
            foreach (var problem in exception.Problems)
            {
                await error.WriteLineAsync($"error: {problem}").ConfigureAwait(false);
            }

            return Failed;
        }
        catch (CapabilityException exception)
        {
            await error.WriteLineAsync($"error: {exception.Message}").ConfigureAwait(false);
            await error.WriteLineAsync(
                "error: generation needs the service's static type facts: without them every property would be read as a Value, "
                + "indistinguishable from an untyped feature, so nothing was generated").ConfigureAwait(false);
            return Failed;
        }
        catch (OpenSysMLException exception)
        {
            await error.WriteLineAsync($"error: {exception.Message}").ConfigureAwait(false);
            return Failed;
        }

        try
        {
            if (command.Check)
            {
                string? existing = null;
                try
                {
                    existing = await File.ReadAllTextAsync(command.Output!, Utf8, cancellationToken).ConfigureAwait(false);
                }
                catch (FileNotFoundException)
                {
                }
                catch (DirectoryNotFoundException)
                {
                }

                if (existing == generated)
                {
                    return Succeeded;
                }

                await error.WriteLineAsync(
                    $"error: {command.Output} {(existing is null ? "does not exist" : "is out of date")}; regenerate it with: {command.RegenerateCommand()}").ConfigureAwait(false);
                return Failed;
            }

            if (command.Output is null)
            {
                await output.WriteAsync(generated).ConfigureAwait(false);
                await output.FlushAsync(cancellationToken).ConfigureAwait(false);
                return Succeeded;
            }

            await WriteAsync(command.Output, generated, cancellationToken).ConfigureAwait(false);
            return Succeeded;
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            return await MisuseAsync(
                error,
                $"cannot {(command.Check ? "read" : "write")} {command.Output ?? "standard output"}: {exception.Message}").ConfigureAwait(false);
        }
    }

    /// <summary>The file generated from the documents, through a service the command line names or the default one.</summary>
    /// <exception cref="GenerationException">The model has errors, or cannot be generated soundly.</exception>
    /// <exception cref="CapabilityException">The service does not report static type facts; nothing was parsed.</exception>
    /// <exception cref="OpenSysMLException">The service could not be reached or refused a call.</exception>
    internal static async Task<string> GenerateAsync(CommandLine command, IReadOnlyList<SourceFile> sources, CancellationToken cancellationToken)
    {
        var options = new ConnectionOptions { Address = command.Address, BinaryPath = command.Binary };
        var connection = await Connection.OpenAsync(options, cancellationToken).ConfigureAwait(false);
        await using (connection.ConfigureAwait(false))
        {
            return await GenerateAsync(connection, sources, command.Namespace, cancellationToken).ConfigureAwait(false);
        }
    }

    /// <summary>The file generated from the documents, parsed over an open connection.</summary>
    /// <exception cref="GenerationException">The model has errors, or cannot be generated soundly.</exception>
    /// <exception cref="CapabilityException">The service does not report static type facts; nothing was parsed.</exception>
    /// <exception cref="OpenSysMLException">The service refused a call.</exception>
    internal static async Task<string> GenerateAsync(Connection connection, IReadOnlyList<SourceFile> sources, string rootNamespace, CancellationToken cancellationToken)
    {
        connection.Capabilities.Require(Capabilities.TypeFacts);
        var documents = sources.Select(source => SourceDocument.Inline(source.Text, source.LogicalName, source.InlineLanguage));
        var model = await connection.ParseSourcesAsync(documents, cancellationToken: cancellationToken).ConfigureAwait(false);
        if (model.HasErrors)
        {
            throw new GenerationException(model.Errors.Select(diagnostic => diagnostic.ToString()));
        }

        var roots = await SymbolNode.ReadAsync(model, cancellationToken).ConfigureAwait(false);
        var plan = Planner.Plan(roots, rootNamespace, SourceClosure.Stamp(sources), [.. sources.Select(source => source.LogicalName)]);
        return Emitter.Render(plan);
    }

    // Written beside the destination first, then moved over it, so a failed run never leaves a partial file.
    private static async Task WriteAsync(string path, string text, CancellationToken cancellationToken)
    {
        var temporary = Path.Combine(Path.GetDirectoryName(Path.GetFullPath(path))!, $".{Path.GetFileName(path)}.{Guid.NewGuid():N}.tmp");
        try
        {
            await File.WriteAllTextAsync(temporary, text, Utf8, cancellationToken).ConfigureAwait(false);
            File.Move(temporary, path, overwrite: true);
        }
        finally
        {
            try
            {
                File.Delete(temporary);
            }
            catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
            {
            }
        }
    }

    private static async Task<int> MisuseAsync(TextWriter error, string problem)
    {
        await error.WriteLineAsync($"error: {problem}").ConfigureAwait(false);
        await error.WriteLineAsync("run with --help for usage").ConfigureAwait(false);
        return Misused;
    }
}
