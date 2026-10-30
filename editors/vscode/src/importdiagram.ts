// How an opensysml/importDiagram answer is reported: what the import left alone,
// why it was refused, and what was imported. No VS Code here, so it is unit-tested.
import type { ImportDiagramResult } from "./protocol";

/** The message an import ends with, the severity it is shown at, and whether the edit is applied before it is shown. */
export interface ImportReport {
  severity: "error" | "warning" | "information";
  message: string;
  applies: boolean;
}

/**
 * importReport describes the import of `file`, a draw.io page of `result.view`. The
 * server refuses an import as a whole, so a refused result carries one refusal and
 * nothing to apply; a result with nothing to apply is reported as matching the
 * rendering; otherwise the edit is applied and the count reported, as a warning
 * when the import left something alone.
 */
export function importReport(file: string, result: ImportDiagramResult): ImportReport {
  const refusal = result.refused?.[0];
  if (refusal) {
    return { severity: "error", message: `${file}: not imported: ${refusal.message}`, applies: false };
  }
  const remarks = importRemarks(result);
  if (!result.edit || result.edits.length === 0) {
    const aside = remarks.length ? ` (${remarks.join("; ")})` : "";
    return { severity: "information", message: `${file} matches the rendering of ${result.view}; nothing to import${aside}`, applies: false };
  }
  const count = result.edits.length;
  const summary = `${count} layout ${count === 1 ? "edit" : "edits"} imported from ${file} into ${result.view}`;
  if (remarks.length) {
    return { severity: "warning", message: `${summary}; ${remarks.join("; ")}`, applies: true };
  }
  return { severity: "information", message: summary, applies: true };
}

// importRemarks lists what the import left alone: cells of elements the rendering
// does not draw, elements the file deleted, the server's notices, and how many
// cells name no element at all.
function importRemarks(result: ImportDiagramResult): string[] {
  const remarks = [
    ...(result.unmatched ?? []).map((name) => `${name} is not in the rendering`),
    ...(result.deleted ?? []).map((name) => `${name} was deleted in the file; the element stays`),
    ...(result.notices ?? []),
  ];
  if (result.unknown?.length) {
    remarks.push(`${result.unknown.length} cells name no element`);
  }
  return remarks;
}
