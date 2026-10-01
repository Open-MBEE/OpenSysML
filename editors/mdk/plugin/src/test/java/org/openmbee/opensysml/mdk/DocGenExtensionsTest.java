package org.openmbee.opensysml.mdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.mdk.docgen.DocGenExtensions;
import org.openmbee.opensysml.mdk.engine.Operation;

class DocGenExtensionsTest {
  @Test
  void everyOperationHasAStereotypeNamedAfterAnExtensionClassInTheBridge() {
    Path bridge = Path.of("..", "mdk-bridge", "src", "main", "java", "org", "openmbee", "opensysml", "mdk", "docgen");
    for (Operation operation : Operation.values()) {
      String name = DocGenExtensions.stereotypeName(operation);
      assertTrue(name.startsWith("org.openmbee.opensysml.mdk.docgen."), name);
      String simple = name.substring(name.lastIndexOf('.') + 1);
      assertTrue(Files.isRegularFile(bridge.resolve(simple + ".java")), "no bridge class for " + name);
    }
  }

  @Test
  void reportListsCreatedExistingAndNotes() {
    DocGenExtensions.Report report = new DocGenExtensions.Report(
        List.of("package OpenSysML MDK DocGen", "«a»"), List.of("«b»"), List.of("String type missing."));
    assertEquals("Created: package OpenSysML MDK DocGen, «a»\nAlready present: «b»\nString type missing.",
        report.describe());
    assertEquals("", new DocGenExtensions.Report(List.of(), List.of(), List.of()).describe());
  }
}
