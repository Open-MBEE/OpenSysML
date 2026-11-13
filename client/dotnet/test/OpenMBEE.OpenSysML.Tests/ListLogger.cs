using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>A logger that keeps every entry, so a test can assert what was logged and what never was.</summary>
internal sealed class ListLogger : ILogger
{
    private readonly ConcurrentQueue<Entry> entries = new();

    /// <summary>Every entry, in order.</summary>
    internal IReadOnlyCollection<Entry> Entries => [.. entries];

    /// <summary>Called with each entry as it is logged, on the logging thread.</summary>
    internal Action<Entry>? Logged { get; init; }

    public IDisposable? BeginScope<TState>(TState state)
        where TState : notnull => null;

    public bool IsEnabled(LogLevel logLevel) => true;

    public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
    {
        var entry = new Entry(logLevel, formatter(state, exception), exception);
        entries.Enqueue(entry);
        Logged?.Invoke(entry);
    }

    /// <summary>One entry.</summary>
    /// <param name="Level">Its level.</param>
    /// <param name="Message">Its formatted message.</param>
    /// <param name="Exception">The exception it carried.</param>
    internal sealed record Entry(LogLevel Level, string Message, Exception? Exception);
}
