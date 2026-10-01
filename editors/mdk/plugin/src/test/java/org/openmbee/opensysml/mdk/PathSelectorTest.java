package org.openmbee.opensysml.mdk;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.mdk.model.ModelPath;
import org.openmbee.opensysml.mdk.model.PathSelector;

class PathSelectorTest {
  @Test
  void v2RequiresBothCapabilities() {
    assertEquals(ModelPath.V2_TEXTUAL, PathSelector.select(true, true));
    assertEquals(ModelPath.V1_MDZIP, PathSelector.select(false, true));
    assertEquals(ModelPath.V1_MDZIP, PathSelector.select(true, false));
  }
}
