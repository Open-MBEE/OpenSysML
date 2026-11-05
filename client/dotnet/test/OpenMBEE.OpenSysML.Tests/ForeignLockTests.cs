using System;
using System.Diagnostics;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>A failed foreign-holder acquisition ends the process whose ownership could not reach the caller.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ForeignLockTests
{
    [Theory]
    [InlineData("unexpected")]
    [InlineData("closed")]
    [InlineData("timeout")]
    public async Task AnAcquisitionThatFailsEndsItsProcessBeforeThrowing(string failure)
    {
        using var child = FakeChild.Create(_ => new FakeScript
        {
            Report = failure == "unexpected" ? "unexpected" : null,
            CloseStdout = failure == "closed",
            IgnoreStdinEnd = true,
        });
        var pid = 0;
        Process Start()
        {
            var process = Process.Start(new ProcessStartInfo(child.Path)
            {
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            }) ?? throw new InvalidOperationException("The stand-in did not start.");
            pid = process.Id;
            return process;
        }

        try
        {
            var error = await Record.ExceptionAsync(async () =>
            {
                await using var holder = await ForeignLock.HoldAsync(
                    ForeignClient.Python, "unused", Start, TimeSpan.FromSeconds(failure == "timeout" ? 1 : 10));
            });

            Assert.NotNull(error);
            Assert.Contains($"holder did not take the lock: it said {(failure == "unexpected" ? "unexpected" : "nothing")}", error.Message, StringComparison.Ordinal);
            Assert.True(pid > 0);
            Assert.False(FakeChild.IsRunning(pid), $"the holder process {pid} survived its failed acquisition");
        }
        finally
        {
            if (pid > 0)
            {
                FakeChild.Kill(pid);
                await FakeChild.AssertGoneAsync(pid);
            }
        }
    }
}
