import type { DocumentSelector } from "vscode-languageclient/node";

// The languages the server reads, as the editor names them.
export const LANGUAGES = ["sysml", "kerml"] as const;

// documentSelector names the documents the language client serves: model
// files, the library's virtual documents under stdlibScheme, and the cells of
// any notebook in a model language. A notebook's cells are synchronized as
// one notebook document, so the server sees them as one model, and the cell
// entries let every request a cell makes reach it.
export function documentSelector(stdlibScheme: string): DocumentSelector {
  return LANGUAGES.flatMap((language) => [
    { scheme: "file", language },
    { scheme: stdlibScheme, language },
    { notebook: "*", language },
  ]);
}
