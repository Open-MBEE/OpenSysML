package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.cameo.bin.HostBinary;

class HostBinaryTest {
  @Test
  void namesCurrentHostAsset() {
    assertTrue(HostBinary.assetName().startsWith("sysml-grpc-"));
  }
}
