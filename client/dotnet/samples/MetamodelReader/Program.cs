using System;
using System.Linq;
using System.Reflection;
using System.Runtime.Versioning;
using OpenMBEE.OpenSysML.Metamodel;

if (args.Length != 1)
{
    Console.Error.WriteLine("usage: MetamodelReader <export.json>");
    return 2;
}

var metamodel = typeof(MetamodelGraph).Assembly;
var version = metamodel.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
var framework = metamodel.GetCustomAttribute<TargetFrameworkAttribute>()?.FrameworkName;
Console.WriteLine($"{metamodel.GetName().Name} {version} ({framework})");

// A path, never JSON text; a Stream, a JsonDocument or JsonElements read as well.
var graph = MetamodelGraph.ReadJson(args[0]);
Console.WriteLine($"{graph.Elements.Count} elements, {graph.Roots.Count} root namespace(s), metamodel {Metaclasses.MetamodelVersion}");

// All<T> selects by metamodel ancestry: every part definition, then every part usage.
foreach (var definition in graph.All<IPartDefinition>())
{
    Console.WriteLine($"part def {Name(definition)} owns {string.Join(", ", definition.OwnedFeature.Select(Name))}");
}

foreach (var usage in graph.All<IPartUsage>())
{
    Console.WriteLine($"part {Name(usage)} : {Types(usage)}");
}

return 0;

// An export need not carry every property: an absent one is not supplied rather than defaulted.
static string Name(IElement element)
{
    try
    {
        return element.DeclaredName ?? "(unnamed)";
    }
    catch (NotSuppliedException)
    {
        return "(unnamed)";
    }
}

// A derived property may be absent, and a reference may name an element the export does not include.
static string Types(IFeature feature)
{
    try
    {
        return feature.Type.Count == 0 ? "(none)" : string.Join(", ", feature.Type.Select(Name));
    }
    catch (NotSuppliedException)
    {
        return "(type not exported)";
    }
    catch (UnresolvedReferenceException e)
    {
        return $"(type {e.TargetId ?? e.TargetRef} is outside the document)";
    }
}
