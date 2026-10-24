// What the Output channel is told of a render: the view asked for and what came
// back, so a panel with nothing to draw leaves its reason in the log.
import { nothingToDraw, type RenderParams, type RenderResult } from "./protocol";

/** logsRender says whether a render is logged: every one while `opensysml.trace.server` is on, and always one with nothing to draw. */
export function logsRender(trace: string | undefined, result: RenderResult): boolean {
  return (trace !== undefined && trace !== "off") || nothingToDraw(result);
}

/** renderSummary is the one entry a render leaves in the log: what was asked, what was drawn, and every notice. */
export function renderSummary(document: string, params: RenderParams, result: RenderResult): string {
  const view = result.view || params.view || "the implied view";
  const counts = [`${result.nodes.length} node(s)`, `${result.edges.length} edge(s)`];
  if (result.rows !== undefined) {
    counts.push(`${result.rows.length} row(s)`);
  }
  const notices = result.notices ?? [];
  const head = `Rendered ${view} of ${document}: ${result.kind} (${result.form}), ${counts.join(", ")}, ${notices.length} notice(s)`;
  const lead = nothingToDraw(result) ? `${head}; nothing to draw` : head;
  const listed = notices.map((notice) => `  ${notice}`).join("\n");
  return notices.length === 0 ? lead : `${lead}:\n${listed}`;
}
