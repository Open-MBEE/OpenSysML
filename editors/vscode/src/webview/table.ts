// A table-kind rendering drawn as an HTML table: one header row of its column
// names, one row per element, every cell plain text.
import type { RenderResult } from "../protocol";

// tableOf draws the rendering's rows; a row the server traced to a declaration
// is marked located, so a click on it can open that declaration.
export function tableOf(result: RenderResult): HTMLElement {
  const rows = result.rows ?? [];
  if (rows.length === 0) {
    const empty = document.createElement("p");
    empty.className = "empty";
    empty.textContent = "No elements to list.";
    return empty;
  }
  const columns = result.columns?.length ? result.columns.length : Math.max(...rows.map((row) => row.cells.length));
  const table = document.createElement("table");
  table.className = "opensysml-table";
  const headRow = table.createTHead().insertRow();
  for (let c = 0; c < columns; c++) {
    const th = document.createElement("th");
    th.textContent = result.columns?.[c] ?? "";
    headRow.append(th);
  }
  const body = table.createTBody();
  rows.forEach((row, index) => {
    const tr = body.insertRow();
    tr.dataset.opensysmlRow = String(index);
    if (row.origin) {
      tr.classList.add("located");
    }
    for (let c = 0; c < Math.max(columns, row.cells.length); c++) {
      const td = tr.insertCell();
      td.textContent = row.cells[c] ?? "";
    }
  });
  return table;
}
