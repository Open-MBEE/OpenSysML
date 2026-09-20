package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermission;
import java.util.Set;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.openmbee.opensysml.cameo.bin.HostBinary;

class HostBinaryTest {
  @Test
  void namesCurrentHostAsset() {
    assertTrue(HostBinary.assetName().isPresent());
    assertTrue(HostBinary.assetName().orElseThrow().startsWith("sysml-grpc-"));
  }

  @Test
  void rejectsUnsupportedPlatformsAndArchitectures() {
    assertTrue(HostBinary.assetName("Linux", "riscv64").isEmpty());
    assertTrue(HostBinary.assetName("SunOS", "amd64").isEmpty());
  }

  @Test
  void mapsSupportedPlatformAliases() {
    assertEquals("sysml-grpc-windows-amd64.exe", HostBinary.assetName("Windows 11", "x86_64").orElseThrow());
    assertEquals("sysml-grpc-darwin-arm64", HostBinary.assetName("Mac OS X", "aarch64").orElseThrow());
  }

  @Test
  void makesStagedBinaryExecutable(@TempDir Path pluginDir) throws IOException {
    Path bin = Files.createDirectories(pluginDir.resolve("bin"));
    Path binary = bin.resolve(HostBinary.assetName().orElseThrow());
    Files.writeString(binary, "service");
    Files.setPosixFilePermissions(
        binary,
        Set.of(
            PosixFilePermission.OWNER_READ,
            PosixFilePermission.GROUP_READ,
            PosixFilePermission.OTHERS_READ));

    assertFalse(Files.isExecutable(binary));
    assertEquals(binary, HostBinary.locate(pluginDir));
    assertTrue(Files.isExecutable(binary));
  }
}
