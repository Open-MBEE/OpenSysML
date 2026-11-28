using System;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// The conformance runner of the .NET client: runs conformance/scenarios through the client's public
/// API and judges every answer by the rules of conformance/README.md. Exits 0 when the run passed,
/// 1 when it did not, and 2 for a command line it cannot act on.
/// </summary>
internal static class Program
{
    private static async Task<int> Main(string[] args)
    {
        var utf8 = new UTF8Encoding(encoderShouldEmitUTF8Identifier: false);
        await using var output = new StreamWriter(Console.OpenStandardOutput(), utf8) { AutoFlush = true };
        await using var errors = new StreamWriter(Console.OpenStandardError(), utf8) { AutoFlush = true };
        RunOptions? options;
        try
        {
            options = RunArguments.Parse(args, Environment.CurrentDirectory);
        }
        catch (UsageException usage)
        {
            await errors.WriteAsync($"conformance: {usage.Message}\n{RunArguments.Usage}").ConfigureAwait(false);
            return 2;
        }

        if (options is null)
        {
            await output.WriteAsync(RunArguments.Usage).ConfigureAwait(false);
            return 0;
        }

        using var interrupted = new CancellationTokenSource();
        ConsoleCancelEventHandler interrupt = (_, e) =>
        {
            e.Cancel = true;
            interrupted.Cancel();
        };
        Console.CancelKeyPress += interrupt;
        try
        {
            return await ConformanceRun.RunAsync(options, output, errors, interrupted.Token).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (interrupted.IsCancellationRequested)
        {
            await errors.WriteAsync("conformance: interrupted\n").ConfigureAwait(false);
            return 1;
        }
        finally
        {
            Console.CancelKeyPress -= interrupt;
        }
    }
}
