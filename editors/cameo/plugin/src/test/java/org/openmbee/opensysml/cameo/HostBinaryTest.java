package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;
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
}
