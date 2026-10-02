package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * The account a migration gives of itself: what became of every SysML v1 element.
 *
 * <p>The summary and the four counts come back with every migration. The entries and the text
 * come back when {@link MigrationOptions#withReport(boolean)} asked for them; the text is what
 * {@code sysml -migrate -migration-report} writes.
 *
 * @param source the v1 model migrated, as the service named it
 * @param exporter the tool that exported it, as its XMI says
 * @param summary the one-line account: {@code migrated N element(s): … mapped, … approximated, …
 *     unmapped (… skipped …)}
 * @param mapped elements with a faithful v2 form
 * @param approximated elements written in a v2 form that is not quite theirs
 * @param unmapped elements with no v2 form, left out and reported
 * @param skipped elements the migration does not consider: profile, library and notation-only
 *     content, and elements nothing refers to
 * @param entries every element's verdict, when the report was asked for; else empty
 * @param text the report as {@code -migration-report} writes it, when asked for; else empty
 */
public record MigrationReport(
    String source,
    String exporter,
    String summary,
    int mapped,
    int approximated,
    int unmapped,
    int skipped,
    List<MigrationEntry> entries,
    String text) {

  /**
   * Creates a report, copying its entries.
   *
   * @param source the source, never {@code null}
   * @param exporter the exporter, never {@code null}
   * @param summary the summary, never {@code null}
   * @param mapped the mapped count
   * @param approximated the approximated count
   * @param unmapped the unmapped count
   * @param skipped the skipped count
   * @param entries the entries
   * @param text the text, never {@code null}
   */
  public MigrationReport {
    Objects.requireNonNull(source, "source");
    Objects.requireNonNull(exporter, "exporter");
    Objects.requireNonNull(summary, "summary");
    entries = List.copyOf(entries);
    Objects.requireNonNull(text, "text");
  }

  /**
   * The entries with one verdict.
   *
   * @param verdict {@code "mapped"}, {@code "approximated"}, {@code "unmapped"} or {@code
   *     "skipped"}
   * @return those entries, in report order
   */
  public List<MigrationEntry> byVerdict(String verdict) {
    Objects.requireNonNull(verdict, "verdict");
    return entries.stream().filter(entry -> entry.verdict().equals(verdict)).toList();
  }
}
