using System;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    /// <summary>
    /// Writes the model out in another format, as <see cref="Connection.ConvertAsync"/> does for
    /// <see cref="ConvertSource.FromModelHash"/>: the source the parse read is converted, so a file edited since does not
    /// change the answer, and notation keeps its comments and lexemes, re-indented. The service holds that source in a
    /// bounded cache, so a model loaded long ago may have been evicted; load it again then.
    /// </summary>
    /// <param name="toFormat">The format to write, named as <see cref="ConversionFormat"/> names one.</param>
    /// <param name="options">How to convert; null for <see cref="ConvertOptions.Default"/>. A model is notation, so <see cref="ConvertOptions.FromFormat"/> is best left null.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The converted model.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="toFormat"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option is invalid; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks a capability the conversion needs; nothing was sent.</exception>
    /// <exception cref="ConversionException">The model could not be written in that format.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a format it cannot write or notation for a model of several documents.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public Task<Conversion> ConvertAsync(string toFormat, ConvertOptions? options = null, CancellationToken cancellationToken = default) =>
        Connection.ConvertAsync(ConvertSource.FromModelHash(Hash), toFormat, options, cancellationToken);

    /// <summary>Writes the model out as SysML textual notation; see <see cref="ConvertAsync"/>.</summary>
    /// <param name="options">How to convert; null for <see cref="ConvertOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The notation.</returns>
    /// <exception cref="ConversionException">The model could not be written as notation.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    public Task<Conversion> ToNotationAsync(ConvertOptions? options = null, CancellationToken cancellationToken = default) =>
        ConvertAsync(ConversionFormat.SysML, options, cancellationToken);

    /// <summary>Writes the model out as RDF in Turtle syntax, through the experimental mapping; see <see cref="ConvertAsync"/>.</summary>
    /// <param name="options">How to convert; null for <see cref="ConvertOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The Turtle.</returns>
    /// <exception cref="ConversionException">The mapping refused what it cannot write.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    public Task<Conversion> ToTurtleAsync(ConvertOptions? options = null, CancellationToken cancellationToken = default) =>
        ConvertAsync(ConversionFormat.Turtle, options, cancellationToken);

    /// <summary>Writes the model out in the SysML v2 API's JSON element form, the same experimental mapping as Turtle; see <see cref="ConvertAsync"/>.</summary>
    /// <param name="options">How to convert, including the compact form; null for <see cref="ConvertOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The JSON.</returns>
    /// <exception cref="ConversionException">The mapping refused what it cannot write.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    public Task<Conversion> ToApiJsonAsync(ConvertOptions? options = null, CancellationToken cancellationToken = default) =>
        ConvertAsync(ConversionFormat.ApiJson, options, cancellationToken);

    /// <summary>
    /// Converts the model and writes it to a file in this process's file system, by <see cref="Conversion.SaveAsync"/>:
    /// whole, through a temporary file moved into place, and never over an existing file unless
    /// <see cref="SaveOptions.Overwrite"/> asks.
    /// </summary>
    /// <param name="path">The file to write.</param>
    /// <param name="toFormat">The format to write; null for the one the path's extension names, by <see cref="ConversionFormat.OfPath"/>.</param>
    /// <param name="options">How to convert; null for <see cref="ConvertOptions.Default"/>.</param>
    /// <param name="saveOptions">Whether an existing file is replaced; null for <see cref="SaveOptions.Default"/>, which refuses to.</param>
    /// <param name="cancellationToken">Cancels the conversion or the write, which then leaves nothing behind.</param>
    /// <returns>What was written.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="InvalidRequestException"><paramref name="toFormat"/> is null and the path's extension names no format, or an option is invalid; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks a capability the conversion needs; nothing was sent.</exception>
    /// <exception cref="ConversionException">The model could not be written in that format; nothing was written.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    /// <exception cref="UnwritableException">The destination was refused; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; nothing is left of it.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; nothing is left of it.</exception>
    public async Task<Conversion> SaveAsync(
        string path,
        string? toFormat = null,
        ConvertOptions? options = null,
        SaveOptions? saveOptions = null,
        CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        var format = toFormat ?? ConversionFormat.OfPath(path)
            ?? throw new InvalidRequestException($"{path} has no extension that names a format to write (.sysml, .kerml, .ttl, .turtle or .json); name the format.");
        var conversion = await ConvertAsync(format, options, cancellationToken).ConfigureAwait(false);
        await conversion.SaveAsync(path, saveOptions, cancellationToken).ConfigureAwait(false);
        return conversion;
    }
}
