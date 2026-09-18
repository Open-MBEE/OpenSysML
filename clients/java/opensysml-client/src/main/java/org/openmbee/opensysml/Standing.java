package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * How strongly an answer stands: the engine that answered, the strength of its evidence and the
 * bounds it ran under. A service without the {@code engines} capability reports none of it.
 *
 * @param engine the engine that answered, as {@link Connection#listEngines()} names it; empty when
 *     the service reported none
 * @param strength {@code "not covered"}, {@code "observed"}, {@code "witnessed"}, {@code "bounded"}
 *     or {@code "proved"}; empty when the service reported none
 * @param bounds the bounds the answer ran under
 */
public record Standing(String engine, String strength, List<Bound> bounds) {

  /** The engine selection that leaves the choice of engine to the service. */
  public static final String ENGINE_AUTO = "auto";

  /** The engine selection that puts the question to every engine covering it. */
  public static final String ENGINE_ALL = "all";

  /**
   * Creates a standing, copying its bounds.
   *
   * @param engine the engine, empty rather than {@code null} when none was reported
   * @param strength the strength, empty rather than {@code null} when none was reported
   * @param bounds the bounds
   */
  public Standing {
    Objects.requireNonNull(engine, "engine");
    Objects.requireNonNull(strength, "strength");
    bounds = List.copyOf(bounds);
  }

  /**
   * The standing a service that reports none leaves.
   *
   * @return a standing naming no engine
   */
  public static Standing none() {
    return new Standing("", "", List.of());
  }

  /**
   * Whether the service said which engine answered.
   *
   * @return {@code true} when an engine is named
   */
  public boolean reported() {
    return !engine.isEmpty();
  }

  /**
   * One bound an engine ran under.
   *
   * @param name the bound, as the engine names it ({@code "runs"}, {@code "depth"})
   * @param limit the value it ran under
   * @param reached whether the run met it, which is what keeps the answer from being stronger
   */
  public record Bound(String name, long limit, boolean reached) {
    /**
     * Creates a bound.
     *
     * @param name the bound's name, never {@code null}
     * @param limit the limit
     * @param reached whether it was met
     */
    public Bound {
      Objects.requireNonNull(name, "name");
    }
  }
}
