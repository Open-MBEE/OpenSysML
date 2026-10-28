package org.openmbee.mdk.magic.tools;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class ResourceDescriptorWriterTest {
  @Test
  void listsEveryInstalledFileOnce(@TempDir Path dir) throws IOException {
    Path lib = Files.createDirectory(dir.resolve("lib"));
    Files.createFile(lib.resolve("client.jar"));
    Files.createFile(lib.resolve("notes.txt"));
    Path bin = Files.createDirectory(dir.resolve("bin"));
    Files.createFile(bin.resolve("sysml-grpc-linux-amd64"));
    List<String> files = ResourceDescriptorWriter.installedFiles("plugin.jar", "bridge.jar", lib, bin);
    assertEquals(
        List.of(
            "plugins/org.openmbee.mdk.magic/plugin.xml",
            "plugins/org.openmbee.mdk.magic/plugin.jar",
            "plugins/org.openmbee.mdk.magic/lib/client.jar",
            "plugins/org.openmbee.mdk.magic/bin/sysml-grpc-linux-amd64",
            "plugins/org.openmbee.mdk/extensions/bridge.jar",
            "data/resourcemanager/MDR_Plugin_Magic_MDK_91120_descriptor.xml"),
        files);
  }

  @Test
  void packsTheVersionIntoAnOrderedInteger() {
    assertEquals(11_000L, ResourceDescriptorWriter.internalVersion("0.11.0"));
    assertEquals(11_000L, ResourceDescriptorWriter.internalVersion("v0.11.0"));
    assertEquals(1_002_003L, ResourceDescriptorWriter.internalVersion("1.2.3-nightly-20260101-abc"));
    assertTrue(ResourceDescriptorWriter.internalVersion("0.12.0") > ResourceDescriptorWriter.internalVersion("0.11.9"));
    assertTrue(ResourceDescriptorWriter.internalVersion("1.0.0") > ResourceDescriptorWriter.internalVersion("0.99.99"));
    assertThrows(IllegalArgumentException.class, () -> ResourceDescriptorWriter.internalVersion("nightly"));
  }

  @Test
  void rendersTheVendorDescriptorForm() {
    String xml = ResourceDescriptorWriter.render("0.11.0", "2026-01-02", List.of("plugins/org.openmbee.mdk.magic/plugin.xml"));
    assertTrue(xml.startsWith("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<resourceDescriptor critical=\"false\""));
    assertTrue(xml.contains("date=\"2026-01-02\""));
    assertTrue(xml.contains("id=\"91120\""));
    assertTrue(xml.contains("type=\"Plugin\""));
    assertTrue(xml.contains("mdVersionMax=\"higher\""));
    assertTrue(xml.contains("name=\"Magic MDK\""));
    assertTrue(xml.contains("<version human=\"0.11.0\" internal=\"11000\" resource=\"11000\"/>"));
    assertTrue(xml.contains("<provider name=\"Open-MBEE\""));
    assertTrue(xml.contains("<edition>Enterprise</edition>"));
    assertTrue(xml.contains(
        "<file from=\"plugins/org.openmbee.mdk.magic/plugin.xml\" to=\"plugins/org.openmbee.mdk.magic/plugin.xml\"/>"));
    assertTrue(xml.endsWith("  </installation>\n</resourceDescriptor>\n"));
  }
}
