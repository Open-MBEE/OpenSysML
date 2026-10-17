package org.openmbee.opensysml;

import java.util.List;

/**
 * Which lints a diagnostics request keeps, as {@code sysml -disable-lint} and {@code -enable-lint}
 * select them.
 *
 * @param disableLint codes of lints left out of the answer
 * @param enableLint codes of opt-in lints kept in the answer
 */
public record DiagnosticsOptions(List<String> disableLint, List<String> enableLint) {

  /**
   * Validates and copies the selection.
   *
   * @param disableLint the lints left out
   * @param enableLint the opt-in lints kept
   */
  public DiagnosticsOptions {
    disableLint = List.copyOf(disableLint);
    enableLint = List.copyOf(enableLint);
  }

  /**
   * The service's default selection: every lint on by default, none opted in.
   *
   * @return the default options
   */
  public static DiagnosticsOptions defaults() {
    return new DiagnosticsOptions(List.of(), List.of());
  }

  /**
   * The same selection with these lints left out.
   *
   * @param codes lint codes to disable
   * @return options leaving them out
   */
  public DiagnosticsOptions disabling(String... codes) {
    return new DiagnosticsOptions(List.of(codes), enableLint);
  }

  /**
   * The same selection with these opt-in lints kept.
   *
   * @param codes lint codes to enable
   * @return options keeping them
   */
  public DiagnosticsOptions enabling(String... codes) {
    return new DiagnosticsOptions(disableLint, List.of(codes));
  }
}
