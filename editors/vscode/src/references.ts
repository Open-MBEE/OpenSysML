import type * as vscode from "vscode";
import type { Location, Position } from "vscode-languageclient/node";

// The editor's references peek, which takes its own Uri, Position and Location
// values where a server's command carries their protocol JSON forms.
export const SHOW_REFERENCES_COMMAND = "editor.action.showReferences";

export interface ReferenceConverter {
  asUri(value: string): vscode.Uri;
  asPosition(value: Position): vscode.Position;
  asLocation(value: Location): vscode.Location;
}

// showReferencesArguments converts a lens command's `[uri, position, locations]`
// for the references peek; undefined when the arguments are not that shape.
export function showReferencesArguments(
  args: unknown[] | undefined,
  convert: ReferenceConverter,
): [vscode.Uri, vscode.Position, vscode.Location[]] | undefined {
  if (!args || args.length !== 3) {
    return undefined;
  }
  const [uri, position, locations] = args;
  if (typeof uri !== "string" || !isPosition(position) || !Array.isArray(locations) || !locations.every(isLocation)) {
    return undefined;
  }
  return [convert.asUri(uri), convert.asPosition(position), locations.map((location) => convert.asLocation(location))];
}

function isPosition(value: unknown): value is Position {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const { line, character } = value as Record<string, unknown>;
  return typeof line === "number" && typeof character === "number";
}

function isLocation(value: unknown): value is Location {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const { uri, range } = value as Record<string, unknown>;
  if (typeof uri !== "string" || typeof range !== "object" || range === null) {
    return false;
  }
  const { start, end } = range as Record<string, unknown>;
  return isPosition(start) && isPosition(end);
}
