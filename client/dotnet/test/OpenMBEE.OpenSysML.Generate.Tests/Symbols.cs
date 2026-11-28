using System.Linq;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>Synthetic symbol trees, shaped as the service reports them, for planning and emission without a service.</summary>
internal static class Symbols
{
    /// <summary>The root namespace of one document.</summary>
    internal static SymbolNode Root(params SymbolNode[] children) => Node("", "", "RootNamespace", null, null, [], children);

    /// <summary>A package.</summary>
    internal static SymbolNode Package(string id, params SymbolNode[] children) => Node(id, NameOf(id), "package", null, null, [], children);

    /// <summary>A definition, of <c>partDef</c> unless named otherwise.</summary>
    internal static SymbolNode Definition(string id, params SymbolNode[] features) => Definition(id, "partDef", [], features);

    /// <summary>A definition of a kind, with its generalization edges.</summary>
    internal static SymbolNode Definition(string id, string kind, Specialization[] edges, params SymbolNode[] members) =>
        Node(id, NameOf(id), kind, new TypeFacts("", "", "", "", "", false, ""), null, edges, members);

    /// <summary>A feature, of <c>attributeUsage</c> unless named otherwise.</summary>
    internal static SymbolNode Feature(string id, TypeFacts? type = null, Multiplicity? multiplicity = null, string kind = "attributeUsage", params Specialization[] edges) =>
        Node(id, NameOf(id), kind, type ?? new TypeFacts("", "", "", "", "", false, ""), multiplicity, edges, []);

    /// <summary>A feature with a name other than its id's last segment, as an unnamed one or one whose name holds <c>::</c>.</summary>
    internal static SymbolNode Named(string id, string name, TypeFacts? type = null, string kind = "attributeUsage", params Specialization[] edges) =>
        Node(id, name, kind, type ?? new TypeFacts("", "", "", "", "", false, ""), null, edges, []);

    /// <summary>The type facts of a library scalar.</summary>
    internal static TypeFacts Scalar(string primitive) => new(primitive, $"ScalarValues::{primitive}", "attributeDef", primitive, "declared", false, "");

    /// <summary>The type facts of a usage typed by a definition.</summary>
    internal static TypeFacts TypedBy(string id, string kind = "partDef") => new(NameOf(id), id, kind, "", "", false, "");

    /// <summary>A multiplicity.</summary>
    internal static Multiplicity Range(string lower, string upper) => new(lower, upper);

    /// <summary>A <c>specializes</c> edge to a definition.</summary>
    internal static Specialization Specializes(string target, string kind = "partDef") => new("specializes", NameOf(target), target, kind);

    /// <summary>A <c>redefines</c> edge to a feature.</summary>
    internal static Specialization Redefines(string target, string kind = "attributeUsage") => new("redefines", NameOf(target), target, kind);

    /// <summary>A <c>typing</c> edge to a definition.</summary>
    internal static Specialization Typing(string target, string kind = "partDef") => new("typing", NameOf(target), target, kind);

    private static SymbolNode Node(string id, string name, string kind, TypeFacts? type, Multiplicity? multiplicity, Specialization[] edges, SymbolNode[] children) =>
        new(new Symbol(id, name, kind, [], children.Select(child => child.Symbol.Id), [], type, multiplicity, edges, 0), children);

    private static string NameOf(string id) =>
        id.LastIndexOf("::", System.StringComparison.Ordinal) is var at and >= 0 ? id[(at + 2)..] : id;
}
