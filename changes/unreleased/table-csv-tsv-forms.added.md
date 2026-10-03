- **Table views export as CSV and TSV.** `-render-form csv` or `tsv`, `%render <view> csv|tsv`, and
  `"form": "csv"` or `"tsv"` on `opensysml/render` write a table as a header record of its columns
  and one record per row, quoted as RFC 4180 quotes a field, so a spreadsheet or a CSV reader opens it
  directly; `-render-all` writes `.csv` or `.tsv` files and VS Code's *Export Diagram* saves them.
