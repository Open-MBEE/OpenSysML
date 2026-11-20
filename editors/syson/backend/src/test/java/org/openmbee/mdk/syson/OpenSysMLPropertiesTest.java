package org.openmbee.mdk.syson;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.util.Map;

import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.repository.ApiConfig;
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

    @Test
    void publishesToTheConfiguredServerAndFallsBackToTheEnvironment() {
        OpenSysMLProperties properties = new OpenSysMLProperties();
        Map<String, String> environment = Map.of("FLEXO_SYSMLV2_URL", "https://flexo.example.org/",
                "FLEXO_INTEROP_TOKEN", "from-environment");
        ApiConfig fromEnvironment = properties.getPublish().apiConfig(environment);
        assertThat(fromEnvironment.baseUrl()).isEqualTo("https://flexo.example.org");
        assertThat(fromEnvironment.token()).isEqualTo("from-environment");
        assertThat(properties.getPublish().apiConfig(Map.of()).baseUrl()).isEqualTo(ApiConfig.DEFAULT_BASE_URL);

        properties.getPublish().setUrl("http://10.0.0.5:8083");
        properties.getPublish().setToken("configured");
        properties.getPublish().setAllowPlainHttp(true);
        properties.getPublish().setTimeout(Duration.ofSeconds(5));
        ApiConfig configured = properties.getPublish().apiConfig(environment);
        assertThat(configured.baseUrl()).isEqualTo("http://10.0.0.5:8083");
        assertThat(configured.token()).isEqualTo("configured");
        assertThat(configured.allowPlainHttp()).isTrue();
        assertThat(configured.timeout()).isEqualTo(Duration.ofSeconds(5));
    }
}
