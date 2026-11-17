namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.ValidateInstanceAsync"/> validates an object: the engine the validation is put to. An
/// engine the service does not know is refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class ValidateOptions
{
    /// <summary>The engine the service chooses.</summary>
    public static ValidateOptions Default { get; } = new();

    /// <summary>
    /// The engine the validation is put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to
    /// the service. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }
}
