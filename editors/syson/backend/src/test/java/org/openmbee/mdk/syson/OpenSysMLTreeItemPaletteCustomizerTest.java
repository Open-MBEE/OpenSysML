package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.openmbee.mdk.syson.menu.OpenSysMLTreeItemPaletteCustomizer;

class OpenSysMLTreeItemPaletteCustomizerTest {
    @Test
    void exposesStableToolIds() {
        assertThat(OpenSysMLTreeItemPaletteCustomizer.TOOL_ID).isEqualTo("runWithOpenSysML");
        assertThat(OpenSysMLTreeItemPaletteCustomizer.PUBLISH_TOOL_ID).isEqualTo("publishWithOpenSysML");
    }
}
