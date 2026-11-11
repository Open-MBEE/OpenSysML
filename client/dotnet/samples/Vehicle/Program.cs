using System;
using OpenMBEE.OpenSysML;

// A ProjectReference consumer of the client, and the program docs/clients/dotnet.md walks
// through: the walkthrough region below is that page's code, and a test holds the two together.
// It connects to the sysml-grpc OPENSYSML_SERVICE names, or starts the one OPENSYSML_GRPC_BINARY
// names privately, prints what the service reports, parses a vehicle model, checks its
// diagnostics and evaluates the sedan's mass.
// <walkthrough>
const string Source = """
    package Demo {
      part def Vehicle {
        attribute mass default = 1500.0;
      }
      part sedan : Vehicle {
        attribute :>> mass = 1800.0;
      }
    }
    """;

try
{
    await using var connection = await Connection.OpenAsync();
    Console.WriteLine(connection.ServerInfo);

    var model = await connection.LoadContentAsync(Source);
    if (model.HasErrors)
    {
        foreach (var diagnostic in model.Errors)
        {
            Console.Error.WriteLine(diagnostic);
        }
        return 1;
    }

    var sedan = await model.LookupAsync("sedan");
    var mass = await model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = sedan.Id });
    Console.WriteLine($"{sedan}: mass = {mass}");
    return 0;
}
catch (OpenSysMLException error)
{
    Console.Error.WriteLine(error.Message);
    return 1;
}
// </walkthrough>
