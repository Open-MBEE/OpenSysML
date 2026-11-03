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
    assertEquals(11_009_999L, ResourceDescriptorWriter.internalVersion("0.11.0"));
    assertEquals(11_009_999L, ResourceDescriptorWriter.internalVersion("v0.11.0"));
    assertEquals(102_030_000L, ResourceDescriptorWriter.internalVersion("1.2.3-nightly-20260101-abc"));
    assertTrue(ResourceDescriptorWriter.internalVersion("0.12.0") > ResourceDescriptorWriter.internalVersion("0.11.9"));
    assertTrue(ResourceDescriptorWriter.internalVersion("1.0.0") > ResourceDescriptorWriter.internalVersion("0.99.99"));
    assertTrue(ResourceDescriptorWriter.internalVersion("21.0.0") <= Integer.MAX_VALUE);
    assertThrows(IllegalArgumentException.class, () -> ResourceDescriptorWriter.internalVersion("nightly"));
    assertThrows(IllegalArgumentException.class, () -> ResourceDescriptorWriter.internalVersion("0.11.0-rc1"));
    assertThrows(IllegalArgumentException.class, () -> ResourceDescriptorWriter.internalVersion("0.100.0"));
    assertThrows(IllegalArgumentException.class, () -> ResourceDescriptorWriter.internalVersion("22.0.0"));
  }

  @Test
  void ordersNightliesByDayBetweenTheReleaseTheyLeadToAndThePreviousOne() {
    long previous = ResourceDescriptorWriter.internalVersion("0.11.0");
    long first = ResourceDescriptorWriter.internalVersion("0.11.1-nightly-20261028-aaa1111");
    long sameDay = ResourceDescriptorWriter.internalVersion("0.11.1-nightly-20261028-bbb2222");
    long next = ResourceDescriptorWriter.internalVersion("0.11.1-nightly-20261029-ccc3333");
    long release = ResourceDescriptorWriter.internalVersion("0.11.1");
    assertEquals(11_010_300L, first);
    assertEquals(first, sameDay);
    assertTrue(previous < first && first < next && next < release);
    assertTrue(release < ResourceDescriptorWriter.internalVersion("0.11.2-nightly-20260101-ddd4444"));
    assertEquals(ResourceDescriptorWriter.internalVersion("0.11.1-nightly-20250101-e5e5e5e"), first - 300);
    assertEquals(release - 1, ResourceDescriptorWriter.internalVersion("0.11.1-nightly-20991231-f6f6f6f"));
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
    assertTrue(xml.contains("<version human=\"0.11.0\" internal=\"11009999\" resource=\"11009999\"/>"));
    assertTrue(xml.contains("<provider name=\"Open-MBEE\""));
    assertTrue(xml.contains("<edition>Enterprise</edition>"));
    assertTrue(xml.contains(
        "<file from=\"plugins/org.openmbee.mdk.magic/plugin.xml\" to=\"plugins/org.openmbee.mdk.magic/plugin.xml\"/>"));
    assertTrue(xml.endsWith("  </installation>\n</resourceDescriptor>\n"));
  }
}
