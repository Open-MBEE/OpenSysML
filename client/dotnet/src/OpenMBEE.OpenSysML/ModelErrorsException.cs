using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// <see cref="Model.RequireNoErrors"/> found error-severity diagnostics in the parse that produced the
/// model. <see cref="ModelException.Diagnostics"/> holds those errors, in report order.
/// </summary>
public class ModelErrorsException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">What was found.</param>
    /// <param name="method">The method whose answer reported the errors, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="errors">The error diagnostics.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic is null.</exception>
    public ModelErrorsException(string message, string method, IEnumerable<Diagnostic> errors)
        : base(message, method, errors)
    {
    }
}
