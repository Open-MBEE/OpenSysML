package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.syson.menu.OpenSysMLTreeItemPaletteCustomizer;

class OpenSysMLTreeItemPaletteCustomizerTest {
    @Test
    void exposesStableToolId() {
        assertThat(OpenSysMLTreeItemPaletteCustomizer.TOOL_ID).isEqualTo("runWithOpenSysML");
    }
}
