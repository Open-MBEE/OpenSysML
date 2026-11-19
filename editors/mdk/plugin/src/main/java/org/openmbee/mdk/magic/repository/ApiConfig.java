package org.openmbee.mdk.magic.repository;

import java.net.InetAddress;
import java.net.URI;
import java.net.URISyntaxException;
import java.time.Duration;
import java.util.Map;
import java.util.Objects;

/**
 * One SysML v2 API server as the plugin addresses it: the API's base URL, the bearer token sent
 * when one is set, and the bound on a single request. The environment variables are the ones
 * {@code sysml -sync-apply} reads, so the command line and the plugin address a server alike.
 */
public record ApiConfig(String baseUrl, String token, Duration timeout, boolean allowPlainHttp) {
  public static final String ENV_BASE_URL = "FLEXO_SYSMLV2_URL";
  public static final String ENV_TOKEN = "FLEXO_INTEROP_TOKEN";
  public static final String ENV_PLAIN_HTTP = "FLEXO_ALLOW_PLAIN_HTTP";
  public static final String DEFAULT_BASE_URL = "http://localhost:8083";
  public static final Duration DEFAULT_TIMEOUT = Duration.ofSeconds(60);

  public ApiConfig {
    Objects.requireNonNull(baseUrl);
    Objects.requireNonNull(token);
    Objects.requireNonNull(timeout);
    baseUrl = trimSlashes(baseUrl.strip());
    if (timeout.isZero() || timeout.isNegative()) timeout = DEFAULT_TIMEOUT;
  }

  /** The server and token the environment names; the URL defaults to the published compose file's port. */
  public static ApiConfig fromEnvironment(Map<String, String> environment) {
    String base = environment.getOrDefault(ENV_BASE_URL, "");
    return new ApiConfig(
        base.isBlank() ? DEFAULT_BASE_URL : base,
        environment.getOrDefault(ENV_TOKEN, ""),
        DEFAULT_TIMEOUT,
        "1".equals(environment.get(ENV_PLAIN_HTTP)));
  }

  public ApiConfig withBaseUrl(String baseUrl) {
    return new ApiConfig(baseUrl, token, timeout, allowPlainHttp);
  }

  public ApiConfig withToken(String token) {
    return new ApiConfig(baseUrl, token, timeout, allowPlainHttp);
  }

  public boolean hasToken() {
    return !token.isEmpty();
  }

  /** The base URL as a URI, refused unless it is an http(s) URL the token may travel to. */
  public URI checkedBase() {
    return checkUrl(baseUrl);
  }

  /**
   * Refuses a URL that is not http(s), and a plaintext one to anything but this machine unless
   * the configuration opts in: a bearer token would cross the network in the clear.
   */
  public URI checkUrl(String raw) {
    URI uri;
    try {
      uri = new URI(raw);
    } catch (URISyntaxException exception) {
      throw new ApiException("server url \"" + raw + "\" is not a URL: " + exception.getReason());
    }
    String scheme = uri.getScheme() == null ? "" : uri.getScheme().toLowerCase(java.util.Locale.ROOT);
    if (!(scheme.equals("http") || scheme.equals("https")) || uri.getHost() == null) {
      throw new ApiException("server url \"" + raw + "\" is not an http(s) URL");
    }
    if (scheme.equals("http") && !loopback(uri.getHost()) && !allowPlainHttp) {
      throw new ApiException(
          raw + " would send the bearer token in the clear; use https://, or set " + ENV_PLAIN_HTTP
              + "=1 for a server you trust the network to");
    }
    return uri;
  }

  /** A host name that is this machine without a lookup. */
  static boolean loopback(String host) {
    String name = host.toLowerCase(java.util.Locale.ROOT);
    if (name.equals("localhost") || name.endsWith(".localhost")) return true;
    String literal = name.startsWith("[") && name.endsWith("]") ? name.substring(1, name.length() - 1) : name;
    if (!literal.matches("[0-9.]+") && !literal.contains(":")) return false;
    try {
      // An IP literal is parsed, never resolved.
      return InetAddress.getByName(literal).isLoopbackAddress();
    } catch (java.net.UnknownHostException exception) {
      return false;
    }
  }

  private static String trimSlashes(String url) {
    int end = url.length();
    while (end > 0 && url.charAt(end - 1) == '/') end--;
    return url.substring(0, end);
  }

  /** The token never appears in a log line or a dialog. */
  @Override
  public String toString() {
    return "ApiConfig[baseUrl=" + baseUrl + ", token=" + (hasToken() ? "set" : "none") + ", timeout=" + timeout + "]";
  }
}
