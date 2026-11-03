package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.Map;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.cameo.CameoElements;

class CameoElementsTest {
  @Test
  void walksOwnersWithoutRoot() {
    Map<Object, Object> owners = Map.of("leaf", "pkg", "pkg", "model");
    Map<Object, String> names = Map.of("leaf", "Leaf", "pkg", "Pkg", "model", "Model");
    assertEquals("Pkg::Leaf", CameoElements.nameWalk("leaf", owners::get, names::get));
  }
}
