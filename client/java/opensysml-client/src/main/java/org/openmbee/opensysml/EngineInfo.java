package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * One analysis engine the service answers with: what it answers, how strongly it can, and whether
 * it can run here.
 *
 * @param name the engine's name, as {@link Model#withEngine(String)} selects it
 * @param authority the strongest evidence the engine can ever produce, spelled as {@link
 *     Standing#strength()} is
 * @param answers the kinds of question the engine answers ({@code "evaluate"}, {@code "holds"},
 *     {@code "outcomes"}, ...)
 * @param bounds the bounds the engine takes, in the order it reports them
 * @param process the external process the engine needs, empty for one running in-process
 * @param processFound where the process was found, empty when it was not or none is needed
 * @param ready whether the engine can run: it is served, and needs no process or its process was
 *     found
 * @param unavailable why the engine cannot run, empty when it can
 * @param kind where the engine comes from: {@code "built-in"}, or the kind of the manifest entry
 *     registering it ({@code "tool"}, {@code "engine"}, {@code "policy"}, {@code "sampler"})
 * @param protocol how the engine is spoken to, empty for a built-in one
 * @param source the manifest entry the engine was registered from, empty for a built-in one
 * @param command the command the entry resolved to, empty for a built-in one
 * @param version the version the entry declares, empty for a built-in one
 * @param served whether this service runs the engine for a request that names it
 */
public record EngineInfo(
    String name,
    String authority,
    List<String> answers,
    List<String> bounds,
    String process,
    String processFound,
    boolean ready,
    String unavailable,
    String kind,
    String protocol,
    String source,
    String command,
    String version,
    boolean served) {

  /**
   * Creates an engine description, copying its lists.
   *
   * @param name the name, never {@code null}
   * @param authority the authority, never {@code null}
   * @param answers the questions answered
   * @param bounds the bounds taken
   * @param process the process, never {@code null}
   * @param processFound where it was found, never {@code null}
   * @param ready whether it can run
   * @param unavailable why it cannot, never {@code null}
   * @param kind its origin, never {@code null}
   * @param protocol its protocol, never {@code null}
   * @param source its manifest entry, never {@code null}
   * @param command its command, never {@code null}
   * @param version its version, never {@code null}
   * @param served whether it is served
   */
  public EngineInfo {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(authority, "authority");
    answers = List.copyOf(answers);
    bounds = List.copyOf(bounds);
    Objects.requireNonNull(process, "process");
    Objects.requireNonNull(processFound, "processFound");
    Objects.requireNonNull(unavailable, "unavailable");
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(protocol, "protocol");
    Objects.requireNonNull(source, "source");
    Objects.requireNonNull(command, "command");
    Objects.requireNonNull(version, "version");
  }
}
