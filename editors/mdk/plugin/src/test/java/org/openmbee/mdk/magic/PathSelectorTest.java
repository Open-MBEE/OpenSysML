package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.model.ModelPath;
import org.openmbee.mdk.magic.model.PathSelector;

class PathSelectorTest {
  @Test
  void v2RequiresBothCapabilities() {
    assertEquals(ModelPath.V2_TEXTUAL, PathSelector.select(true, true));
    assertEquals(ModelPath.V1_MDZIP, PathSelector.select(false, true));
    assertEquals(ModelPath.V1_MDZIP, PathSelector.select(true, false));
  }
}
