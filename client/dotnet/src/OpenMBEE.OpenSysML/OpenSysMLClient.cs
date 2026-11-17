using System;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One-shot loads: each opens a connection of its own, parses one source over it, and returns a model
/// that owns the connection, which disposing the model closes. When the parse fails, the connection
/// is closed before the failure is reported. To read several models over one connection, open a
/// <see cref="Connection"/> and load them through it instead.
/// </summary>
/// <remarks>
/// The connection is opened as <see cref="Connection.OpenAsync(ConnectionOptions?, CancellationToken)"/> opens it: to the service
/// <see cref="ConnectionOptions.Address"/> or <see cref="ConnectionOptions.ServiceVariable"/> names, or else to a private
/// one, which the model's connection holds like any other, so the service stops when nothing else holds it.
/// </remarks>
public static class OpenSysMLClient
{
    /// <summary>Opens a connection and parses a file the service reads over it, as <see cref="Connection.LoadAsync"/> does.</summary>
    /// <param name="path">The file to parse, which the service reads in its own file system.</param>
    /// <param name="connection">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="options">How strictly to judge the file; null for <see cref="ParseOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the open and the parse.</param>
    /// <returns>The model, owning its connection; the caller disposes it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="OpenSysMLException">The connection could not be opened, or the parse failed, as <see cref="Connection.OpenAsync(ConnectionOptions?, CancellationToken)"/> and <see cref="Connection.LoadAsync"/> report.</exception>
    public static Task<Model> LoadAsync(string path, ConnectionOptions? connection = null, ParseOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        return LoadOwnedAsync(token => Connection.OpenAsync(connection, token), (opened, token) => opened.LoadAsync(path, options, token), cancellationToken);
    }

    /// <summary>Opens a connection and parses notation given inline over it, as <see cref="Connection.LoadContentAsync"/> does.</summary>
    /// <param name="content">The notation.</param>
    /// <param name="connection">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="options">Its notation and how strictly to judge it; null for <see cref="ParseOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the open and the parse.</param>
    /// <returns>The model, owning its connection; the caller disposes it.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="content"/> is null.</exception>
    /// <exception cref="OpenSysMLException">The connection could not be opened, or the parse failed, as <see cref="Connection.OpenAsync(ConnectionOptions?, CancellationToken)"/> and <see cref="Connection.LoadContentAsync"/> report.</exception>
    public static Task<Model> LoadContentAsync(string content, ConnectionOptions? connection = null, ParseOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(content);
        return LoadOwnedAsync(token => Connection.OpenAsync(connection, token), (opened, token) => opened.LoadContentAsync(content, options, token), cancellationToken);
    }

    /// <summary>Opens a connection, parses over it, and hands the connection to the model, or closes it when the parse fails.</summary>
    /// <param name="open">Opens the connection.</param>
    /// <param name="parse">Parses over it.</param>
    /// <param name="cancellationToken">Cancels both.</param>
    /// <returns>The model, owning the connection.</returns>
    internal static async Task<Model> LoadOwnedAsync(
        Func<CancellationToken, Task<Connection>> open,
        Func<Connection, CancellationToken, Task<Model>> parse,
        CancellationToken cancellationToken)
    {
        var connection = await open(cancellationToken).ConfigureAwait(false);
        try
        {
            var model = await parse(connection, cancellationToken).ConfigureAwait(false);
            return model.Owning();
        }
        catch
        {
            await connection.DisposeAsync().ConfigureAwait(false);
            throw;
        }
    }
}
