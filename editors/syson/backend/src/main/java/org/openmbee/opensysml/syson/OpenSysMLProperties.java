package org.openmbee.opensysml.syson;

import java.nio.file.Path;
import java.time.Duration;

import org.openmbee.opensysml.ConnectionOptions;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("opensysml")
public class OpenSysMLProperties {
    private Path binaryPath;
    private String service;
    private String downloadVersion;
    private String expectedBinarySha256;
    private String githubRepo;
    private boolean allowUnpinnedDownload;
    private Duration requestTimeout = ConnectionOptions.defaults().requestTimeout();
    private Duration startupTimeout = ConnectionOptions.defaults().startupTimeout();
    private String schedule;
    private Integer exploreRuns;
    private Integer exploreDepth;
    private String performer;

    public Path getBinaryPath() { return binaryPath; }
    public void setBinaryPath(Path value) { binaryPath = value; }
    public String getService() { return service; }
    public void setService(String value) { service = value; }
    public String getDownloadVersion() { return downloadVersion; }
    public void setDownloadVersion(String value) { downloadVersion = value; }
    public String getExpectedBinarySha256() { return expectedBinarySha256; }
    public void setExpectedBinarySha256(String value) { expectedBinarySha256 = value; }
    public String getGithubRepo() { return githubRepo; }
    public void setGithubRepo(String value) { githubRepo = value; }
    public boolean isAllowUnpinnedDownload() { return allowUnpinnedDownload; }
    public void setAllowUnpinnedDownload(boolean value) { allowUnpinnedDownload = value; }
    public Duration getRequestTimeout() { return requestTimeout; }
    public void setRequestTimeout(Duration value) { requestTimeout = value; }
    public Duration getStartupTimeout() { return startupTimeout; }
    public void setStartupTimeout(Duration value) { startupTimeout = value; }
    public String getSchedule() { return schedule; }
    public void setSchedule(String value) { schedule = value; }
    public Integer getExploreRuns() { return exploreRuns; }
    public void setExploreRuns(Integer value) { exploreRuns = value; }
    public Integer getExploreDepth() { return exploreDepth; }
    public void setExploreDepth(Integer value) { exploreDepth = value; }
    public String getPerformer() { return performer; }
    public void setPerformer(String value) { performer = value; }

    public ConnectionOptions toConnectionOptions() {
        ConnectionOptions.Builder builder = ConnectionOptions.builder()
                .allowUnpinnedDownload(allowUnpinnedDownload)
                .requestTimeout(requestTimeout)
                .startupTimeout(startupTimeout);
        if (binaryPath != null) builder.binaryPath(binaryPath);
        if (service != null && !service.isBlank()) {
            String[] parts = service.split(":", 2);
            builder.service(parts[0], Integer.parseInt(parts[1])).autoStart(false);
        }
        if (downloadVersion != null) builder.downloadVersion(downloadVersion);
        if (expectedBinarySha256 != null) builder.expectedBinarySha256(expectedBinarySha256);
        if (githubRepo != null) builder.githubRepo(githubRepo);
        return builder.build();
    }

    public String explorationSchedule() {
        if (exploreRuns == null && exploreDepth == null) return "explore";
        StringBuilder result = new StringBuilder("explore:");
        if (exploreRuns != null) result.append("runs=").append(exploreRuns);
        if (exploreDepth != null) {
            if (result.length() > "explore:".length()) result.append(',');
            result.append("depth=").append(exploreDepth);
        }
        return result.toString();
    }
}
