package org.openmbee.opensysml.cameo.tools;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;

class PluginDescriptorWriterTest {
  @Test
  void listsEveryJarInLibSorted() throws Exception {
    Path lib = Files.createTempDirectory("lib");
    Files.createFile(lib.resolve("b.jar"));
    Files.createFile(lib.resolve("a.jar"));
    Files.createFile(lib.resolve("notes.txt"));
    assertEquals(List.of("a.jar", "b.jar"), PluginDescriptorWriter.libraries(lib));
  }

  @Test
  void rendersOwnClassloaderDescriptor() {
    String xml = PluginDescriptorWriter.render("v1.2.3", "plugin.jar", List.of("client.jar"));
    assertTrue(xml.contains("ownClassloader=\"true\""));
    assertTrue(xml.contains("class-lookup=\"LocalFirst\""));
    assertTrue(xml.contains("<class>org.openmbee.opensysml.cameo.OpenSysMLPlugin</class>"));
    assertTrue(xml.contains("<library name=\"plugin.jar\"/>"));
    assertTrue(xml.contains("<library name=\"lib/client.jar\"/>"));
    assertTrue(xml.contains("version=\"v1.2.3\""));
  }
}
