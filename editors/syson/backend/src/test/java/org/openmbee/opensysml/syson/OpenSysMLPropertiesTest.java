package org.openmbee.opensysml.syson;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ConnectionOptions;

class OpenSysMLPropertiesTest {
    @Test
    void defaultsExplorationSchedule() {
        OpenSysMLProperties properties = new OpenSysMLProperties();
        assertThat(properties.explorationSchedule()).isEqualTo("explore");
        properties.setExploreRuns(3);
        properties.setExploreDepth(5);
        assertThat(properties.explorationSchedule()).isEqualTo("explore:runs=3,depth=5");
    }

    @Test
    void buildsConnectionOptions() {
        OpenSysMLProperties properties = new OpenSysMLProperties();
        properties.setService("localhost:9000");
        properties.setAllowUnpinnedDownload(true);
        ConnectionOptions options = properties.toConnectionOptions();
        assertThat(options.host()).contains("localhost");
        assertThat(options.port()).isEqualTo(9000);
        assertThat(options.autoStart()).isFalse();
        assertThat(options.allowUnpinnedDownload()).isTrue();
    }
}
